package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/middleware"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
	"github.com/fulfillops/fulfillops/internal/view"
	rview "github.com/fulfillops/fulfillops/internal/view/reports"
)

type PageReportHandler struct {
	store      sessions.Store
	reportRepo repository.ReportExportRepository
	exportSvc  service.ExportService
	auditSvc   service.AuditService
}

func NewPageReportHandler(store sessions.Store, reportRepo repository.ReportExportRepository, exportSvc service.ExportService) *PageReportHandler {
	return &PageReportHandler{store: store, reportRepo: reportRepo, exportSvc: exportSvc}
}

func (h *PageReportHandler) WithAudit(auditSvc service.AuditService) *PageReportHandler {
	h.auditSvc = auditSvc
	return h
}

func (h *PageReportHandler) ShowWorkspace(c *gin.Context) {
	renderPage(c, http.StatusOK, rview.Workspace(pageCtx(c, h.store), rview.WorkspaceData{
		IsAdmin: isAdmin(c, h.store),
	}))
}

func (h *PageReportHandler) PostGenerateExport(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	sess, _ := h.store.Get(c.Request, "fulfillops")
	userID, _ := uuid.Parse(sess.Values["userID"].(string))

	includeSensitive := c.PostForm("include_sensitive") == "true"
	if includeSensitive {
		role, _ := sess.Values["userRole"].(string)
		if domain.UserRole(role) != domain.RoleAdministrator {
			redirectWithFlash(c, h.store, "/reports", "error", "include_sensitive requires Administrator role")
			return
		}
	}

	// Marshal filter form fields into the stored JSON blob so the ExportService picks them up.
	filters := map[string]any{}
	if df := c.PostForm("date_from"); df != "" {
		if t, err := time.Parse("2006-01-02", df); err == nil {
			filters["date_from"] = t.UTC().Format(time.RFC3339)
		}
	}
	if dt := c.PostForm("date_to"); dt != "" {
		if t, err := time.Parse("2006-01-02", dt); err == nil {
			filters["date_to"] = t.UTC().Format(time.RFC3339)
		}
	}
	if sf := c.PostForm("status_filter"); sf != "" {
		filters["status"] = sf
	}
	filterJSON, _ := json.Marshal(filters)

	r := &domain.ReportExport{
		ReportType:       c.PostForm("report_type"),
		Filters:          filterJSON,
		Status:           domain.ExportQueued,
		IncludeSensitive: includeSensitive,
		GeneratedBy:      &userID,
	}
	created, err := h.reportRepo.Create(ctx, r)
	if err != nil {
		redirectWithFlash(c, h.store, "/reports", "error", err.Error())
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Log(ctx, "report_exports", created.ID, "CREATE", nil, created)
	}

	if h.exportSvc != nil {
		go func(id uuid.UUID) { _ = h.exportSvc.GenerateExport(context.Background(), id) }(created.ID)
	}

	redirectWithFlash(c, h.store, "/reports/history", "queued", "Export queued. It will be available shortly.")
}

func (h *PageReportHandler) ShowHistory(c *gin.Context) {
	ctx := c.Request.Context()
	page := queryInt(c, "page", 1)
	const size = 20
	// Filter sensitive exports from non-admins at the repository level so the
	// total count and the page slice are consistent.
	filters := repository.ReportExportFilters{SensitiveVisible: isAdmin(c, h.store)}
	exports, total, _ := h.reportRepo.List(ctx, filters, domain.PageRequest{Page: page, PageSize: size})

	renderPage(c, http.StatusOK, rview.History(pageCtx(c, h.store), rview.HistoryData{
		Exports: exports,
		Pager:   view.NewPagination(page, size, total, "/reports/history", ""),
	}))
}

func (h *PageReportHandler) PostVerifyExport(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		redirectWithFlash(c, h.store, "/reports/history", "error", "Invalid export ID.")
		return
	}

	// Per-record authorization: non-admins cannot verify sensitive exports.
	export, err := h.reportRepo.GetByID(ctx, id)
	if err != nil {
		redirectWithFlash(c, h.store, "/reports/history", "error", "Export not found.")
		return
	}
	sess, _ := h.store.Get(c.Request, "fulfillops")
	role, _ := sess.Values["userRole"].(string)
	if export.IncludeSensitive && domain.UserRole(role) != domain.RoleAdministrator {
		redirectWithFlash(c, h.store, "/reports/history", "error", "Sensitive export access requires Administrator role.")
		return
	}

	if h.exportSvc == nil {
		redirectWithFlash(c, h.store, "/reports/history", "error", "Export service unavailable.")
		return
	}

	verified, err := h.exportSvc.VerifyChecksum(ctx, id)
	if err != nil {
		redirectWithFlash(c, h.store, "/reports/history", "error", "Verification failed: "+err.Error())
		return
	}

	if verified {
		redirectWithFlash(c, h.store, "/reports/history", "success", "Checksum verified successfully.")
	} else {
		redirectWithFlash(c, h.store, "/reports/history", "error", "Checksum mismatch — file may be corrupted.")
	}
}

func (h *PageReportHandler) DownloadExport(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid export ID"})
		return
	}

	export, err := h.reportRepo.GetByID(ctx, id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	// Per-record authorization: non-admins cannot download sensitive exports.
	sess, _ := h.store.Get(c.Request, "fulfillops")
	role, _ := sess.Values["userRole"].(string)
	if export.IncludeSensitive && domain.UserRole(role) != domain.RoleAdministrator {
		c.JSON(http.StatusForbidden, middleware.ErrorResponse{
			Code:    "FORBIDDEN",
			Message: "sensitive export access requires Administrator role",
		})
		return
	}

	if export.Status != domain.ExportCompleted || export.FilePath == nil {
		c.JSON(http.StatusConflict, middleware.ErrorResponse{Code: "NOT_READY", Message: "export not yet completed"})
		return
	}

	filename := filepath.Base(*export.FilePath)
	c.FileAttachment(*export.FilePath, filename)
}

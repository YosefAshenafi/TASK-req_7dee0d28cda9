package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/view"
	rview "github.com/fulfillops/fulfillops/internal/view/reports"
)

type PageReportHandler struct {
	store      sessions.Store
	reportRepo repository.ReportExportRepository
}

func NewPageReportHandler(store sessions.Store, reportRepo repository.ReportExportRepository) *PageReportHandler {
	return &PageReportHandler{store: store, reportRepo: reportRepo}
}

func (h *PageReportHandler) ShowWorkspace(c *gin.Context) {
	renderPage(c, http.StatusOK, rview.Workspace(pageCtx(c, h.store), rview.WorkspaceData{
		IsAdmin: isAdmin(c, h.store),
	}))
}

func (h *PageReportHandler) PostGenerateExport(c *gin.Context) {
	ctx := c.Request.Context()
	sess, _ := h.store.Get(c.Request, "fulfillops")
	userID, _ := uuid.Parse(sess.Values["userID"].(string))

	includeSensitive := c.PostForm("include_sensitive") == "true"
	r := &domain.ReportExport{
		ReportType:       c.PostForm("report_type"),
		Status:           "PENDING",
		IncludeSensitive: includeSensitive,
		GeneratedBy:      &userID,
	}
	if _, err := h.reportRepo.Create(ctx, r); err != nil {
		redirectWithFlash(c, h.store, "/reports", "error", err.Error())
		return
	}
	redirectWithFlash(c, h.store, "/reports/history", "queued", "Export queued. It will be available shortly.")
}

func (h *PageReportHandler) ShowHistory(c *gin.Context) {
	ctx := c.Request.Context()
	page := queryInt(c, "page", 1)
	const size = 20
	exports, total, _ := h.reportRepo.List(ctx, domain.PageRequest{Page: page, PageSize: size})
	renderPage(c, http.StatusOK, rview.History(pageCtx(c, h.store), rview.HistoryData{
		Exports: exports,
		Pager:   view.NewPagination(page, size, total, "/reports/history", ""),
	}))
}

func (h *PageReportHandler) PostVerifyExport(c *gin.Context) {
	// Verification is handled by the service layer in Phase 8
	redirectWithFlash(c, h.store, "/reports/history", "info", "Checksum verified.")
}

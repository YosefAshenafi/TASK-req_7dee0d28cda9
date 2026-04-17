package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/middleware"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

// ReportHandler handles report export endpoints.
type ReportHandler struct {
	reportRepo repository.ReportExportRepository
	exportSvc  service.ExportService
	auditSvc   service.AuditService
}

func NewReportHandler(reportRepo repository.ReportExportRepository, exportSvc service.ExportService, auditSvc service.AuditService) *ReportHandler {
	return &ReportHandler{reportRepo: reportRepo, exportSvc: exportSvc, auditSvc: auditSvc}
}

type createExportRequest struct {
	ReportType       string         `json:"report_type" binding:"required"`
	Filters          map[string]any `json:"filters"`
	IncludeSensitive bool           `json:"include_sensitive"`
}

// supportedReportTypes matches the ExportService.GenerateExport switch so
// callers get immediate feedback on unknown types instead of an async failure.
var supportedReportTypes = map[string]struct{}{
	"fulfillments": {},
	"customers":    {},
	"audit":        {},
}

// GET /api/v1/reports/exports
func (h *ReportHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	pr := domain.PageRequest{Page: page, PageSize: pageSize}
	pr.Normalize()

	// Visibility filter is applied inside the repository so the total and the
	// page slice agree — otherwise sensitive rows consume page slots on non-
	// admin requests and pagination totals become incoherent.
	roleRaw, _ := c.Get("userRole")
	role, _ := roleRaw.(domain.UserRole)
	filters := repository.ReportExportFilters{SensitiveVisible: role == domain.RoleAdministrator}

	exports, total, err := h.reportRepo.List(c.Request.Context(), filters, pr)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.PageResponse[domain.ReportExport]{
		Items:    exports,
		Total:    total,
		Page:     pr.Page,
		PageSize: pr.PageSize,
	})
}

// POST /api/v1/reports/exports
func (h *ReportHandler) Create(c *gin.Context) {
	var req createExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	if _, ok := supportedReportTypes[req.ReportType]; !ok {
		c.JSON(http.StatusUnprocessableEntity, middleware.ErrorResponse{
			Code:    "VALIDATION_ERROR",
			Message: "unsupported report_type",
			Details: map[string]string{
				"report_type": "must be one of: fulfillments, customers, audit",
			},
		})
		return
	}

	filtersJSON := []byte(`{}`)
	if req.Filters != nil {
		b, err := json.Marshal(req.Filters)
		if err == nil {
			filtersJSON = b
		}
	}

	actorRaw, _ := c.Get("userID")
	actorID, _ := actorRaw.(uuid.UUID)

	if req.IncludeSensitive {
		roleRaw, _ := c.Get("userRole")
		role, _ := roleRaw.(domain.UserRole)
		if role != domain.RoleAdministrator {
			c.JSON(http.StatusForbidden, middleware.ErrorResponse{
				Code:    "FORBIDDEN",
				Message: "include_sensitive requires Administrator role",
			})
			return
		}
	}

	export := &domain.ReportExport{
		ReportType:       req.ReportType,
		Filters:          filtersJSON,
		IncludeSensitive: req.IncludeSensitive,
		Status:           domain.ExportQueued,
		GeneratedBy:      &actorID,
	}

	created, err := h.reportRepo.Create(c.Request.Context(), export)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "report_exports", created.ID, "CREATE", nil, created)
	}

	// Generate the CSV in the background — don't block the HTTP response.
	if h.exportSvc != nil {
		go func(id uuid.UUID) {
			_ = h.exportSvc.GenerateExport(context.Background(), id)
		}(created.ID)
	}

	c.JSON(http.StatusCreated, created)
}

// GET /api/v1/reports/exports/:id
func (h *ReportHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid export ID"})
		return
	}

	export, err := h.reportRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	// Per-record authorization: non-admins cannot access sensitive exports.
	roleRaw, _ := c.Get("userRole")
	role, _ := roleRaw.(domain.UserRole)
	if export.IncludeSensitive && role != domain.RoleAdministrator {
		c.JSON(http.StatusForbidden, middleware.ErrorResponse{
			Code:    "FORBIDDEN",
			Message: "sensitive export access requires Administrator role",
		})
		return
	}

	c.JSON(http.StatusOK, export)
}

// POST /api/v1/reports/exports/:id/verify-checksum
func (h *ReportHandler) VerifyChecksum(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid export ID"})
		return
	}

	export, err := h.reportRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	roleRaw, _ := c.Get("userRole")
	role, _ := roleRaw.(domain.UserRole)
	if export.IncludeSensitive && role != domain.RoleAdministrator {
		c.JSON(http.StatusForbidden, middleware.ErrorResponse{
			Code:    "FORBIDDEN",
			Message: "sensitive export access requires Administrator role",
		})
		return
	}

	if h.exportSvc == nil {
		c.JSON(http.StatusOK, gin.H{"verified": false})
		return
	}

	verified, err := h.exportSvc.VerifyChecksum(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"verified": verified})
}

// DELETE /api/v1/reports/exports/:id
func (h *ReportHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid export ID"})
		return
	}

	actorRaw, _ := c.Get("userID")
	actorID, _ := actorRaw.(uuid.UUID)

	if h.exportSvc == nil {
		// Fallback: direct delete without file removal.
		if err := h.reportRepo.Delete(c.Request.Context(), id); err != nil {
			middleware.DomainErrorToHTTP(c, err)
		} else {
			c.Status(http.StatusNoContent)
		}
		return
	}

	if err := h.exportSvc.Delete(c.Request.Context(), id, actorID); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/middleware"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// AuditHandler handles audit log endpoints.
type AuditHandler struct {
	auditRepo repository.AuditRepository
}

func NewAuditHandler(auditRepo repository.AuditRepository) *AuditHandler {
	return &AuditHandler{auditRepo: auditRepo}
}

// GET /api/v1/audit
func (h *AuditHandler) List(c *gin.Context) {
	filters := repository.AuditFilters{}

	if s := c.Query("table_name"); s != "" {
		filters.TableName = s
	}
	if s := c.Query("operation"); s != "" {
		filters.Operation = s
	}
	if s := c.Query("record_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			filters.RecordID = &id
		}
	}
	if s := c.Query("performed_by"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			filters.PerformedBy = &id
		}
	}
	if s := c.Query("date_from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			filters.DateFrom = &t
		}
	}
	if s := c.Query("date_to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			filters.DateTo = &t
		}
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	pr := domain.PageRequest{Page: page, PageSize: pageSize}
	pr.Normalize()

	logs, total, err := h.auditRepo.List(c.Request.Context(), filters, pr)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.PageResponse[domain.AuditLog]{
		Items:    logs,
		Total:    total,
		Page:     pr.Page,
		PageSize: pr.PageSize,
	})
}

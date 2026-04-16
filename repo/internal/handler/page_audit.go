package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/view"
	aview "github.com/fulfillops/fulfillops/internal/view/audit"
)

type PageAuditHandler struct {
	store     sessions.Store
	auditRepo repository.AuditRepository
}

func NewPageAuditHandler(store sessions.Store, auditRepo repository.AuditRepository) *PageAuditHandler {
	return &PageAuditHandler{store: store, auditRepo: auditRepo}
}

func (h *PageAuditHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	filters := repository.AuditFilters{}
	if t := c.Query("table_name"); t != "" {
		filters.TableName = t
	}
	if op := c.Query("operation"); op != "" {
		filters.Operation = op
	}
	if rid := c.Query("record_id"); rid != "" {
		if id, err := uuid.Parse(rid); err == nil {
			filters.RecordID = &id
		}
	}
	if df := c.Query("date_from"); df != "" {
		if t, err := time.Parse("2006-01-02", df); err == nil {
			filters.DateFrom = &t
		}
	}
	if dt := c.Query("date_to"); dt != "" {
		if t, err := time.Parse("2006-01-02", dt); err == nil {
			filters.DateTo = &t
		}
	}

	page := queryInt(c, "page", 1)
	const size = 30
	logs, total, _ := h.auditRepo.List(ctx, filters, domain.PageRequest{Page: page, PageSize: size})

	renderPage(c, http.StatusOK, aview.List(pageCtx(c, h.store), aview.ListData{
		Logs:        logs,
		Pager:       view.NewPagination(page, size, total, "/audit", ""),
		TableFilter: c.Query("table_name"),
		OpFilter:    c.Query("operation"),
		RecordQ:     c.Query("record_id"),
		DateFrom:    c.Query("date_from"),
		DateTo:      c.Query("date_to"),
	}))
}

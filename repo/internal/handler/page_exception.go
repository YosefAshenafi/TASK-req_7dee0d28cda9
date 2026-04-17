package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
	"github.com/fulfillops/fulfillops/internal/view"
	eview "github.com/fulfillops/fulfillops/internal/view/exceptions"
)

type PageExceptionHandler struct {
	store        sessions.Store
	exRepo       repository.ExceptionRepository
	evRepo       repository.ExceptionEventRepository
	exceptionSvc service.ExceptionService
}

func NewPageExceptionHandler(
	store sessions.Store,
	exRepo repository.ExceptionRepository,
	evRepo repository.ExceptionEventRepository,
) *PageExceptionHandler {
	return &PageExceptionHandler{store: store, exRepo: exRepo, evRepo: evRepo}
}

func (h *PageExceptionHandler) WithExceptionService(svc service.ExceptionService) *PageExceptionHandler {
	h.exceptionSvc = svc
	return h
}

func (h *PageExceptionHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	filters := repository.ExceptionFilters{}
	if s := c.Query("status"); s != "" {
		filters.Status = domain.ExceptionStatus(s)
	}
	if t := c.Query("type"); t != "" {
		filters.Type = domain.ExceptionType(t)
	}
	if fid := c.Query("fulfillment_id"); fid != "" {
		if id, err := uuid.Parse(fid); err == nil {
			filters.FulfillmentID = &id
		}
	}
	if df := c.Query("date_from"); df != "" {
		if t, err := time.Parse("2006-01-02", df); err == nil {
			filters.OpenedFrom = &t
		}
	}
	if dt := c.Query("date_to"); dt != "" {
		if t, err := time.Parse("2006-01-02", dt); err == nil {
			filters.OpenedTo = &t
		}
	}

	page := queryInt(c, "page", 1)
	const size = 20
	all, _ := h.exRepo.List(ctx, filters)
	total := len(all)
	start, end := (page-1)*size, page*size
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	renderPage(c, http.StatusOK, eview.List(pageCtx(c, h.store), eview.ListData{
		Exceptions:   all[start:end],
		StatusFilter: string(filters.Status),
		TypeFilter:   string(filters.Type),
		FulfillmentQ: c.Query("fulfillment_id"),
		DateFrom:     c.Query("date_from"),
		DateTo:       c.Query("date_to"),
		Pager:        view.NewPagination(page, size, total, "/exceptions", ""),
	}))
}

func (h *PageExceptionHandler) ShowDetail(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	ex, err := h.exRepo.GetByID(ctx, id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	events, _ := h.evRepo.ListByExceptionID(ctx, id)
	evItems := make([]eview.ExceptionEvent, len(events))
	for i, ev := range events {
		evItems[i] = eview.ExceptionEvent{ExceptionEvent: ev}
	}

	openedByName := ""
	if ex.OpenedBy != nil {
		s := ex.OpenedBy.String()
		if len(s) >= 8 {
			openedByName = s[:8]
		} else {
			openedByName = s
		}
	}

	renderPage(c, http.StatusOK, eview.Detail(pageCtx(c, h.store), eview.DetailData{
		Exception:    *ex,
		Events:       evItems,
		OpenedByName: openedByName,
		CanUpdate:    canEdit(c, h.store),
	}))
}

func (h *PageExceptionHandler) PostUpdateStatus(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	sess, _ := h.store.Get(c.Request, "fulfillops")
	userID, _ := uuid.Parse(sess.Values["userID"].(string))
	ctx = service.WithUserID(ctx, userID)

	status := domain.ExceptionStatus(c.PostForm("status"))
	note := c.PostForm("resolution_note")

	if h.exceptionSvc != nil {
		if _, err := h.exceptionSvc.UpdateStatus(ctx, id, status, note); err != nil {
			redirectWithFlash(c, h.store, "/exceptions/"+id.String(), "error", err.Error())
			return
		}
	} else {
		var notePtr *string
		if note != "" {
			notePtr = &note
		}
		if err := h.exRepo.UpdateStatus(ctx, id, status, notePtr, &userID); err != nil {
			redirectWithFlash(c, h.store, "/exceptions/"+id.String(), "error", err.Error())
			return
		}
	}
	redirectWithFlash(c, h.store, "/exceptions/"+id.String(), "success", "Status updated.")
}

func (h *PageExceptionHandler) PostAddEvent(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	sess, _ := h.store.Get(c.Request, "fulfillops")
	userID, _ := uuid.Parse(sess.Values["userID"].(string))

	ev := &domain.ExceptionEvent{
		ExceptionID: id,
		EventType:   c.PostForm("event_type"),
		Content:     c.PostForm("content"),
		CreatedBy:   &userID,
	}
	if err := h.evRepo.Create(ctx, ev); err != nil {
		redirectWithFlash(c, h.store, "/exceptions/"+id.String(), "error", err.Error())
		return
	}
	redirectWithFlash(c, h.store, "/exceptions/"+id.String(), "success", "Event added.")
}

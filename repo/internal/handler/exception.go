package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/middleware"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

// ExceptionHandler handles fulfillment exception endpoints.
type ExceptionHandler struct {
	exceptionSvc  service.ExceptionService
	exceptionRepo repository.ExceptionRepository
	exEventRepo   repository.ExceptionEventRepository
}

func NewExceptionHandler(
	exceptionSvc service.ExceptionService,
	exceptionRepo repository.ExceptionRepository,
	exEventRepo repository.ExceptionEventRepository,
) *ExceptionHandler {
	return &ExceptionHandler{
		exceptionSvc:  exceptionSvc,
		exceptionRepo: exceptionRepo,
		exEventRepo:   exEventRepo,
	}
}

type createExceptionRequest struct {
	FulfillmentID uuid.UUID           `json:"fulfillment_id" binding:"required"`
	Type          domain.ExceptionType `json:"type" binding:"required"`
	Note          string               `json:"note"`
}

type updateExceptionStatusRequest struct {
	Status         domain.ExceptionStatus `json:"status" binding:"required"`
	ResolutionNote string                 `json:"resolution_note"`
}

type addExceptionEventRequest struct {
	EventType string `json:"event_type" binding:"required"`
	Content   string `json:"content" binding:"required"`
}

// GET /api/v1/exceptions
func (h *ExceptionHandler) List(c *gin.Context) {
	filters := repository.ExceptionFilters{}

	if s := c.Query("status"); s != "" {
		filters.Status = domain.ExceptionStatus(s)
	}
	if s := c.Query("type"); s != "" {
		filters.Type = domain.ExceptionType(s)
	}
	if s := c.Query("fulfillment_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			filters.FulfillmentID = &id
		}
	}

	exceptions, err := h.exceptionSvc.List(c.Request.Context(), filters)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": exceptions})
}

// POST /api/v1/exceptions
func (h *ExceptionHandler) Create(c *gin.Context) {
	var req createExceptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	created, err := h.exceptionSvc.Create(c.Request.Context(), req.FulfillmentID, req.Type, req.Note)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusCreated, created)
}

// GET /api/v1/exceptions/:id
func (h *ExceptionHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid exception ID"})
		return
	}

	ex, err := h.exceptionSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	events, err := h.exEventRepo.ListByExceptionID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"exception": ex,
		"events":    events,
	})
}

// PUT /api/v1/exceptions/:id/status
func (h *ExceptionHandler) UpdateStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid exception ID"})
		return
	}

	var req updateExceptionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	updated, err := h.exceptionSvc.UpdateStatus(c.Request.Context(), id, req.Status, req.ResolutionNote)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, updated)
}

// POST /api/v1/exceptions/:id/events
func (h *ExceptionHandler) AddEvent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid exception ID"})
		return
	}

	var req addExceptionEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	event, err := h.exceptionSvc.AddEvent(c.Request.Context(), id, req.EventType, req.Content)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusCreated, event)
}

package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/middleware"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

// MessageHandler handles messaging endpoints.
type MessageHandler struct {
	messagingSvc service.MessagingService
	templateRepo repository.MessageTemplateRepository
	sendLogRepo  repository.SendLogRepository
	notifRepo    repository.NotificationRepository
	auditSvc     service.AuditService
}

func NewMessageHandler(
	messagingSvc service.MessagingService,
	templateRepo repository.MessageTemplateRepository,
	sendLogRepo repository.SendLogRepository,
	notifRepo repository.NotificationRepository,
	auditSvc service.AuditService,
) *MessageHandler {
	return &MessageHandler{
		messagingSvc: messagingSvc,
		templateRepo: templateRepo,
		sendLogRepo:  sendLogRepo,
		notifRepo:    notifRepo,
		auditSvc:     auditSvc,
	}
}

type createTemplateRequest struct {
	Name         string                  `json:"name" binding:"required"`
	Category     domain.TemplateCategory `json:"category" binding:"required"`
	Channel      domain.SendLogChannel   `json:"channel" binding:"required"`
	BodyTemplate string                  `json:"body_template" binding:"required"`
}

type updateTemplateRequest struct {
	Name         string                  `json:"name" binding:"required"`
	Category     domain.TemplateCategory `json:"category" binding:"required"`
	Channel      domain.SendLogChannel   `json:"channel" binding:"required"`
	BodyTemplate string                  `json:"body_template" binding:"required"`
	Version      int                     `json:"version" binding:"required"`
}

type dispatchRequest struct {
	TemplateID    uuid.UUID                `json:"template_id" binding:"required"`
	RecipientID   uuid.UUID                `json:"recipient_id" binding:"required"`
	ExtraChannels []domain.SendLogChannel  `json:"extra_channels"` // optional: SMS, EMAIL
	Context       map[string]any           `json:"context"`
}

// GET /api/v1/message-templates
func (h *MessageHandler) ListTemplates(c *gin.Context) {
	category := domain.TemplateCategory(c.DefaultQuery("category", ""))
	channel := domain.SendLogChannel(c.DefaultQuery("channel", ""))
	includeDeleted := c.DefaultQuery("include_deleted", "false") == "true"

	templates, err := h.templateRepo.List(c.Request.Context(), category, channel, includeDeleted)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": templates})
}

// POST /api/v1/message-templates
func (h *MessageHandler) CreateTemplate(c *gin.Context) {
	var req createTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	tmpl := &domain.MessageTemplate{
		Name:         req.Name,
		Category:     req.Category,
		Channel:      req.Channel,
		BodyTemplate: req.BodyTemplate,
	}

	created, err := h.templateRepo.Create(c.Request.Context(), tmpl)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "message_templates", created.ID, "CREATE", nil, created)
	}

	c.JSON(http.StatusCreated, created)
}

// GET /api/v1/message-templates/:id
func (h *MessageHandler) GetTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid template ID"})
		return
	}

	tmpl, err := h.templateRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, tmpl)
}

// PUT /api/v1/message-templates/:id
func (h *MessageHandler) UpdateTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid template ID"})
		return
	}

	var req updateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	before, _ := h.templateRepo.GetByID(c.Request.Context(), id)

	tmpl := &domain.MessageTemplate{
		ID:           id,
		Name:         req.Name,
		Category:     req.Category,
		Channel:      req.Channel,
		BodyTemplate: req.BodyTemplate,
		Version:      req.Version,
	}

	updated, err := h.templateRepo.Update(c.Request.Context(), tmpl)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "message_templates", id, "UPDATE", before, updated)
	}

	c.JSON(http.StatusOK, updated)
}

// DELETE /api/v1/message-templates/:id
func (h *MessageHandler) DeleteTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid template ID"})
		return
	}

	actorID, _ := c.Get("userID")
	deletedBy, _ := actorID.(uuid.UUID)

	before, _ := h.templateRepo.GetByID(c.Request.Context(), id)

	if err := h.templateRepo.SoftDelete(c.Request.Context(), id, deletedBy); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "message_templates", id, "DELETE", before, map[string]any{"deleted_by": deletedBy})
	}

	c.Status(http.StatusNoContent)
}

// GET /api/v1/send-logs
func (h *MessageHandler) ListSendLogs(c *gin.Context) {
	filters := repository.SendLogFilters{}

	if s := c.Query("channel"); s != "" {
		filters.Channel = domain.SendLogChannel(s)
	}
	if s := c.Query("status"); s != "" {
		filters.Status = domain.SendLogStatus(s)
	}
	if s := c.Query("recipient_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			filters.RecipientID = &id
		}
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	pr := domain.PageRequest{Page: page, PageSize: pageSize}
	pr.Normalize()

	logs, total, err := h.sendLogRepo.List(c.Request.Context(), filters, pr)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.PageResponse[domain.SendLog]{
		Items:    logs,
		Total:    total,
		Page:     pr.Page,
		PageSize: pr.PageSize,
	})
}

// PUT /api/v1/send-logs/:id/printed
func (h *MessageHandler) MarkPrinted(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid send log ID"})
		return
	}

	if err := h.messagingSvc.MarkPrinted(c.Request.Context(), id); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

type markFailedRequest struct {
	Reason string `json:"reason"`
}

// PUT /api/v1/send-logs/:id/failed
// Operator-reported failure transition for an offline handoff. Sets status to
// FAILED (stamping first_failed_at on the first call) so the retry scheduler
// can re-queue it on the configured cadence.
func (h *MessageHandler) MarkFailed(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid send log ID"})
		return
	}

	var req markFailedRequest
	_ = c.ShouldBindJSON(&req) // reason is optional

	if err := h.messagingSvc.MarkFailed(c.Request.Context(), id, req.Reason); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GET /api/v1/notifications
func (h *MessageHandler) ListNotifications(c *gin.Context) {
	actorID, _ := c.Get("userID")
	userID, _ := actorID.(uuid.UUID)

	var isRead *bool
	if s := c.Query("is_read"); s != "" {
		b := s == "true"
		isRead = &b
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	pr := domain.PageRequest{Page: page, PageSize: pageSize}
	pr.Normalize()

	notifications, total, err := h.notifRepo.ListByUserID(c.Request.Context(), userID, isRead, pr)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.PageResponse[domain.Notification]{
		Items:    notifications,
		Total:    total,
		Page:     pr.Page,
		PageSize: pr.PageSize,
	})
}

// PUT /api/v1/notifications/:id/read
func (h *MessageHandler) MarkNotificationRead(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid notification ID"})
		return
	}

	actorID, _ := c.Get("userID")
	userID, _ := actorID.(uuid.UUID)

	if err := h.notifRepo.MarkRead(c.Request.Context(), id, userID); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// POST /api/v1/dispatch
func (h *MessageHandler) Dispatch(c *gin.Context) {
	var req dispatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	if req.Context == nil {
		req.Context = map[string]any{}
	}

	log, err := h.messagingSvc.Dispatch(c.Request.Context(), req.TemplateID, req.RecipientID, req.ExtraChannels, req.Context)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusCreated, log)
}

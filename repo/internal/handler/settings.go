package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/middleware"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

// SettingsHandler handles system settings and blackout date endpoints.
type SettingsHandler struct {
	settingRepo  repository.SystemSettingRepository
	blackoutRepo repository.BlackoutDateRepository
	auditSvc     service.AuditService
}

func NewSettingsHandler(
	settingRepo repository.SystemSettingRepository,
	blackoutRepo repository.BlackoutDateRepository,
) *SettingsHandler {
	return &SettingsHandler{settingRepo: settingRepo, blackoutRepo: blackoutRepo}
}

// WithAudit attaches an AuditService so settings writes are audit-logged.
func (h *SettingsHandler) WithAudit(auditSvc service.AuditService) *SettingsHandler {
	h.auditSvc = auditSvc
	return h
}

type setSettingRequest struct {
	Value string `json:"value" binding:"required"`
}

type createBlackoutDateRequest struct {
	Date        string  `json:"date" binding:"required"` // RFC3339 or YYYY-MM-DD
	Description *string `json:"description"`
}

// GET /api/v1/settings
func (h *SettingsHandler) GetAll(c *gin.Context) {
	settings, err := h.settingRepo.GetAll(c.Request.Context())
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": settings})
}

// PUT /api/v1/settings/:key
func (h *SettingsHandler) Set(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "key is required"})
		return
	}

	var req setSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	actorRaw, _ := c.Get("userID")
	actorID, _ := actorRaw.(uuid.UUID)

	// Store as JSON so consumers using json.Unmarshal can decode the value.
	// If value is already valid JSON (e.g. a JSON array), store as-is; otherwise JSON-encode as a string.
	var jsonValue []byte
	if json.Valid([]byte(req.Value)) {
		jsonValue = []byte(req.Value)
	} else {
		var err error
		jsonValue, err = json.Marshal(req.Value)
		if err != nil {
			c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid value"})
			return
		}
	}

	// Capture "before" for audit.
	before, _ := h.settingRepo.Get(c.Request.Context(), key)

	if err := h.settingRepo.Set(c.Request.Context(), key, jsonValue, &actorID); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	setting, err := h.settingRepo.Get(c.Request.Context(), key)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	if h.auditSvc != nil {
		id := uuid.Nil
		if setting != nil {
			id = setting.ID
		}
		_ = h.auditSvc.Log(c.Request.Context(), "system_settings", id, "UPDATE", before, setting)
	}

	c.JSON(http.StatusOK, setting)
}

// GET /api/v1/settings/blackout-dates
func (h *SettingsHandler) ListBlackoutDates(c *gin.Context) {
	dates, err := h.blackoutRepo.List(c.Request.Context())
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": dates})
}

// POST /api/v1/settings/blackout-dates
func (h *SettingsHandler) CreateBlackoutDate(c *gin.Context) {
	var req createBlackoutDateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	t, err := parseBlackoutDate(req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid date format; use RFC3339 or YYYY-MM-DD"})
		return
	}

	actorRaw, _ := c.Get("userID")
	actorID, _ := actorRaw.(uuid.UUID)

	bd := &domain.BlackoutDate{
		Date:        t,
		Description: req.Description,
		CreatedBy:   &actorID,
	}

	created, err := h.blackoutRepo.Create(c.Request.Context(), bd)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "blackout_dates", created.ID, "CREATE", nil, created)
	}

	c.JSON(http.StatusCreated, created)
}

// DELETE /api/v1/settings/blackout-dates/:id
func (h *SettingsHandler) DeleteBlackoutDate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid blackout date ID"})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "blackout_dates", id, "DELETE", map[string]any{"id": id}, nil)
	}

	if err := h.blackoutRepo.Delete(c.Request.Context(), id); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// parseBlackoutDate accepts RFC3339 or YYYY-MM-DD.
func parseBlackoutDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Parse("2006-01-02", s)
}

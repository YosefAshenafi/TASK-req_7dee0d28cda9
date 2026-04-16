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

// TierHandler handles reward tier endpoints.
type TierHandler struct {
	tierRepo repository.TierRepository
	auditSvc service.AuditService
}

func NewTierHandler(tierRepo repository.TierRepository, auditSvc service.AuditService) *TierHandler {
	return &TierHandler{tierRepo: tierRepo, auditSvc: auditSvc}
}

type createTierRequest struct {
	Name           string  `json:"name" binding:"required"`
	Description    *string `json:"description"`
	InventoryCount int     `json:"inventory_count"`
	PurchaseLimit  int     `json:"purchase_limit"`
	AlertThreshold int     `json:"alert_threshold"`
}

type updateTierRequest struct {
	Name           string  `json:"name" binding:"required"`
	Description    *string `json:"description"`
	InventoryCount int     `json:"inventory_count"`
	PurchaseLimit  int     `json:"purchase_limit"`
	AlertThreshold int     `json:"alert_threshold"`
	Version        int     `json:"version" binding:"required"`
}

// GET /api/v1/tiers
func (h *TierHandler) List(c *gin.Context) {
	name := c.DefaultQuery("name", "")
	includeDeleted := c.DefaultQuery("include_deleted", "false") == "true"

	tiers, err := h.tierRepo.List(c.Request.Context(), name, includeDeleted)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": tiers})
}

// POST /api/v1/tiers
func (h *TierHandler) Create(c *gin.Context) {
	var req createTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	tier := &domain.RewardTier{
		Name:           req.Name,
		Description:    req.Description,
		InventoryCount: req.InventoryCount,
		PurchaseLimit:  req.PurchaseLimit,
		AlertThreshold: req.AlertThreshold,
	}

	created, err := h.tierRepo.Create(c.Request.Context(), tier)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "reward_tiers", created.ID, "CREATE", nil, created)
	}

	c.JSON(http.StatusCreated, created)
}

// GET /api/v1/tiers/:id
func (h *TierHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid tier ID"})
		return
	}

	tier, err := h.tierRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, tier)
}

// PUT /api/v1/tiers/:id
func (h *TierHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid tier ID"})
		return
	}

	var req updateTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	tier := &domain.RewardTier{
		ID:             id,
		Name:           req.Name,
		Description:    req.Description,
		InventoryCount: req.InventoryCount,
		PurchaseLimit:  req.PurchaseLimit,
		AlertThreshold: req.AlertThreshold,
		Version:        req.Version,
	}

	updated, err := h.tierRepo.Update(c.Request.Context(), tier)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "reward_tiers", updated.ID, "UPDATE", nil, updated)
	}

	c.JSON(http.StatusOK, updated)
}

// DELETE /api/v1/tiers/:id
func (h *TierHandler) SoftDelete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid tier ID"})
		return
	}

	actorID, _ := c.Get("userID")
	deletedBy, _ := actorID.(uuid.UUID)

	if err := h.tierRepo.SoftDelete(c.Request.Context(), id, deletedBy); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "reward_tiers", id, "DELETE", nil, nil)
	}

	c.Status(http.StatusNoContent)
}

// POST /api/v1/tiers/:id/restore
func (h *TierHandler) Restore(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid tier ID"})
		return
	}

	if err := h.tierRepo.Restore(c.Request.Context(), id); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "reward_tiers", id, "RESTORE", nil, nil)
	}

	tier, err := h.tierRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, tier)
}

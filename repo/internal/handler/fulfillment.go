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
	"github.com/fulfillops/fulfillops/internal/service"
	"github.com/fulfillops/fulfillops/internal/util"
)

// FulfillmentHandler handles fulfillment endpoints.
type FulfillmentHandler struct {
	fulfillSvc   service.FulfillmentService
	fulfillRepo  repository.FulfillmentRepository
	timelineRepo repository.TimelineRepository
	encSvc       service.EncryptionService
}

func NewFulfillmentHandler(
	fulfillSvc service.FulfillmentService,
	fulfillRepo repository.FulfillmentRepository,
	timelineRepo repository.TimelineRepository,
	encSvc service.EncryptionService,
) *FulfillmentHandler {
	return &FulfillmentHandler{
		fulfillSvc:   fulfillSvc,
		fulfillRepo:  fulfillRepo,
		timelineRepo: timelineRepo,
		encSvc:       encSvc,
	}
}

type createFulfillmentRequest struct {
	TierID     uuid.UUID              `json:"tier_id" binding:"required"`
	CustomerID uuid.UUID              `json:"customer_id" binding:"required"`
	Type       domain.FulfillmentType `json:"type" binding:"required"`
}

type transitionShippingAddress struct {
	Line1   string `json:"line_1"`
	Line2   string `json:"line_2"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
}

type transitionRequest struct {
	ToStatus          domain.FulfillmentStatus   `json:"to_status" binding:"required"`
	Version           int                        `json:"version" binding:"required"`
	CarrierName       *string                    `json:"carrier_name"`
	TrackingNumber    *string                    `json:"tracking_number"`
	VoucherCode       *string                    `json:"voucher_code"`
	VoucherExpiration *time.Time                 `json:"voucher_expiration"`
	Reason            *string                    `json:"reason"`
	ShippingAddress   *transitionShippingAddress `json:"shipping_address"`
}

func (h *FulfillmentHandler) toResponse(f *domain.Fulfillment) *domain.FulfillmentResponse {
	resp := &domain.FulfillmentResponse{
		ID:                f.ID,
		TierID:            f.TierID,
		CustomerID:        f.CustomerID,
		Type:              f.Type,
		Status:            f.Status,
		CarrierName:       f.CarrierName,
		TrackingNumber:    f.TrackingNumber,
		VoucherExpiration: f.VoucherExpiration,
		HoldReason:        f.HoldReason,
		CancelReason:      f.CancelReason,
		ReadyAt:           f.ReadyAt,
		ShippedAt:         f.ShippedAt,
		DeliveredAt:       f.DeliveredAt,
		CompletedAt:       f.CompletedAt,
		Version:           f.Version,
		CreatedAt:         f.CreatedAt,
		UpdatedAt:         f.UpdatedAt,
	}

	if len(f.VoucherCodeEncrypted) > 0 {
		if plain, err := h.encSvc.DecryptToString(f.VoucherCodeEncrypted); err == nil {
			masked := util.MaskVoucherCode(plain)
			resp.VoucherCodeMasked = &masked
		}
	}

	return resp
}

// GET /api/v1/fulfillments
func (h *FulfillmentHandler) List(c *gin.Context) {
	filters := repository.FulfillmentFilters{}

	if s := c.Query("status"); s != "" {
		filters.Status = domain.FulfillmentStatus(s)
	}
	if s := c.Query("tier_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			filters.TierID = &id
		}
	}
	if s := c.Query("customer_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			filters.CustomerID = &id
		}
	}
	if s := c.Query("type"); s != "" {
		filters.Type = domain.FulfillmentType(s)
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

	fulfillments, total, err := h.fulfillRepo.List(c.Request.Context(), filters, pr)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	responses := make([]*domain.FulfillmentResponse, len(fulfillments))
	for i := range fulfillments {
		responses[i] = h.toResponse(&fulfillments[i])
	}

	c.JSON(http.StatusOK, domain.PageResponse[*domain.FulfillmentResponse]{
		Items:    responses,
		Total:    total,
		Page:     pr.Page,
		PageSize: pr.PageSize,
	})
}

// POST /api/v1/fulfillments
func (h *FulfillmentHandler) Create(c *gin.Context) {
	var req createFulfillmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	input := service.CreateFulfillmentInput{
		TierID:     req.TierID,
		CustomerID: req.CustomerID,
		Type:       req.Type,
	}

	created, err := h.fulfillSvc.Create(c.Request.Context(), input)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusCreated, h.toResponse(created))
}

// GET /api/v1/fulfillments/:id
func (h *FulfillmentHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid fulfillment ID"})
		return
	}

	f, err := h.fulfillRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, h.toResponse(f))
}

// POST /api/v1/fulfillments/:id/transition
func (h *FulfillmentHandler) Transition(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid fulfillment ID"})
		return
	}

	var req transitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	input := service.TransitionInput{
		FulfillmentID:     id,
		ToStatus:          req.ToStatus,
		ExpectedVersion:   req.Version,
		CarrierName:       req.CarrierName,
		TrackingNumber:    req.TrackingNumber,
		VoucherExpiration: req.VoucherExpiration,
		Reason:            req.Reason,
	}

	// Encrypt voucher code if provided.
	if req.VoucherCode != nil && *req.VoucherCode != "" {
		enc, encErr := h.encSvc.EncryptString(*req.VoucherCode)
		if encErr != nil {
			c.JSON(http.StatusInternalServerError, middleware.ErrorResponse{Code: "INTERNAL_ERROR", Message: "encryption error"})
			return
		}
		input.VoucherCode = enc
	}

	// Encrypt shipping address if provided.
	if req.ShippingAddress != nil {
		line1Enc, err1 := h.encSvc.EncryptString(req.ShippingAddress.Line1)
		if err1 != nil {
			c.JSON(http.StatusInternalServerError, middleware.ErrorResponse{Code: "INTERNAL_ERROR", Message: "encryption error"})
			return
		}
		var line2Enc []byte
		if req.ShippingAddress.Line2 != "" {
			line2Enc, err1 = h.encSvc.EncryptString(req.ShippingAddress.Line2)
			if err1 != nil {
				c.JSON(http.StatusInternalServerError, middleware.ErrorResponse{Code: "INTERNAL_ERROR", Message: "encryption error"})
				return
			}
		}
		input.ShippingAddr = &service.ShippingAddressEncrypted{
			Line1Encrypted: line1Enc,
			Line2Encrypted: line2Enc,
			City:           req.ShippingAddress.City,
			State:          req.ShippingAddress.State,
			ZipCode:        req.ShippingAddress.ZipCode,
		}
	}

	updated, err := h.fulfillSvc.Transition(c.Request.Context(), input)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, h.toResponse(updated))
}

// GET /api/v1/fulfillments/:id/timeline
func (h *FulfillmentHandler) Timeline(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid fulfillment ID"})
		return
	}

	events, err := h.timelineRepo.ListByFulfillmentID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": events})
}

// DELETE /api/v1/fulfillments/:id
func (h *FulfillmentHandler) SoftDelete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid fulfillment ID"})
		return
	}

	actorID, _ := c.Get("userID")
	deletedBy, _ := actorID.(uuid.UUID)

	if err := h.fulfillRepo.SoftDelete(c.Request.Context(), id, deletedBy); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// POST /api/v1/fulfillments/:id/restore
func (h *FulfillmentHandler) Restore(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid fulfillment ID"})
		return
	}

	if err := h.fulfillRepo.Restore(c.Request.Context(), id); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	f, err := h.fulfillRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, h.toResponse(f))
}

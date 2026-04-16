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
	"github.com/fulfillops/fulfillops/internal/util"
)

// CustomerHandler handles customer endpoints.
type CustomerHandler struct {
	customerRepo repository.CustomerRepository
	encSvc       service.EncryptionService
}

func NewCustomerHandler(customerRepo repository.CustomerRepository, encSvc service.EncryptionService) *CustomerHandler {
	return &CustomerHandler{customerRepo: customerRepo, encSvc: encSvc}
}

type createCustomerRequest struct {
	Name    string `json:"name" binding:"required"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	Address string `json:"address"`
}

type updateCustomerRequest struct {
	Name    string `json:"name" binding:"required"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	Address string `json:"address"`
	Version int    `json:"version" binding:"required"`
}

func (h *CustomerHandler) toResponse(c *domain.Customer) *domain.CustomerResponse {
	resp := &domain.CustomerResponse{
		ID:        c.ID,
		Name:      c.Name,
		Version:   c.Version,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}

	if len(c.PhoneEncrypted) > 0 {
		if plain, err := h.encSvc.DecryptToString(c.PhoneEncrypted); err == nil {
			resp.PhoneMasked = util.MaskPhone(plain)
		}
	}
	if len(c.EmailEncrypted) > 0 {
		if plain, err := h.encSvc.DecryptToString(c.EmailEncrypted); err == nil {
			resp.EmailMasked = util.MaskEmail(plain)
		}
	}
	if len(c.AddressEncrypted) > 0 {
		if plain, err := h.encSvc.DecryptToString(c.AddressEncrypted); err == nil {
			resp.AddressMasked = util.MaskAddress(plain)
		}
	}

	return resp
}

// GET /api/v1/customers
func (h *CustomerHandler) List(c *gin.Context) {
	name := c.DefaultQuery("name", "")
	includeDeleted := c.DefaultQuery("include_deleted", "false") == "true"

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	pr := domain.PageRequest{Page: page, PageSize: pageSize}
	pr.Normalize()

	customers, total, err := h.customerRepo.List(c.Request.Context(), name, pr, includeDeleted)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	responses := make([]*domain.CustomerResponse, len(customers))
	for i := range customers {
		responses[i] = h.toResponse(&customers[i])
	}

	c.JSON(http.StatusOK, domain.PageResponse[*domain.CustomerResponse]{
		Items:    responses,
		Total:    total,
		Page:     pr.Page,
		PageSize: pr.PageSize,
	})
}

// POST /api/v1/customers
func (h *CustomerHandler) Create(c *gin.Context) {
	var req createCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	customer := &domain.Customer{
		Name: req.Name,
	}

	if req.Phone != "" {
		enc, err := h.encSvc.EncryptString(req.Phone)
		if err != nil {
			c.JSON(http.StatusInternalServerError, middleware.ErrorResponse{Code: "INTERNAL_ERROR", Message: "encryption error"})
			return
		}
		customer.PhoneEncrypted = enc
	}
	if req.Email != "" {
		enc, err := h.encSvc.EncryptString(req.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, middleware.ErrorResponse{Code: "INTERNAL_ERROR", Message: "encryption error"})
			return
		}
		customer.EmailEncrypted = enc
	}
	if req.Address != "" {
		enc, err := h.encSvc.EncryptString(req.Address)
		if err != nil {
			c.JSON(http.StatusInternalServerError, middleware.ErrorResponse{Code: "INTERNAL_ERROR", Message: "encryption error"})
			return
		}
		customer.AddressEncrypted = enc
	}

	created, err := h.customerRepo.Create(c.Request.Context(), customer)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusCreated, h.toResponse(created))
}

// GET /api/v1/customers/:id
func (h *CustomerHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid customer ID"})
		return
	}

	customer, err := h.customerRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, h.toResponse(customer))
}

// PUT /api/v1/customers/:id
func (h *CustomerHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid customer ID"})
		return
	}

	var req updateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	customer := &domain.Customer{
		ID:      id,
		Name:    req.Name,
		Version: req.Version,
	}

	if req.Phone != "" {
		enc, err := h.encSvc.EncryptString(req.Phone)
		if err != nil {
			c.JSON(http.StatusInternalServerError, middleware.ErrorResponse{Code: "INTERNAL_ERROR", Message: "encryption error"})
			return
		}
		customer.PhoneEncrypted = enc
	}
	if req.Email != "" {
		enc, err := h.encSvc.EncryptString(req.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, middleware.ErrorResponse{Code: "INTERNAL_ERROR", Message: "encryption error"})
			return
		}
		customer.EmailEncrypted = enc
	}
	if req.Address != "" {
		enc, err := h.encSvc.EncryptString(req.Address)
		if err != nil {
			c.JSON(http.StatusInternalServerError, middleware.ErrorResponse{Code: "INTERNAL_ERROR", Message: "encryption error"})
			return
		}
		customer.AddressEncrypted = enc
	}

	updated, err := h.customerRepo.Update(c.Request.Context(), customer)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, h.toResponse(updated))
}

// DELETE /api/v1/customers/:id
func (h *CustomerHandler) SoftDelete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid customer ID"})
		return
	}

	actorID, _ := c.Get("userID")
	deletedBy, _ := actorID.(uuid.UUID)

	if err := h.customerRepo.SoftDelete(c.Request.Context(), id, deletedBy); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// POST /api/v1/customers/:id/restore
func (h *CustomerHandler) Restore(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid customer ID"})
		return
	}

	if err := h.customerRepo.Restore(c.Request.Context(), id); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	customer, err := h.customerRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, h.toResponse(customer))
}

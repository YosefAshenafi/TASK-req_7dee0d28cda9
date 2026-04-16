package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/middleware"
	"github.com/fulfillops/fulfillops/internal/service"
)

// UserHandler handles user management endpoints.
type UserHandler struct {
	userSvc service.UserService
}

func NewUserHandler(userSvc service.UserService) *UserHandler {
	return &UserHandler{userSvc: userSvc}
}

type createUserRequest struct {
	Username string          `json:"username" binding:"required"`
	Email    string          `json:"email" binding:"required"`
	Password string          `json:"password" binding:"required"`
	Role     domain.UserRole `json:"role" binding:"required"`
}

type updateUserRequest struct {
	Email string          `json:"email" binding:"required"`
	Role  domain.UserRole `json:"role" binding:"required"`
}

type listUsersQuery struct {
	Role     domain.UserRole `form:"role"`
	IsActive *bool           `form:"is_active"`
}

// GET /api/v1/admin/users
func (h *UserHandler) List(c *gin.Context) {
	var q listUsersQuery
	_ = c.ShouldBindQuery(&q)

	users, err := h.userSvc.List(c.Request.Context(), q.Role, q.IsActive)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": users})
}

// POST /api/v1/admin/users
func (h *UserHandler) Create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	created, err := h.userSvc.CreateUser(c.Request.Context(), req.Username, req.Email, req.Password, req.Role)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusCreated, created)
}

// GET /api/v1/admin/users/:id
func (h *UserHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid user ID"})
		return
	}

	user, err := h.userSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

// PUT /api/v1/admin/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid user ID"})
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	updated, err := h.userSvc.UpdateUser(c.Request.Context(), id, req.Email, req.Role)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, updated)
}

// DELETE /api/v1/admin/users/:id
func (h *UserHandler) Deactivate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid user ID"})
		return
	}

	if err := h.userSvc.DeactivateUser(c.Request.Context(), id); err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

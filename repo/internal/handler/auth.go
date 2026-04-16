package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/middleware"
	"github.com/fulfillops/fulfillops/internal/service"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	userSvc service.UserService
	store   sessions.Store
}

func NewAuthHandler(userSvc service.UserService, store sessions.Store) *AuthHandler {
	return &AuthHandler{userSvc: userSvc, store: store}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	user, err := h.userSvc.Authenticate(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		middleware.DomainErrorToHTTP(c, domain.ErrUnauthorized)
		return
	}

	if err := middleware.SetSession(c, h.store, user.ID, user.Role); err != nil {
		c.JSON(http.StatusInternalServerError, middleware.ErrorResponse{Code: "INTERNAL_ERROR", Message: "session error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
}

// GET /api/v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get("userID")
	user, err := h.userSvc.GetByID(c.Request.Context(), userID.(uuid.UUID))
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":        user.ID,
		"username":  user.Username,
		"email":     user.Email,
		"role":      user.Role,
		"is_active": user.IsActive,
	})
}

// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	if err := middleware.ClearSession(c, h.store); err != nil {
		c.JSON(http.StatusInternalServerError, middleware.ErrorResponse{Code: "INTERNAL_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

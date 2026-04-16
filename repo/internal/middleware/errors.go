package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fulfillops/fulfillops/internal/domain"
)

// ErrorResponse is the standard JSON error shape.
type ErrorResponse struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// DomainErrorToHTTP maps domain errors to HTTP status codes and writes JSON.
func DomainErrorToHTTP(c *gin.Context, err error) {
	var de *domain.DomainError
	if errors.As(err, &de) {
		status := domainCodeToStatus(de.Code)
		c.AbortWithStatusJSON(status, ErrorResponse{
			Code:    de.Code,
			Message: de.Message,
			Details: de.Details,
		})
		return
	}

	switch {
	case errors.Is(err, domain.ErrNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, ErrorResponse{Code: "NOT_FOUND", Message: err.Error()})
	case errors.Is(err, domain.ErrConflict):
		c.AbortWithStatusJSON(http.StatusConflict, ErrorResponse{Code: "CONFLICT", Message: err.Error()})
	case errors.Is(err, domain.ErrInventoryUnavailable):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorResponse{Code: "INVENTORY_UNAVAILABLE", Message: err.Error()})
	case errors.Is(err, domain.ErrPurchaseLimitReached):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorResponse{Code: "PURCHASE_LIMIT_REACHED", Message: err.Error()})
	case errors.Is(err, domain.ErrInvalidTransition):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorResponse{Code: "INVALID_TRANSITION", Message: err.Error()})
	case errors.Is(err, domain.ErrValidation):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()})
	case errors.Is(err, domain.ErrUnauthorized):
		c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Code: "UNAUTHORIZED", Message: "authentication required"})
	case errors.Is(err, domain.ErrForbidden):
		c.AbortWithStatusJSON(http.StatusForbidden, ErrorResponse{Code: "FORBIDDEN", Message: "insufficient permissions"})
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"})
	}
}

func domainCodeToStatus(code string) int {
	switch code {
	case "NOT_FOUND":
		return http.StatusNotFound
	case "CONFLICT":
		return http.StatusConflict
	case "INVENTORY_UNAVAILABLE", "PURCHASE_LIMIT_REACHED", "INVALID_TRANSITION", "VALIDATION_ERROR":
		return http.StatusUnprocessableEntity
	case "UNAUTHORIZED":
		return http.StatusUnauthorized
	case "FORBIDDEN":
		return http.StatusForbidden
	case "ALREADY_EXISTS":
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

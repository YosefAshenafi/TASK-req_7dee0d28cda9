package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/service"
)

const sessionName = "fulfillops_session"
const sessionUserID = "user_id"
const sessionUserRole = "user_role"

// PageSessionName is the cookie name used for browser page sessions.
const PageSessionName = "fulfillops"

// SessionStore key in gin context.
const SessionStoreKey = "session_store"

// UserLookup fetches the current persisted record for a user. Middleware uses
// it on every request so deactivations and role changes take effect immediately
// instead of waiting for the cookie to expire.
type UserLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// SessionAuth validates the session cookie and injects user info into context.
// When userLookup is non-nil, the user is reloaded from the database on every
// request — deactivated users are rejected and the role used by downstream
// authorization comes from the database, not the cookie payload.
func SessionAuth(store sessions.Store, userLookup UserLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, err := store.Get(c.Request, sessionName)
		if err != nil || sess.IsNew {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
				Code:    "UNAUTHORIZED",
				Message: "authentication required",
			})
			return
		}

		rawID, ok := sess.Values[sessionUserID]
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
				Code:    "UNAUTHORIZED",
				Message: "session invalid",
			})
			return
		}

		userIDStr, ok := rawID.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Code: "UNAUTHORIZED", Message: "session corrupted"})
			return
		}
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Code: "UNAUTHORIZED", Message: "invalid session user"})
			return
		}

		role, _ := sess.Values[sessionUserRole].(string)
		effectiveRole := domain.UserRole(role)

		if userLookup != nil {
			u, err := userLookup.GetByID(c.Request.Context(), userID)
			if err != nil || u == nil || !u.IsActive {
				sess.Options.MaxAge = -1
				_ = sess.Save(c.Request, c.Writer)
				c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
					Code:    "UNAUTHORIZED",
					Message: "session revoked",
				})
				return
			}
			effectiveRole = u.Role
		}

		// Inject into request context for downstream handlers.
		ctx := service.WithUserID(c.Request.Context(), userID)
		c.Request = c.Request.WithContext(ctx)
		c.Set("userID", userID)
		c.Set("userRole", effectiveRole)
		c.Next()
	}
}

// RequireRole returns a middleware that allows only the specified roles.
func RequireRole(roles ...domain.UserRole) gin.HandlerFunc {
	allowed := make(map[domain.UserRole]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		role, exists := c.Get("userRole")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, ErrorResponse{Code: "FORBIDDEN", Message: "insufficient permissions"})
			return
		}
		if !allowed[role.(domain.UserRole)] {
			c.AbortWithStatusJSON(http.StatusForbidden, ErrorResponse{Code: "FORBIDDEN", Message: "insufficient permissions"})
			return
		}
		c.Next()
	}
}

// SetSession writes user credentials into the session cookie.
func SetSession(c *gin.Context, store sessions.Store, userID uuid.UUID, role domain.UserRole) error {
	sess, err := store.Get(c.Request, sessionName)
	if err != nil {
		return err
	}
	sess.Values[sessionUserID] = userID.String()
	sess.Values[sessionUserRole] = string(role)
	return store.Save(c.Request, c.Writer, sess)
}

// PageSessionAuth is like SessionAuth but redirects to /auth/login instead of
// returning a JSON 401. When userLookup is non-nil, the user is reloaded from
// the database on every request so deactivations take effect immediately.
func PageSessionAuth(store sessions.Store, userLookup UserLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, err := store.Get(c.Request, PageSessionName)
		if err != nil || sess.IsNew {
			c.Redirect(http.StatusSeeOther, "/auth/login")
			c.Abort()
			return
		}
		rawID, ok := sess.Values["userID"].(string)
		if !ok || rawID == "" {
			c.Redirect(http.StatusSeeOther, "/auth/login")
			c.Abort()
			return
		}

		if userLookup != nil {
			uid, err := uuid.Parse(rawID)
			if err != nil {
				sess.Options.MaxAge = -1
				_ = sess.Save(c.Request, c.Writer)
				c.Redirect(http.StatusSeeOther, "/auth/login")
				c.Abort()
				return
			}
			u, err := userLookup.GetByID(c.Request.Context(), uid)
			if err != nil || u == nil || !u.IsActive {
				sess.Options.MaxAge = -1
				_ = sess.Save(c.Request, c.Writer)
				c.Redirect(http.StatusSeeOther, "/auth/login")
				c.Abort()
				return
			}
			// Refresh role in the session so PageRequireRole always sees DB truth.
			sess.Values["userRole"] = string(u.Role)
			_ = sess.Save(c.Request, c.Writer)
		}

		c.Next()
	}
}

// PageRequireRole is like RequireRole but reads role from the page session.
func PageRequireRole(store sessions.Store, roles ...domain.UserRole) gin.HandlerFunc {
	allowed := make(map[domain.UserRole]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		sess, _ := store.Get(c.Request, PageSessionName)
		role, _ := sess.Values["userRole"].(string)
		if !allowed[domain.UserRole(role)] {
			c.Status(http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// ClearSession destroys the API session cookie.
func ClearSession(c *gin.Context, store sessions.Store) error {
	sess, err := store.Get(c.Request, sessionName)
	if err != nil {
		return err
	}
	sess.Options.MaxAge = -1
	return store.Save(c.Request, c.Writer, sess)
}

// ClearPageSession destroys the page session cookie.
func ClearPageSession(c *gin.Context, store sessions.Store) error {
	sess, err := store.Get(c.Request, PageSessionName)
	if err != nil {
		return err
	}
	sess.Options.MaxAge = -1
	return store.Save(c.Request, c.Writer, sess)
}

// ClearAllSessions destroys both the API and page cookies — used by both
// logout paths to keep session state consistent across surfaces.
func ClearAllSessions(c *gin.Context, store sessions.Store) {
	_ = ClearSession(c, store)
	_ = ClearPageSession(c, store)
}

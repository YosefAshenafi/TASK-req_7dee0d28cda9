package middleware

import (
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

// SessionStore key in gin context.
const SessionStoreKey = "session_store"

// SessionAuth validates the session cookie and injects user info into context.
func SessionAuth(store sessions.Store) gin.HandlerFunc {
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

		// Inject into request context for downstream handlers.
		ctx := service.WithUserID(c.Request.Context(), userID)
		c.Request = c.Request.WithContext(ctx)
		c.Set("userID", userID)
		c.Set("userRole", domain.UserRole(role))
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
// returning a JSON 401. It reads the "fulfillops" cookie used by page handlers.
func PageSessionAuth(store sessions.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, err := store.Get(c.Request, "fulfillops")
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
		c.Next()
	}
}

// PageRequireRole is like RequireRole but reads role from the "fulfillops" page session.
func PageRequireRole(store sessions.Store, roles ...domain.UserRole) gin.HandlerFunc {
	allowed := make(map[domain.UserRole]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		sess, _ := store.Get(c.Request, "fulfillops")
		role, _ := sess.Values["userRole"].(string)
		if !allowed[domain.UserRole(role)] {
			c.Status(http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// ClearSession destroys the session cookie.
func ClearSession(c *gin.Context, store sessions.Store) error {
	sess, err := store.Get(c.Request, sessionName)
	if err != nil {
		return err
	}
	sess.Options.MaxAge = -1
	return store.Save(c.Request, c.Writer, sess)
}

package handler

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/service"
	"github.com/fulfillops/fulfillops/internal/view"
)

const (
	flashKey    = "flash_type"
	flashMsgKey = "flash_msg"
)

// renderPage writes a templ component as an HTTP response.
func renderPage(c *gin.Context, status int, comp templ.Component) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	if err := comp.Render(c.Request.Context(), c.Writer); err != nil {
		_ = c.Error(err)
	}
}

// pageCtx extracts NavUser and Flash from the gin context / session.
func pageCtx(c *gin.Context, store sessions.Store) view.PageCtx {
	sess, _ := store.Get(c.Request, "fulfillops")

	var user view.NavUser
	if rawID, ok := sess.Values["userID"].(string); ok {
		if id, err := uuid.Parse(rawID); err == nil {
			user.ID = id
		}
	}
	if rawUsername, ok := sess.Values["username"].(string); ok {
		user.Username = rawUsername
	}
	if rawRole, ok := sess.Values["userRole"].(string); ok {
		user.Role = domain.UserRole(rawRole)
	}

	var flash *view.Flash
	if ft, ok := sess.Values[flashKey].(string); ok && ft != "" {
		if fm, ok2 := sess.Values[flashMsgKey].(string); ok2 && fm != "" {
			flash = &view.Flash{Type: ft, Message: fm}
			delete(sess.Values, flashKey)
			delete(sess.Values, flashMsgKey)
			_ = sess.Save(c.Request, c.Writer)
		}
	}

	return view.PageCtx{User: user, Flash: flash}
}

// setFlash stores a flash message in the session for the next request.
func setFlash(c *gin.Context, store sessions.Store, flashType, message string) {
	sess, _ := store.Get(c.Request, "fulfillops")
	sess.Values[flashKey] = flashType
	sess.Values[flashMsgKey] = message
	_ = sess.Save(c.Request, c.Writer)
}

// redirectWithFlash sets a flash then redirects.
func redirectWithFlash(c *gin.Context, store sessions.Store, to, flashType, message string) {
	setFlash(c, store, flashType, message)
	c.Redirect(http.StatusSeeOther, to)
}

// isAdmin returns true when the current session user is an Administrator.
func isAdmin(c *gin.Context, store sessions.Store) bool {
	sess, _ := store.Get(c.Request, "fulfillops")
	role, _ := sess.Values["userRole"].(string)
	return domain.UserRole(role) == domain.RoleAdministrator
}

// canEdit returns true when user is Admin or Specialist.
func canEdit(c *gin.Context, store sessions.Store) bool {
	sess, _ := store.Get(c.Request, "fulfillops")
	role, _ := sess.Values["userRole"].(string)
	r := domain.UserRole(role)
	return r == domain.RoleAdministrator || r == domain.RoleFulfillmentSpecialist
}

func currentPageUserID(c *gin.Context, store sessions.Store) (uuid.UUID, bool) {
	sess, _ := store.Get(c.Request, "fulfillops")
	rawID, _ := sess.Values["userID"].(string)
	if rawID == "" {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		return uuid.UUID{}, false
	}
	return id, true
}

func pageRequestContextWithUser(c *gin.Context, store sessions.Store) context.Context {
	ctx := c.Request.Context()
	if userID, ok := currentPageUserID(c, store); ok {
		ctx = service.WithUserID(ctx, userID)
	}
	return ctx
}

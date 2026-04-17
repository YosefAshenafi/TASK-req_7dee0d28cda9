package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/middleware"
	"github.com/fulfillops/fulfillops/internal/service"
	authview "github.com/fulfillops/fulfillops/internal/view/auth"
)

type PageAuthHandler struct {
	userSvc service.UserService
	store   sessions.Store
}

func NewPageAuthHandler(userSvc service.UserService, store sessions.Store) *PageAuthHandler {
	return &PageAuthHandler{userSvc: userSvc, store: store}
}

func (h *PageAuthHandler) ShowLogin(c *gin.Context) {
	renderPage(c, http.StatusOK, authview.Login(""))
}

func (h *PageAuthHandler) PostLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	user, err := h.userSvc.Authenticate(c.Request.Context(), username, password)
	if err != nil {
		renderPage(c, http.StatusUnauthorized, authview.Login("Invalid username or password."))
		return
	}

	sess, _ := h.store.Get(c.Request, middleware.PageSessionName)
	// Rotate the session ID to prevent session fixation.
	sess.ID = ""
	sess.Values["userID"] = user.ID.String()
	sess.Values["username"] = user.Username
	sess.Values["userRole"] = string(user.Role)
	_ = sess.Save(c.Request, c.Writer)

	// Also set the API cookie so the page and API surfaces share session state.
	_ = middleware.SetSession(c, h.store, user.ID, user.Role)

	if user.MustRotatePassword {
		c.Redirect(http.StatusSeeOther, "/auth/change-password")
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

func (h *PageAuthHandler) ShowChangePassword(c *gin.Context) {
	renderPage(c, http.StatusOK, authview.ChangePassword(""))
}

func (h *PageAuthHandler) PostChangePassword(c *gin.Context) {
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	if newPassword != confirmPassword {
		renderPage(c, http.StatusUnprocessableEntity, authview.ChangePassword("Passwords do not match."))
		return
	}

	sess, _ := h.store.Get(c.Request, middleware.PageSessionName)
	rawID, _ := sess.Values["userID"].(string)
	userID, err := uuid.Parse(rawID)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/auth/login")
		return
	}

	// Load the current user to verify they still require rotation.
	user, err := h.userSvc.GetByID(c.Request.Context(), userID)
	if err != nil {
		renderPage(c, http.StatusUnprocessableEntity, authview.ChangePassword("Session expired. Please log in again."))
		return
	}

	// Reject if the new password equals the bootstrapped default — force a real change.
	if _, authErr := h.userSvc.Authenticate(c.Request.Context(), user.Username, newPassword); authErr == nil && user.MustRotatePassword {
		// Successfully authenticated with the new password against existing hash means
		// the user tried to "change" to the same credential as the bootstrap password.
		renderPage(c, http.StatusUnprocessableEntity, authview.ChangePassword("New password must differ from the initial bootstrap password."))
		return
	}

	if err := h.userSvc.ChangePassword(c.Request.Context(), userID, newPassword); err != nil {
		renderPage(c, http.StatusUnprocessableEntity, authview.ChangePassword(err.Error()))
		return
	}

	redirectWithFlash(c, h.store, "/", "success", "Password updated. Welcome!")
}

func (h *PageAuthHandler) PostLogout(c *gin.Context) {
	// Clear both the page cookie and the API cookie so page/API session state
	// stays consistent when the user logs out.
	middleware.ClearAllSessions(c, h.store)
	c.Redirect(http.StatusSeeOther, "/auth/login")
}

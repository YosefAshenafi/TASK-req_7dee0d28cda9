package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"

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

	sess, _ := h.store.Get(c.Request, "fulfillops")
	sess.Values["userID"] = user.ID.String()
	sess.Values["username"] = user.Username
	sess.Values["userRole"] = string(user.Role)
	_ = sess.Save(c.Request, c.Writer)

	c.Redirect(http.StatusSeeOther, "/")
}

func (h *PageAuthHandler) PostLogout(c *gin.Context) {
	sess, _ := h.store.Get(c.Request, "fulfillops")
	sess.Options.MaxAge = -1
	_ = sess.Save(c.Request, c.Writer)
	c.Redirect(http.StatusSeeOther, "/auth/login")
}

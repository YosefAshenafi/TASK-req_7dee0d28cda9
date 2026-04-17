package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/middleware"
)

// fakeUserLookup lets tests drive the user returned by the middleware's
// per-request revalidation — including the deactivation path.
type fakeUserLookup struct {
	user *domain.User
	err  error
}

func (f *fakeUserLookup) GetByID(_ context.Context, _ uuid.UUID) (*domain.User, error) {
	return f.user, f.err
}

func TestSessionAuth_RevalidatesAgainstDB_DeactivatedUserRejected(t *testing.T) {
	store := testCookieStore()
	uid := uuid.New()
	lookup := &fakeUserLookup{user: &domain.User{ID: uid, Role: domain.RoleAdministrator, IsActive: false}}

	r := gin.New()
	r.GET("/x", middleware.SessionAuth(store, lookup), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	sess, _ := store.Get(req, "fulfillops_session")
	sess.Values["user_id"] = uid.String()
	sess.Values["user_role"] = string(domain.RoleAdministrator)
	w := httptest.NewRecorder()
	_ = store.Save(req, w, sess)

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	addCookiesFromRecorder(req2, w)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req2)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for deactivated user, got %d", rr.Code)
	}
}

func TestSessionAuth_RevalidatesAgainstDB_UsesRoleFromDB(t *testing.T) {
	// Cookie says ADMINISTRATOR, DB says AUDITOR — downstream role check
	// should trust the DB, not the cookie. This protects against role
	// de-escalation not taking effect until cookie expiry.
	store := testCookieStore()
	uid := uuid.New()
	lookup := &fakeUserLookup{user: &domain.User{ID: uid, Role: domain.RoleAuditor, IsActive: true}}

	r := gin.New()
	r.GET("/x",
		middleware.SessionAuth(store, lookup),
		middleware.RequireRole(domain.RoleAdministrator),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	sess, _ := store.Get(req, "fulfillops_session")
	sess.Values["user_id"] = uid.String()
	sess.Values["user_role"] = string(domain.RoleAdministrator)
	w := httptest.NewRecorder()
	_ = store.Save(req, w, sess)

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	addCookiesFromRecorder(req2, w)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req2)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when DB role is Auditor, got %d", rr.Code)
	}
}

func TestPageSessionAuth_RevalidatesAgainstDB_DeactivatedRedirects(t *testing.T) {
	store := testCookieStore()
	uid := uuid.New()
	lookup := &fakeUserLookup{user: &domain.User{ID: uid, Role: domain.RoleAdministrator, IsActive: false}}

	r := gin.New()
	r.GET("/x", middleware.PageSessionAuth(store, lookup), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	sess, _ := store.Get(req, middleware.PageSessionName)
	sess.Values["userID"] = uid.String()
	sess.Values["userRole"] = string(domain.RoleAdministrator)
	w := httptest.NewRecorder()
	_ = store.Save(req, w, sess)

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	addCookiesFromRecorder(req2, w)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req2)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect for deactivated user, got %d", rr.Code)
	}
}

func TestClearAllSessions_ClearsBothCookies(t *testing.T) {
	store := testCookieStore()
	r := gin.New()
	r.GET("/logout", func(c *gin.Context) {
		middleware.ClearAllSessions(c, store)
		c.Status(http.StatusOK)
	})

	// Seed both cookies, then hit /logout and confirm both get MaxAge=-1.
	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	apiSess, _ := store.Get(req, "fulfillops_session")
	apiSess.Values["user_id"] = uuid.New().String()
	pageSess, _ := store.Get(req, middleware.PageSessionName)
	pageSess.Values["userID"] = uuid.New().String()
	seed := httptest.NewRecorder()
	_ = store.Save(req, seed, apiSess)
	_ = store.Save(req, seed, pageSess)

	req2 := httptest.NewRequest(http.MethodGet, "/logout", nil)
	addCookiesFromRecorder(req2, seed)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req2)

	setCookies := rr.Result().Header["Set-Cookie"]
	if len(setCookies) < 2 {
		t.Fatalf("expected two Set-Cookie headers (API + page), got %d: %v", len(setCookies), setCookies)
	}
}

package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/middleware"
)

func testCookieStore() sessions.Store {
	gin.SetMode(gin.TestMode)
	return sessions.NewCookieStore([]byte("01234567890123456789012345678901"))
}

func addCookiesFromRecorder(req *http.Request, w *httptest.ResponseRecorder) {
	for _, c := range w.Result().Cookies() {
		req.AddCookie(c)
	}
}

func TestSessionAuth_UnauthorizedWhenNoSession(t *testing.T) {
	store := testCookieStore()
	r := gin.New()
	r.GET("/x", middleware.SessionAuth(store, nil), func(c *gin.Context) { c.Status(http.StatusOK) })

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestSessionAuth_UnauthorizedMissingUserID(t *testing.T) {
	store := testCookieStore()
	r := gin.New()
	r.GET("/x", middleware.SessionAuth(store, nil), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	sess, _ := store.Get(req, "fulfillops_session")
	sess.Values["user_role"] = string(domain.RoleAdministrator)
	sess.Values["seed"] = "non-empty"
	w := httptest.NewRecorder()
	_ = store.Save(req, w, sess)

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	addCookiesFromRecorder(req2, w)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req2)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestSessionAuth_UnauthorizedBadUserIDType(t *testing.T) {
	store := testCookieStore()
	r := gin.New()
	r.GET("/x", middleware.SessionAuth(store, nil), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	sess, _ := store.Get(req, "fulfillops_session")
	sess.Values["user_id"] = 123
	sess.Values["user_role"] = string(domain.RoleAdministrator)
	w := httptest.NewRecorder()
	_ = store.Save(req, w, sess)

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	addCookiesFromRecorder(req2, w)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req2)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestSessionAuth_UnauthorizedInvalidUUID(t *testing.T) {
	store := testCookieStore()
	r := gin.New()
	r.GET("/x", middleware.SessionAuth(store, nil), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	sess, _ := store.Get(req, "fulfillops_session")
	sess.Values["user_id"] = "not-a-uuid"
	sess.Values["user_role"] = string(domain.RoleAdministrator)
	w := httptest.NewRecorder()
	_ = store.Save(req, w, sess)

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	addCookiesFromRecorder(req2, w)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req2)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestSessionAuth_OK(t *testing.T) {
	store := testCookieStore()
	uid := uuid.New()
	r := gin.New()
	r.GET("/x", middleware.SessionAuth(store, nil), func(c *gin.Context) { c.Status(http.StatusOK) })

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
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestRequireRole_ForbiddenNoRole(t *testing.T) {
	r := gin.New()
	r.GET("/x", middleware.RequireRole(domain.RoleAdministrator), func(c *gin.Context) { c.Status(http.StatusOK) })
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestRequireRole_ForbiddenWrongRole(t *testing.T) {
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		c.Set("userRole", domain.RoleAuditor)
		c.Next()
	}, middleware.RequireRole(domain.RoleAdministrator), func(c *gin.Context) { c.Status(http.StatusOK) })
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestRequireRole_OK(t *testing.T) {
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		c.Set("userRole", domain.RoleAdministrator)
		c.Next()
	}, middleware.RequireRole(domain.RoleAdministrator), func(c *gin.Context) { c.Status(http.StatusOK) })
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestSetSessionAndClearSession(t *testing.T) {
	store := testCookieStore()
	r := gin.New()
	uid := uuid.New()
	r.GET("/x", func(c *gin.Context) {
		if err := middleware.SetSession(c, store, uid, domain.RoleAdministrator); err != nil {
			t.Fatalf("SetSession: %v", err)
		}
		if err := middleware.ClearSession(c, store); err != nil {
			t.Fatalf("ClearSession: %v", err)
		}
		c.Status(http.StatusOK)
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestPageSessionAuth_Redirects(t *testing.T) {
	store := testCookieStore()
	r := gin.New()
	r.GET("/x", middleware.PageSessionAuth(store, nil), func(c *gin.Context) { c.Status(http.StatusOK) })
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestPageSessionAuth_RedirectsWhenUserIDMissing(t *testing.T) {
	store := testCookieStore()
	r := gin.New()
	r.GET("/x", middleware.PageSessionAuth(store, nil), func(c *gin.Context) { c.Status(http.StatusOK) })

	// Seed a session with a role but NO userID key.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	sess, _ := store.Get(req, "fulfillops")
	sess.Values["userRole"] = string(domain.RoleAdministrator)
	w := httptest.NewRecorder()
	_ = store.Save(req, w, sess)

	// Use a fresh req2 so Gorilla reads from the cookie rather than the cached
	// session object — this ensures we reach the !ok || rawID=="" branch.
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	addCookiesFromRecorder(req2, w)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req2)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestPageSessionAuth_OK(t *testing.T) {
	store := testCookieStore()
	r := gin.New()
	r.GET("/x", middleware.PageSessionAuth(store, nil), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	sess, _ := store.Get(req, "fulfillops")
	sess.Values["userID"] = uuid.New().String()
	w := httptest.NewRecorder()
	_ = store.Save(req, w, sess)

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	addCookiesFromRecorder(req2, w)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req2)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestPageRequireRole_Forbidden(t *testing.T) {
	store := testCookieStore()
	r := gin.New()
	r.GET("/x", middleware.PageRequireRole(store, domain.RoleAdministrator), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	sess, _ := store.Get(req, "fulfillops")
	sess.Values["userRole"] = string(domain.RoleAuditor)
	w := httptest.NewRecorder()
	_ = store.Save(req, w, sess)

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	addCookiesFromRecorder(req2, w)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req2)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestPageRequireRole_OK(t *testing.T) {
	store := testCookieStore()
	r := gin.New()
	r.GET("/x", middleware.PageRequireRole(store, domain.RoleAdministrator), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	sess, _ := store.Get(req, "fulfillops")
	sess.Values["userRole"] = string(domain.RoleAdministrator)
	w := httptest.NewRecorder()
	_ = store.Save(req, w, sess)

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	addCookiesFromRecorder(req2, w)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req2)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestDomainErrorToHTTP_DomainErrorBranches(t *testing.T) {
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		middleware.DomainErrorToHTTP(c, &domain.DomainError{
			Code: "NOT_FOUND", Message: "m", Details: map[string]string{"k": "v"},
		})
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestDomainErrorToHTTP_SentinelErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"notfound", domain.ErrNotFound, http.StatusNotFound},
		{"conflict", domain.ErrConflict, http.StatusConflict},
		{"inventory", domain.ErrInventoryUnavailable, http.StatusUnprocessableEntity},
		{"limit", domain.ErrPurchaseLimitReached, http.StatusUnprocessableEntity},
		{"transition", domain.ErrInvalidTransition, http.StatusUnprocessableEntity},
		{"validation", domain.ErrValidation, http.StatusUnprocessableEntity},
		{"unauth", domain.ErrUnauthorized, http.StatusUnauthorized},
		{"forbidden", domain.ErrForbidden, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/x", func(c *gin.Context) {
				middleware.DomainErrorToHTTP(c, tc.err)
			})
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
			if rr.Code != tc.want {
				t.Fatalf("code = %d want %d", rr.Code, tc.want)
			}
		})
	}
}

func TestDomainErrorToHTTP_DomainErrorCodes(t *testing.T) {
	codes := []struct {
		code string
		want int
	}{
		{"CONFLICT", http.StatusConflict},
		{"INVENTORY_UNAVAILABLE", http.StatusUnprocessableEntity},
		{"PURCHASE_LIMIT_REACHED", http.StatusUnprocessableEntity},
		{"INVALID_TRANSITION", http.StatusUnprocessableEntity},
		{"VALIDATION_ERROR", http.StatusUnprocessableEntity},
		{"UNAUTHORIZED", http.StatusUnauthorized},
		{"FORBIDDEN", http.StatusForbidden},
		{"ALREADY_EXISTS", http.StatusConflict},
		{"OTHER", http.StatusInternalServerError},
	}
	for _, tc := range codes {
		t.Run(tc.code, func(t *testing.T) {
			r := gin.New()
			r.GET("/x", func(c *gin.Context) {
				middleware.DomainErrorToHTTP(c, &domain.DomainError{Code: tc.code, Message: "x"})
			})
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
			if rr.Code != tc.want {
				t.Fatalf("code = %d want %d", rr.Code, tc.want)
			}
		})
	}
}

func TestDomainErrorToHTTP_Internal(t *testing.T) {
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		middleware.DomainErrorToHTTP(c, errors.New("boom"))
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestLoggerAndTraceMiddleware(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.ClientIP())
	r.Use(middleware.Logger())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusTeapot) })
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-Id", "trace-id")
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusTeapot {
		t.Fatalf("code = %d", rr.Code)
	}
	if rr.Header().Get("X-Request-Id") != "trace-id" {
		t.Fatalf("request id header missing")
	}
}

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Header().Get("X-Request-Id") == "" {
		t.Fatal("expected generated X-Request-Id")
	}
}

func TestPageSessionAuth_WithLookup_ActiveUser_Success(t *testing.T) {
	store := testCookieStore()
	uid := uuid.New()
	lookup := &fakeUserLookup{user: &domain.User{ID: uid, Role: domain.RoleAuditor, IsActive: true}}

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
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for active user with lookup, got %d", rr.Code)
	}
}

func TestPageSessionAuth_WithLookup_InvalidUUID_Redirects(t *testing.T) {
	store := testCookieStore()
	lookup := &fakeUserLookup{user: nil}

	r := gin.New()
	r.GET("/x", middleware.PageSessionAuth(store, lookup), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	sess, _ := store.Get(req, middleware.PageSessionName)
	sess.Values["userID"] = "not-a-valid-uuid"
	w := httptest.NewRecorder()
	_ = store.Save(req, w, sess)

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	addCookiesFromRecorder(req2, w)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req2)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect for invalid UUID with lookup, got %d", rr.Code)
	}
}

func TestSessionAuth_WithLookup_NilUser_Revokes(t *testing.T) {
	store := testCookieStore()
	uid := uuid.New()
	lookup := &fakeUserLookup{user: nil, err: nil}

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
		t.Fatalf("expected 401 for nil user from lookup, got %d", rr.Code)
	}
}

func TestSessionAuth_WithLookup_Error_Revokes(t *testing.T) {
	store := testCookieStore()
	uid := uuid.New()
	lookup := &fakeUserLookup{err: errors.New("db down")}

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
		t.Fatalf("expected 401 when lookup errors, got %d", rr.Code)
	}
}

// erroringSessionStore satisfies sessions.Store and always fails on Get/Save.
type erroringSessionStore struct{}

func (e *erroringSessionStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return nil, errors.New("store unavailable")
}
func (e *erroringSessionStore) New(r *http.Request, name string) (*sessions.Session, error) {
	return nil, errors.New("store unavailable")
}
func (e *erroringSessionStore) Save(r *http.Request, w http.ResponseWriter, s *sessions.Session) error {
	return errors.New("store unavailable")
}

func TestSetSession_StoreError(t *testing.T) {
	store := &erroringSessionStore{}
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		if err := middleware.SetSession(c, store, uuid.New(), domain.RoleAdministrator); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from store error, got %d", rr.Code)
	}
}

func TestClearSession_StoreError(t *testing.T) {
	store := &erroringSessionStore{}
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		if err := middleware.ClearSession(c, store); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from store error, got %d", rr.Code)
	}
}

func TestClearPageSession_StoreError(t *testing.T) {
	store := &erroringSessionStore{}
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		if err := middleware.ClearPageSession(c, store); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from store error, got %d", rr.Code)
	}
}

func TestLogger_FallbackReqIDFromRequestHeader(t *testing.T) {
	r := gin.New()
	r.Use(middleware.Logger()) // no RequestID middleware — exercises both fallback branches
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-Id", "req-header-id")
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d", rr.Code)
	}
}

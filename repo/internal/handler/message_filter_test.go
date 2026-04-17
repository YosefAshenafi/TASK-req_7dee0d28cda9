package handler

// Tests for Finding #4: message-center category/channel filters must be passed
// through to the template repository, not silently dropped.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
)

// captureTemplateRepo records the category/channel passed to List.
type captureTemplateRepo struct {
	calledCategory domain.TemplateCategory
	calledChannel  domain.SendLogChannel
	results        []domain.MessageTemplate
}

func (r *captureTemplateRepo) List(_ context.Context, cat domain.TemplateCategory, ch domain.SendLogChannel, _ bool) ([]domain.MessageTemplate, error) {
	r.calledCategory = cat
	r.calledChannel = ch
	return r.results, nil
}
func (r *captureTemplateRepo) GetByID(context.Context, uuid.UUID) (*domain.MessageTemplate, error) {
	return nil, domain.NewNotFoundError("template")
}
func (r *captureTemplateRepo) Create(_ context.Context, t *domain.MessageTemplate) (*domain.MessageTemplate, error) {
	return t, nil
}
func (r *captureTemplateRepo) Update(_ context.Context, t *domain.MessageTemplate) (*domain.MessageTemplate, error) {
	return t, nil
}
func (r *captureTemplateRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *captureTemplateRepo) Restore(context.Context, uuid.UUID) error               { return nil }

func TestListTemplates_FiltersPassedToRepo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &captureTemplateRepo{}
	store := sessions.NewCookieStore([]byte("test-secret-32bytes-long-padding!"))
	h := NewPageMessageHandler(store, repo, nil, nil)

	r := gin.New()
	r.GET("/messages", h.ListTemplates)

	req := httptest.NewRequest(http.MethodGet, "/messages?category=FULFILLMENT_PROGRESS&channel=EMAIL", nil)
	// attach a valid session cookie so pageCtx helpers don't panic
	cookie := sessionCookieFor(t, store, domain.RoleAdministrator)
	req.AddCookie(cookie)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if repo.calledCategory != domain.CategoryFulfillmentProgress {
		t.Errorf("category not forwarded: got %q", repo.calledCategory)
	}
	if repo.calledChannel != domain.ChannelEmail {
		t.Errorf("channel not forwarded: got %q", repo.calledChannel)
	}
}

func TestListTemplates_NoFiltersPassEmptyStrings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &captureTemplateRepo{}
	store := sessions.NewCookieStore([]byte("test-secret-32bytes-long-padding!"))
	h := NewPageMessageHandler(store, repo, nil, nil)

	r := gin.New()
	r.GET("/messages", h.ListTemplates)

	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	cookie := sessionCookieFor(t, store, domain.RoleAdministrator)
	req.AddCookie(cookie)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if repo.calledCategory != "" {
		t.Errorf("expected empty category, got %q", repo.calledCategory)
	}
	if repo.calledChannel != "" {
		t.Errorf("expected empty channel, got %q", repo.calledChannel)
	}
}

func TestListTemplates_FilterReflectedInResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmplID := uuid.New()
	repo := &captureTemplateRepo{
		results: []domain.MessageTemplate{
			{ID: tmplID, Name: "Progress Alert", Category: domain.CategoryFulfillmentProgress, Channel: domain.ChannelEmail, BodyTemplate: "hi"},
		},
	}
	store := sessions.NewCookieStore([]byte("test-secret-32bytes-long-padding!"))
	h := NewPageMessageHandler(store, repo, nil, nil)

	r := gin.New()
	r.GET("/messages", h.ListTemplates)

	req := httptest.NewRequest(http.MethodGet, "/messages?category=FULFILLMENT_PROGRESS&channel=EMAIL", nil)
	cookie := sessionCookieFor(t, store, domain.RoleAdministrator)
	req.AddCookie(cookie)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Progress Alert") {
		t.Errorf("response does not contain filtered template name; body excerpt: %.200s", body)
	}
}

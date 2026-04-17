package handler

// Unit test for Issue 6: unsupported report_type values must be rejected by the
// handler synchronously instead of returning 201 and failing asynchronously in
// the export goroutine.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// stubReportRepo is a minimal ReportExportRepository for handler unit tests.
type stubReportRepo struct {
	created *domain.ReportExport
}

func (r *stubReportRepo) Create(_ context.Context, e *domain.ReportExport) (*domain.ReportExport, error) {
	e.ID = uuid.New()
	r.created = e
	return e, nil
}

func (r *stubReportRepo) GetByID(context.Context, uuid.UUID) (*domain.ReportExport, error) {
	return nil, domain.NewNotFoundError("report")
}

func (r *stubReportRepo) List(context.Context, repository.ReportExportFilters, domain.PageRequest) ([]domain.ReportExport, int, error) {
	return nil, 0, nil
}

func (r *stubReportRepo) UpdateStatus(context.Context, uuid.UUID, domain.ExportStatus, *string, *int64, *string, *time.Time) error {
	return nil
}

func (r *stubReportRepo) GetExpired(context.Context, time.Time) ([]domain.ReportExport, error) {
	return nil, nil
}

func (r *stubReportRepo) Delete(context.Context, uuid.UUID) error { return nil }

func TestReportHandler_Create_RejectsUnsupportedReportType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &stubReportRepo{}
	h := NewReportHandler(repo, nil, nil)

	r := gin.New()
	r.POST("/reports/exports", func(c *gin.Context) {
		c.Set("userID", uuid.New())
		c.Set("userRole", domain.RoleAdministrator)
		h.Create(c)
	})

	payload := map[string]any{
		"report_type":       "fake-report-type",
		"include_sensitive": false,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/reports/exports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid report_type, got %d\nbody: %s", rec.Code, rec.Body.String())
	}
	if repo.created != nil {
		t.Fatal("unsupported type must not create a report row")
	}
}

func TestReportHandler_Create_AcceptsSupportedReportTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, rt := range []string{"fulfillments", "customers", "audit"} {
		t.Run(rt, func(t *testing.T) {
			repo := &stubReportRepo{}
			h := NewReportHandler(repo, nil, nil)

			r := gin.New()
			r.POST("/reports/exports", func(c *gin.Context) {
				c.Set("userID", uuid.New())
				c.Set("userRole", domain.RoleAdministrator)
				h.Create(c)
			})

			payload := map[string]any{
				"report_type":       rt,
				"include_sensitive": false,
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/reports/exports", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("expected 201 for %q, got %d\nbody: %s", rt, rec.Code, rec.Body.String())
			}
			if repo.created == nil {
				t.Fatalf("expected Create to run for %q", rt)
			}
		})
	}
}

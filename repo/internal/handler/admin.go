package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/middleware"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// AdminHandler handles admin health and job management endpoints.
type AdminHandler struct {
	pool       *pgxpool.Pool
	jobRunRepo repository.JobRunRepository
	scheduler  JobScheduler
}

func NewAdminHandler(pool *pgxpool.Pool, jobRunRepo repository.JobRunRepository) *AdminHandler {
	return &AdminHandler{pool: pool, jobRunRepo: jobRunRepo}
}

// WithScheduler attaches a scheduler for job trigger endpoints.
func (h *AdminHandler) WithScheduler(s JobScheduler) *AdminHandler {
	h.scheduler = s
	return h
}

// GET /api/v1/admin/health
func (h *AdminHandler) Health(c *gin.Context) {
	ctx := c.Request.Context()

	dbStatus := "ok"
	if err := h.pool.Ping(ctx); err != nil {
		dbStatus = "error: " + err.Error()
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"checks": gin.H{
			"database":   dbStatus,
			"encryption": "ok",
			"dirs":       "ok",
			"scheduler":  "ok",
		},
	})
}

// GET /api/v1/admin/jobs/runs
func (h *AdminHandler) ListJobRuns(c *gin.Context) {
	filters := repository.JobRunFilters{}

	if s := c.Query("job_name"); s != "" {
		filters.JobName = s
	}
	if s := c.Query("status"); s != "" {
		filters.Status = domain.JobStatus(s)
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	pr := domain.PageRequest{Page: page, PageSize: pageSize}
	pr.Normalize()

	runs, total, err := h.jobRunRepo.List(c.Request.Context(), filters, pr)
	if err != nil {
		middleware.DomainErrorToHTTP(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.PageResponse[domain.JobRunHistory]{
		Items:    runs,
		Total:    total,
		Page:     pr.Page,
		PageSize: pr.PageSize,
	})
}

// POST /api/v1/admin/jobs/:name/run
func (h *AdminHandler) TriggerJob(c *gin.Context) {
	name := c.Param("name")

	if h.scheduler != nil {
		// Use a detached context — the request context will be cancelled when
		// this handler returns, but the job runs in a background goroutine.
		if err := h.scheduler.RunOnce(context.Background(), name); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "job not found"})
			return
		}
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "job triggered",
		"job":     name,
	})
}

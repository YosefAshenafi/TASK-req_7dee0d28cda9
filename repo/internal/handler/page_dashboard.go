package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/view"
)

type PageDashboardHandler struct {
	store       sessions.Store
	fulfillRepo repository.FulfillmentRepository
	tierRepo    repository.TierRepository
	sendLogRepo repository.SendLogRepository
	exRepo      repository.ExceptionRepository
}

func NewPageDashboardHandler(
	store sessions.Store,
	fulfillRepo repository.FulfillmentRepository,
	tierRepo repository.TierRepository,
	sendLogRepo repository.SendLogRepository,
	exRepo repository.ExceptionRepository,
) *PageDashboardHandler {
	return &PageDashboardHandler{
		store: store, fulfillRepo: fulfillRepo, tierRepo: tierRepo,
		sendLogRepo: sendLogRepo, exRepo: exRepo,
	}
}

// overdueExceptionAge defines when an OPEN exception counts as overdue — any
// exception still open longer than this is surfaced on the dashboard so the
// fulfillment team can intervene. Kept at 24h so "overdue" on the console
// matches the operational SLA callout on exceptions day-to-day.
const overdueExceptionAge = 24 * time.Hour

func (h *PageDashboardHandler) Show(c *gin.Context) {
	ctx := c.Request.Context()
	pctx := pageCtx(c, h.store)

	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Pending = today's fulfillments currently in DRAFT or READY_TO_SHIP.
	_, pendingDraft, _ := h.fulfillRepo.List(ctx, repository.FulfillmentFilters{
		Status:   domain.StatusDraft,
		DateFrom: &startOfDay,
		DateTo:   &now,
	}, domain.PageRequest{Page: 1, PageSize: 1})
	_, pendingReady, _ := h.fulfillRepo.List(ctx, repository.FulfillmentFilters{
		Status:   domain.StatusReadyToShip,
		DateFrom: &startOfDay,
		DateTo:   &now,
	}, domain.PageRequest{Page: 1, PageSize: 1})

	// Overdue exceptions = OPEN exceptions whose created_at is older than the
	// overdue threshold, not merely anything open.
	overdueCutoff := now.Add(-overdueExceptionAge)
	openExceptions, _ := h.exRepo.List(ctx, repository.ExceptionFilters{
		Status:   domain.ExceptionOpen,
		OpenedTo: &overdueCutoff,
	})

	tiers, _ := h.tierRepo.List(ctx, "", false)
	var alerts []domain.RewardTier
	for _, t := range tiers {
		if t.InventoryCount <= t.AlertThreshold {
			alerts = append(alerts, t)
		}
	}

	_, fulfilledToday, _ := h.fulfillRepo.List(ctx, repository.FulfillmentFilters{
		Status:   domain.StatusCompleted,
		DateFrom: &startOfDay,
		DateTo:   &now,
	}, domain.PageRequest{Page: 1, PageSize: 1})

	// True "queued messages" = actual QUEUED rows in send_logs (not retryable).
	_, queuedCount, _ := h.sendLogRepo.List(ctx, repository.SendLogFilters{
		Status: domain.SendQueued,
	}, domain.PageRequest{Page: 1, PageSize: 1})

	d := view.DashboardData{
		PendingCount:      pendingDraft + pendingReady,
		OverdueExceptions: len(openExceptions),
		ThresholdAlerts:   alerts,
		FulfilledToday:    fulfilledToday,
		QueuedMessages:    queuedCount,
	}

	renderPage(c, http.StatusOK, view.Dashboard(pctx, d))
}

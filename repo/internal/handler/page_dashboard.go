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

func (h *PageDashboardHandler) Show(c *gin.Context) {
	ctx := c.Request.Context()
	pctx := pageCtx(c, h.store)

	_, pendingDraft, _ := h.fulfillRepo.List(ctx, repository.FulfillmentFilters{
		Status: domain.StatusDraft,
	}, domain.PageRequest{Page: 1, PageSize: 1})
	_, pendingReady, _ := h.fulfillRepo.List(ctx, repository.FulfillmentFilters{
		Status: domain.StatusReadyToShip,
	}, domain.PageRequest{Page: 1, PageSize: 1})

	openExceptions, _ := h.exRepo.List(ctx, repository.ExceptionFilters{
		Status: domain.ExceptionOpen,
	})

	tiers, _ := h.tierRepo.List(ctx, "", false)
	var alerts []domain.RewardTier
	for _, t := range tiers {
		if t.InventoryCount <= t.AlertThreshold {
			alerts = append(alerts, t)
		}
	}

	_, fulfilledToday, _ := h.fulfillRepo.List(ctx, repository.FulfillmentFilters{
		Status: domain.StatusCompleted,
	}, domain.PageRequest{Page: 1, PageSize: 1})

	queuedLogs, _ := h.sendLogRepo.GetRetryable(ctx, time.Now().Add(24*time.Hour))

	d := view.DashboardData{
		PendingCount:      pendingDraft + pendingReady,
		OverdueExceptions: len(openExceptions),
		ThresholdAlerts:   alerts,
		FulfilledToday:    fulfilledToday,
		QueuedMessages:    len(queuedLogs),
	}

	renderPage(c, http.StatusOK, view.Dashboard(pctx, d))
}

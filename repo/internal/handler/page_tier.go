package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
	"github.com/fulfillops/fulfillops/internal/view"
	tierview "github.com/fulfillops/fulfillops/internal/view/tiers"
)

type PageTierHandler struct {
	store    sessions.Store
	tierRepo repository.TierRepository
	auditSvc service.AuditService
}

func NewPageTierHandler(store sessions.Store, tierRepo repository.TierRepository) *PageTierHandler {
	return &PageTierHandler{store: store, tierRepo: tierRepo}
}

func (h *PageTierHandler) WithAudit(auditSvc service.AuditService) *PageTierHandler {
	h.auditSvc = auditSvc
	return h
}

func (h *PageTierHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	q := c.Query("q")
	page := queryInt(c, "page", 1)
	const size = 20

	tiers, _ := h.tierRepo.List(ctx, q, false)

	// simple in-memory pagination on small list
	total := len(tiers)
	start := (page - 1) * size
	end := start + size
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	renderPage(c, http.StatusOK, tierview.List(pageCtx(c, h.store), tierview.ListData{
		Tiers:   tiers[start:end],
		Query:   q,
		Pager:   view.NewPagination(page, size, total, "/tiers", "q="+q),
		IsAdmin: isAdmin(c, h.store),
	}))
}

func (h *PageTierHandler) ShowDetail(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	tier, err := h.tierRepo.GetByID(ctx, id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	renderPage(c, http.StatusOK, tierview.Detail(pageCtx(c, h.store), tierview.DetailData{
		Tier:    *tier,
		IsAdmin: isAdmin(c, h.store),
	}))
}

func (h *PageTierHandler) ShowCreate(c *gin.Context) {
	renderPage(c, http.StatusOK, tierview.Form(pageCtx(c, h.store), tierview.FormData{}))
}

func (h *PageTierHandler) ShowEdit(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	tier, err := h.tierRepo.GetByID(ctx, id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	renderPage(c, http.StatusOK, tierview.Form(pageCtx(c, h.store), tierview.FormData{Tier: tier}))
}

func (h *PageTierHandler) PostCreate(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	t := &domain.RewardTier{
		Name:           c.PostForm("name"),
		InventoryCount: formInt(c, "inventory_count"),
		PurchaseLimit:  normalizePurchaseLimit(formInt(c, "purchase_limit")),
		AlertThreshold: normalizeAlertThreshold(formInt(c, "alert_threshold")),
	}
	if d := c.PostForm("description"); d != "" {
		t.Description = &d
	}
	created, err := h.tierRepo.Create(ctx, t)
	if err != nil {
		renderPage(c, http.StatusUnprocessableEntity, tierview.Form(pageCtx(c, h.store), tierview.FormData{
			Errors: map[string]string{"name": err.Error()},
		}))
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Log(ctx, "reward_tiers", created.ID, "CREATE", nil, created)
	}
	redirectWithFlash(c, h.store, "/tiers", "success", "Tier created successfully.")
}

func (h *PageTierHandler) PostUpdate(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	id, _ := uuid.Parse(c.Param("id"))
	before, _ := h.tierRepo.GetByID(ctx, id)
	t := &domain.RewardTier{
		ID:             id,
		Name:           c.PostForm("name"),
		InventoryCount: formInt(c, "inventory_count"),
		PurchaseLimit:  normalizePurchaseLimit(formInt(c, "purchase_limit")),
		AlertThreshold: normalizeAlertThreshold(formInt(c, "alert_threshold")),
		Version:        formInt(c, "version"),
	}
	if d := c.PostForm("description"); d != "" {
		t.Description = &d
	}
	updated, err := h.tierRepo.Update(ctx, t)
	if err != nil {
		renderPage(c, http.StatusUnprocessableEntity, tierview.Form(pageCtx(c, h.store), tierview.FormData{
			Tier:   t,
			Errors: map[string]string{"name": err.Error()},
		}))
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Log(ctx, "reward_tiers", updated.ID, "UPDATE", before, updated)
	}
	redirectWithFlash(c, h.store, "/tiers/"+id.String(), "success", "Tier updated.")
}

func (h *PageTierHandler) PostDelete(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	id, _ := uuid.Parse(c.Param("id"))
	sess, _ := h.store.Get(c.Request, "fulfillops")
	deletedBy, _ := uuid.Parse(sess.Values["userID"].(string))
	before, _ := h.tierRepo.GetByID(ctx, id)
	_ = h.tierRepo.SoftDelete(ctx, id, deletedBy)
	if h.auditSvc != nil {
		_ = h.auditSvc.Log(ctx, "reward_tiers", id, "DELETE", before, map[string]any{"deleted_by": deletedBy})
	}
	redirectWithFlash(c, h.store, "/tiers", "success", "Tier deleted.")
}

func (h *PageTierHandler) PostRestore(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	id, _ := uuid.Parse(c.Param("id"))
	if err := h.tierRepo.Restore(ctx, id); err != nil {
		redirectWithFlash(c, h.store, "/admin/recovery", "error", "Restore failed: "+err.Error())
		return
	}
	if h.auditSvc != nil {
		if restored, err := h.tierRepo.GetByID(ctx, id); err == nil {
			_ = h.auditSvc.Log(ctx, "reward_tiers", id, "RESTORE", nil, restored)
		}
	}
	redirectWithFlash(c, h.store, "/admin/recovery", "success", "Tier restored.")
}

// helpers
func queryInt(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func formInt(c *gin.Context, key string) int {
	n, _ := strconv.Atoi(c.PostForm(key))
	return n
}

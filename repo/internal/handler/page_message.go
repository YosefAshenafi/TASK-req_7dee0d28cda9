package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
	"github.com/fulfillops/fulfillops/internal/view"
	mview "github.com/fulfillops/fulfillops/internal/view/messages"
	nview "github.com/fulfillops/fulfillops/internal/view/notifications"
)

type PageMessageHandler struct {
	store        sessions.Store
	templateRepo repository.MessageTemplateRepository
	sendLogRepo  repository.SendLogRepository
	notifRepo    repository.NotificationRepository
	auditSvc     service.AuditService
}

func NewPageMessageHandler(
	store sessions.Store,
	templateRepo repository.MessageTemplateRepository,
	sendLogRepo repository.SendLogRepository,
	notifRepo repository.NotificationRepository,
) *PageMessageHandler {
	return &PageMessageHandler{
		store:        store,
		templateRepo: templateRepo,
		sendLogRepo:  sendLogRepo,
		notifRepo:    notifRepo,
	}
}

func (h *PageMessageHandler) WithAudit(auditSvc service.AuditService) *PageMessageHandler {
	h.auditSvc = auditSvc
	return h
}

func (h *PageMessageHandler) ListTemplates(c *gin.Context) {
	ctx := c.Request.Context()
	category := domain.TemplateCategory(c.Query("category"))
	channel := domain.SendLogChannel(c.Query("channel"))
	templates, _ := h.templateRepo.List(ctx, category, channel, false)
	renderPage(c, http.StatusOK, mview.TemplateList(pageCtx(c, h.store), mview.TemplateListData{
		Templates:      templates,
		CategoryFilter: string(category),
		ChannelFilter:  string(channel),
		IsAdmin:        isAdmin(c, h.store),
	}))
}

func (h *PageMessageHandler) ShowCreateTemplate(c *gin.Context) {
	renderPage(c, http.StatusOK, mview.TemplateForm(pageCtx(c, h.store), mview.TemplateFormData{}))
}

func (h *PageMessageHandler) ShowEditTemplate(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	t, err := h.templateRepo.GetByID(ctx, id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	renderPage(c, http.StatusOK, mview.TemplateForm(pageCtx(c, h.store), mview.TemplateFormData{Template: t}))
}

func (h *PageMessageHandler) PostCreateTemplate(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	t := &domain.MessageTemplate{
		Name:         c.PostForm("name"),
		Category:     domain.TemplateCategory(c.PostForm("category")),
		Channel:      domain.SendLogChannel(c.PostForm("channel")),
		BodyTemplate: c.PostForm("body_template"),
	}
	created, err := h.templateRepo.Create(ctx, t)
	if err != nil {
		redirectWithFlash(c, h.store, "/messages/templates/new", "error", err.Error())
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Log(ctx, "message_templates", created.ID, "CREATE", nil, created)
	}
	redirectWithFlash(c, h.store, "/messages", "success", "Template created.")
}

func (h *PageMessageHandler) PostUpdateTemplate(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	id, _ := uuid.Parse(c.Param("id"))
	before, _ := h.templateRepo.GetByID(ctx, id)
	t := &domain.MessageTemplate{
		ID:           id,
		Name:         c.PostForm("name"),
		Category:     domain.TemplateCategory(c.PostForm("category")),
		Channel:      domain.SendLogChannel(c.PostForm("channel")),
		BodyTemplate: c.PostForm("body_template"),
		Version:      formInt(c, "version"),
	}
	updated, err := h.templateRepo.Update(ctx, t)
	if err != nil {
		redirectWithFlash(c, h.store, "/messages/templates/"+id.String()+"/edit", "error", err.Error())
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Log(ctx, "message_templates", id, "UPDATE", before, updated)
	}
	redirectWithFlash(c, h.store, "/messages", "success", "Template updated.")
}

func (h *PageMessageHandler) PostDeleteTemplate(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	id, _ := uuid.Parse(c.Param("id"))
	sess, _ := h.store.Get(c.Request, "fulfillops")
	deletedBy, _ := uuid.Parse(sess.Values["userID"].(string))
	before, _ := h.templateRepo.GetByID(ctx, id)
	if err := h.templateRepo.SoftDelete(ctx, id, deletedBy); err != nil {
		redirectWithFlash(c, h.store, "/messages", "error", "Delete failed: "+err.Error())
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Log(ctx, "message_templates", id, "DELETE", before, map[string]any{"deleted_by": deletedBy})
	}
	redirectWithFlash(c, h.store, "/messages", "success", "Template deleted.")
}

func (h *PageMessageHandler) PostRestoreTemplate(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	id, _ := uuid.Parse(c.Param("id"))
	if err := h.templateRepo.Restore(ctx, id); err != nil {
		redirectWithFlash(c, h.store, "/admin/recovery", "error", "Restore failed.")
		return
	}
	if h.auditSvc != nil {
		if restored, err := h.templateRepo.GetByID(ctx, id); err == nil {
			_ = h.auditSvc.Log(ctx, "message_templates", id, "RESTORE", nil, restored)
		}
	}
	redirectWithFlash(c, h.store, "/admin/recovery", "success", "Template restored.")
}

func (h *PageMessageHandler) ShowSendLogs(c *gin.Context) {
	ctx := c.Request.Context()
	filters := repository.SendLogFilters{}
	if ch := c.Query("channel"); ch != "" {
		filters.Channel = domain.SendLogChannel(ch)
	}
	if s := c.Query("status"); s != "" {
		filters.Status = domain.SendLogStatus(s)
	}
	if r := c.Query("recipient"); r != "" {
		if rid, err := uuid.Parse(r); err == nil {
			filters.RecipientID = &rid
		}
	}
	if df := c.Query("date_from"); df != "" {
		if t, err := time.Parse("2006-01-02", df); err == nil {
			filters.DateFrom = &t
		}
	}
	if dt := c.Query("date_to"); dt != "" {
		if t, err := time.Parse("2006-01-02", dt); err == nil {
			filters.DateTo = &t
		}
	}
	page := queryInt(c, "page", 1)
	const size = 30
	logs, total, _ := h.sendLogRepo.List(ctx, filters, domain.PageRequest{Page: page, PageSize: size})
	renderPage(c, http.StatusOK, mview.SendLogs(pageCtx(c, h.store), mview.SendLogsData{
		Logs:      logs,
		Pager:     view.NewPagination(page, size, total, "/messages/send-logs", ""),
		DateFrom:  c.Query("date_from"),
		DateTo:    c.Query("date_to"),
		Recipient: c.Query("recipient"),
		Channel:   c.Query("channel"),
		Status:    c.Query("status"),
	}))
}

func (h *PageMessageHandler) ShowHandoffQueue(c *gin.Context) {
	ctx := c.Request.Context()
	// Show QUEUED SMS/EMAIL items
	items, _, _ := h.sendLogRepo.List(ctx, repository.SendLogFilters{
		Status: domain.SendQueued,
	}, domain.PageRequest{Page: 1, PageSize: 100})
	// filter for SMS/EMAIL
	var handoffItems []domain.SendLog
	for _, l := range items {
		if l.Channel == domain.ChannelSMS || l.Channel == domain.ChannelEmail {
			handoffItems = append(handoffItems, l)
		}
	}
	renderPage(c, http.StatusOK, mview.HandoffQueue(pageCtx(c, h.store), mview.HandoffData{Items: handoffItems}))
}

func (h *PageMessageHandler) PostMarkPrinted(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	sess, _ := h.store.Get(c.Request, "fulfillops")
	userID, _ := uuid.Parse(sess.Values["userID"].(string))
	if err := h.sendLogRepo.MarkPrinted(ctx, id, userID); err != nil {
		redirectWithFlash(c, h.store, "/messages/handoff", "error", "Mark printed failed: "+err.Error())
		return
	}
	redirectWithFlash(c, h.store, "/messages/handoff", "success", "Marked as printed.")
}

func (h *PageMessageHandler) ListNotifications(c *gin.Context) {
	ctx := c.Request.Context()
	sess, _ := h.store.Get(c.Request, "fulfillops")
	userID, _ := uuid.Parse(sess.Values["userID"].(string))
	onlyUnread := c.Query("unread") == "1"

	page := queryInt(c, "page", 1)
	const size = 20
	var isRead *bool
	if c.Query("unread") == "1" {
		f := false
		isRead = &f
	}
	notifs, total, _ := h.notifRepo.ListByUserID(ctx, userID, isRead, domain.PageRequest{Page: page, PageSize: size})

	renderPage(c, http.StatusOK, nview.List(pageCtx(c, h.store), nview.ListData{
		Notifications: notifs,
		ShowUnread:    onlyUnread,
		Pager:         view.NewPagination(page, size, total, "/notifications", ""),
	}))
}

func (h *PageMessageHandler) PostMarkNotificationRead(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	sess, _ := h.store.Get(c.Request, "fulfillops")
	userID, _ := uuid.Parse(sess.Values["userID"].(string))
	_ = h.notifRepo.MarkRead(ctx, id, userID)
	c.Redirect(302, "/notifications")
}

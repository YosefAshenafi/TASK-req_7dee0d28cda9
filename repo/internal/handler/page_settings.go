package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	sview "github.com/fulfillops/fulfillops/internal/view/settings"
)

type PageSettingsHandler struct {
	store        sessions.Store
	settingRepo  repository.SystemSettingRepository
	blackoutRepo repository.BlackoutDateRepository
}

func NewPageSettingsHandler(
	store sessions.Store,
	settingRepo repository.SystemSettingRepository,
	blackoutRepo repository.BlackoutDateRepository,
) *PageSettingsHandler {
	return &PageSettingsHandler{store: store, settingRepo: settingRepo, blackoutRepo: blackoutRepo}
}

func (h *PageSettingsHandler) ShowBusinessHours(c *gin.Context) {
	ctx := c.Request.Context()
	start := settingStr(ctx, h.settingRepo, "business_hours_start", `"08:00"`)
	end := settingStr(ctx, h.settingRepo, "business_hours_end", `"18:00"`)
	tz := settingStr(ctx, h.settingRepo, "timezone", `"America/New_York"`)
	daysRaw := settingStr(ctx, h.settingRepo, "business_days", "[1,2,3,4,5]")

	// Strip surrounding quotes for time values
	start = unquoteJSON(start)
	end = unquoteJSON(end)
	tz = unquoteJSON(tz)

	var days []int
	_ = json.Unmarshal([]byte(daysRaw), &days)

	renderPage(c, http.StatusOK, sview.BusinessHours(pageCtx(c, h.store), sview.BusinessHoursData{
		Start: start, End: end, BusinessDays: days, Timezone: tz,
	}))
}

func (h *PageSettingsHandler) PostBusinessHours(c *gin.Context) {
	ctx := c.Request.Context()
	sess, _ := h.store.Get(c.Request, "fulfillops")
	userID, _ := uuid.Parse(sess.Values["userID"].(string))

	dayValues := c.PostFormArray("business_days")
	days := make([]int, 0, len(dayValues))
	for _, d := range dayValues {
		if n, err := strconv.Atoi(d); err == nil {
			days = append(days, n)
		}
	}
	daysJSON, _ := json.Marshal(days)

	_ = h.settingRepo.Set(ctx, "business_hours_start", []byte(`"`+c.PostForm("start")+`"`), &userID)
	_ = h.settingRepo.Set(ctx, "business_hours_end", []byte(`"`+c.PostForm("end")+`"`), &userID)
	_ = h.settingRepo.Set(ctx, "timezone", []byte(`"`+c.PostForm("timezone")+`"`), &userID)
	_ = h.settingRepo.Set(ctx, "business_days", daysJSON, &userID)

	redirectWithFlash(c, h.store, "/settings", "success", "Business hours saved.")
}

func (h *PageSettingsHandler) ShowBlackoutDates(c *gin.Context) {
	ctx := c.Request.Context()
	dates, _ := h.blackoutRepo.List(ctx)
	renderPage(c, http.StatusOK, sview.BlackoutDates(pageCtx(c, h.store), sview.BlackoutData{Dates: dates}))
}

func (h *PageSettingsHandler) PostCreateBlackoutDate(c *gin.Context) {
	ctx := c.Request.Context()
	sess, _ := h.store.Get(c.Request, "fulfillops")
	userID, _ := uuid.Parse(sess.Values["userID"].(string))

	t, err := time.Parse("2006-01-02", c.PostForm("date"))
	if err != nil {
		redirectWithFlash(c, h.store, "/settings/blackout-dates", "error", "Invalid date.")
		return
	}
	bd := &domain.BlackoutDate{Date: t, CreatedBy: &userID}
	if desc := c.PostForm("description"); desc != "" {
		bd.Description = &desc
	}
	if _, err := h.blackoutRepo.Create(ctx, bd); err != nil {
		redirectWithFlash(c, h.store, "/settings/blackout-dates", "error", err.Error())
		return
	}
	redirectWithFlash(c, h.store, "/settings/blackout-dates", "success", "Blackout date added.")
}

func (h *PageSettingsHandler) PostDeleteBlackoutDate(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	_ = h.blackoutRepo.Delete(ctx, id)
	redirectWithFlash(c, h.store, "/settings/blackout-dates", "success", "Blackout date removed.")
}

func settingStr(ctx context.Context, repo repository.SystemSettingRepository, key, def string) string {
	setting, err := repo.Get(ctx, key)
	if err != nil || setting == nil {
		return def
	}
	if setting.Value == nil {
		return def
	}
	b, err := json.Marshal(setting.Value)
	if err != nil {
		return def
	}
	return string(b)
}

func unquoteJSON(s string) string {
	var v string
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

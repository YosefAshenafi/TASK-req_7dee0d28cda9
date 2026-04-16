package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// SLAService calculates fulfillment deadlines and checks overdue status.
type SLAService interface {
	// CalculateDeadline returns the SLA deadline for the given fulfillment type, starting from readyAt.
	// Physical: readyAt + 48 hours (wall clock).
	// Voucher: readyAt + 4 business hours, skipping weekends and blackout dates.
	CalculateDeadline(ctx context.Context, fType domain.FulfillmentType, readyAt time.Time) (time.Time, error)
	// IsOverdue returns true if the deadline has passed.
	IsOverdue(deadline time.Time) bool
}

type slaService struct {
	settingRepo     repository.SystemSettingRepository
	blackoutRepo    repository.BlackoutDateRepository
}

// NewSLAService creates an SLAService backed by system settings and blackout dates.
func NewSLAService(settingRepo repository.SystemSettingRepository, blackoutRepo repository.BlackoutDateRepository) SLAService {
	return &slaService{settingRepo: settingRepo, blackoutRepo: blackoutRepo}
}

type businessConfig struct {
	HoursStart  string // "08:00"
	HoursEnd    string // "18:00"
	BusinessDays []int  // 1=Mon … 7=Sun (ISO)
	Timezone    string
}

func (s *slaService) loadBusinessConfig(ctx context.Context) (*businessConfig, error) {
	cfg := &businessConfig{
		HoursStart:   "08:00",
		HoursEnd:     "18:00",
		BusinessDays: []int{1, 2, 3, 4, 5},
		Timezone:     "America/New_York",
	}

	if setting, err := s.settingRepo.Get(ctx, "business_hours_start"); err == nil {
		var v string
		if json.Unmarshal(setting.Value, &v) == nil {
			cfg.HoursStart = v
		}
	}
	if setting, err := s.settingRepo.Get(ctx, "business_hours_end"); err == nil {
		var v string
		if json.Unmarshal(setting.Value, &v) == nil {
			cfg.HoursEnd = v
		}
	}
	if setting, err := s.settingRepo.Get(ctx, "business_days"); err == nil {
		var days []int
		if json.Unmarshal(setting.Value, &days) == nil {
			cfg.BusinessDays = days
		}
	}
	if setting, err := s.settingRepo.Get(ctx, "timezone"); err == nil {
		var tz string
		if json.Unmarshal(setting.Value, &tz) == nil {
			cfg.Timezone = tz
		}
	}
	return cfg, nil
}

func (s *slaService) CalculateDeadline(ctx context.Context, fType domain.FulfillmentType, readyAt time.Time) (time.Time, error) {
	if fType == domain.TypePhysical {
		return readyAt.Add(48 * time.Hour), nil
	}

	// Voucher: 4 business hours
	cfg, err := s.loadBusinessConfig(ctx)
	if err != nil {
		return time.Time{}, err
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.UTC
	}

	// Parse business hour boundaries for a given day
	parseHHMM := func(hhmm string, base time.Time) time.Time {
		var h, m int
		if len(hhmm) == 5 {
			h = int(hhmm[0]-'0')*10 + int(hhmm[1]-'0')
			m = int(hhmm[3]-'0')*10 + int(hhmm[4]-'0')
		}
		return time.Date(base.Year(), base.Month(), base.Day(), h, m, 0, 0, base.Location())
	}

	// Load blackout dates for a 30-day window
	blackouts, _ := s.blackoutRepo.GetBetween(ctx, readyAt, readyAt.AddDate(0, 0, 30))
	blackoutSet := make(map[string]bool)
	for _, b := range blackouts {
		blackoutSet[b.Date.Format("2006-01-02")] = true
	}

	isBusinessDay := func(t time.Time) bool {
		t = t.In(loc)
		// ISO weekday: Mon=1 … Sun=7
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		for _, d := range cfg.BusinessDays {
			if d == wd {
				break
			}
		}
		found := false
		for _, d := range cfg.BusinessDays {
			if d == wd {
				found = true
				break
			}
		}
		if !found {
			return false
		}
		return !blackoutSet[t.Format("2006-01-02")]
	}

	current := readyAt.In(loc)
	remaining := 4 * time.Hour

	for remaining > 0 {
		if !isBusinessDay(current) {
			// Skip to next day at business start
			current = parseHHMM(cfg.HoursStart, current.AddDate(0, 0, 1))
			continue
		}
		dayEnd := parseHHMM(cfg.HoursEnd, current)
		dayStart := parseHHMM(cfg.HoursStart, current)

		// If current time is before business start, jump to start
		if current.Before(dayStart) {
			current = dayStart
		}
		// If current time is at or after end of business day, go to next day
		if !current.Before(dayEnd) {
			current = parseHHMM(cfg.HoursStart, current.AddDate(0, 0, 1))
			continue
		}

		availableToday := dayEnd.Sub(current)
		if remaining <= availableToday {
			current = current.Add(remaining)
			remaining = 0
		} else {
			remaining -= availableToday
			current = parseHHMM(cfg.HoursStart, current.AddDate(0, 0, 1))
		}
	}

	return current.UTC(), nil
}

func (s *slaService) IsOverdue(deadline time.Time) bool {
	return time.Now().UTC().After(deadline)
}

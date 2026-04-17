package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
	adview "github.com/fulfillops/fulfillops/internal/view/admin"
)

type PageAdminHandler struct {
	store           sessions.Store
	pool            *pgxpool.Pool
	jobRunRepo      repository.JobRunRepository
	tierRepo        repository.TierRepository
	customerRepo    repository.CustomerRepository
	fulfillRepo     repository.FulfillmentRepository
	templateRepo    repository.MessageTemplateRepository
	userSvc         service.UserService
	userRepo        repository.UserRepository
	backupSvc       service.BackupService
	scheduler       JobScheduler
	encKeyPath      string
	exportDir       string
	backupDir       string
	auditSvc        service.AuditService
	jobScheduleRepo repository.JobScheduleRepository
	drDrillRepo     repository.DRDrillRepository
}

func NewPageAdminHandler(
	store sessions.Store,
	pool *pgxpool.Pool,
	jobRunRepo repository.JobRunRepository,
	tierRepo repository.TierRepository,
	customerRepo repository.CustomerRepository,
	fulfillRepo repository.FulfillmentRepository,
	templateRepo repository.MessageTemplateRepository,
	userSvc service.UserService,
	userRepo repository.UserRepository,
) *PageAdminHandler {
	return &PageAdminHandler{
		store: store, pool: pool, jobRunRepo: jobRunRepo,
		tierRepo: tierRepo, customerRepo: customerRepo,
		fulfillRepo: fulfillRepo, templateRepo: templateRepo,
		userSvc: userSvc, userRepo: userRepo,
	}
}

// WithHealthConfig attaches config paths needed for real health checks.
func (h *PageAdminHandler) WithHealthConfig(encKeyPath, exportDir, backupDir string, scheduler JobScheduler) *PageAdminHandler {
	h.encKeyPath = encKeyPath
	h.exportDir = exportDir
	h.backupDir = backupDir
	h.scheduler = scheduler
	return h
}

// WithAudit attaches an AuditService for admin operation logging.
func (h *PageAdminHandler) WithAudit(auditSvc service.AuditService) *PageAdminHandler {
	h.auditSvc = auditSvc
	return h
}

// WithScheduleRepos attaches repositories for job schedule and DR drill management.
func (h *PageAdminHandler) WithScheduleRepos(jobScheduleRepo repository.JobScheduleRepository, drDrillRepo repository.DRDrillRepository) *PageAdminHandler {
	h.jobScheduleRepo = jobScheduleRepo
	h.drDrillRepo = drDrillRepo
	return h
}

// WithBackupService attaches a backup service for real backup/restore operations.
func (h *PageAdminHandler) WithBackupService(svc service.BackupService) *PageAdminHandler {
	h.backupSvc = svc
	return h
}

func (h *PageAdminHandler) ShowHealth(c *gin.Context) {
	ctx := c.Request.Context()

	dbOK := h.pool.Ping(ctx) == nil
	dbDetail := "PostgreSQL 16 — connected"
	if !dbOK {
		dbDetail = "PostgreSQL 16 — unreachable"
	}

	encOK := true
	encDetail := h.encKeyPath
	if encDetail == "" {
		encDetail = "(path not configured)"
	}
	if _, err := os.Stat(h.encKeyPath); err != nil {
		encOK = false
		encDetail = h.encKeyPath + " — " + err.Error()
	}

	exportOK := true
	exportDetail := h.exportDir
	if exportDetail == "" {
		exportDetail = "(path not configured)"
	}
	if _, err := os.Stat(h.exportDir); err != nil {
		exportOK = false
		exportDetail = h.exportDir + " — " + err.Error()
	}

	backupOK := true
	backupDetail := h.backupDir
	if backupDetail == "" {
		backupDetail = "(path not configured)"
	}
	if _, err := os.Stat(h.backupDir); err != nil {
		backupOK = false
		backupDetail = h.backupDir + " — " + err.Error()
	}

	schedOK := h.scheduler != nil
	schedDetail := "Running"
	if !schedOK {
		schedDetail = "Not initialised"
	}

	checks := []adview.HealthCheck{
		{Name: "Database", OK: dbOK, Detail: dbDetail},
		{Name: "Encryption Key", OK: encOK, Detail: encDetail},
		{Name: "Export Directory", OK: exportOK, Detail: exportDetail},
		{Name: "Backup Directory", OK: backupOK, Detail: backupDetail},
		{Name: "Scheduler", OK: schedOK, Detail: schedDetail},
	}
	runs, _, _ := h.jobRunRepo.List(ctx, repository.JobRunFilters{}, domain.PageRequest{Page: 1, PageSize: 20})
	renderPage(c, http.StatusOK, adview.Health(pageCtx(c, h.store), adview.HealthData{
		Checks: checks, JobRuns: runs,
	}))
}

func (h *PageAdminHandler) ShowUsers(c *gin.Context) {
	ctx := c.Request.Context()
	users, _ := h.userRepo.List(ctx, domain.UserRole(""), nil)
	renderPage(c, http.StatusOK, adview.Users(pageCtx(c, h.store), adview.UsersData{Users: users}))
}

func (h *PageAdminHandler) ShowCreateUser(c *gin.Context) {
	renderPage(c, http.StatusOK, adview.UserForm(pageCtx(c, h.store), adview.UserFormData{}))
}

func (h *PageAdminHandler) ShowEditUser(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	u, err := h.userRepo.GetByID(ctx, id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	renderPage(c, http.StatusOK, adview.UserForm(pageCtx(c, h.store), adview.UserFormData{User: u}))
}

func (h *PageAdminHandler) PostCreateUser(c *gin.Context) {
	ctx := c.Request.Context()
	if _, err := h.userSvc.CreateUser(ctx,
		c.PostForm("username"),
		c.PostForm("email"),
		c.PostForm("password"),
		domain.UserRole(c.PostForm("role")),
	); err != nil {
		redirectWithFlash(c, h.store, "/admin/users/new", "error", err.Error())
		return
	}
	redirectWithFlash(c, h.store, "/admin/users", "success", "User created.")
}

func (h *PageAdminHandler) PostUpdateUser(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	if _, err := h.userSvc.UpdateUser(ctx,
		id,
		c.PostForm("email"),
		domain.UserRole(c.PostForm("role")),
	); err != nil {
		redirectWithFlash(c, h.store, "/admin/users/"+id.String()+"/edit", "error", err.Error())
		return
	}
	redirectWithFlash(c, h.store, "/admin/users", "success", "User updated.")
}

func (h *PageAdminHandler) PostDeactivateUser(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	if err := h.userSvc.DeactivateUser(ctx, id); err != nil {
		redirectWithFlash(c, h.store, "/admin/users", "error", err.Error())
		return
	}
	redirectWithFlash(c, h.store, "/admin/users", "success", "User deactivated.")
}

func (h *PageAdminHandler) ShowRecovery(c *gin.Context) {
	ctx := c.Request.Context()

	allTiers, _ := h.tierRepo.List(ctx, "", true)
	allCustomers, _, _ := h.customerRepo.List(ctx, "", domain.PageRequest{Page: 1, PageSize: 500}, true)
	allFulfillments, _, _ := h.fulfillRepo.List(ctx, repository.FulfillmentFilters{IncludeDeleted: true}, domain.PageRequest{Page: 1, PageSize: 200})
	allTemplates, _ := h.templateRepo.List(ctx, domain.TemplateCategory(""), domain.SendLogChannel(""), true)

	var deletedTiers []domain.RewardTier
	for _, t := range allTiers {
		if t.DeletedAt != nil {
			deletedTiers = append(deletedTiers, t)
		}
	}
	var deletedCustomers []domain.Customer
	for _, cu := range allCustomers {
		if cu.DeletedAt != nil {
			deletedCustomers = append(deletedCustomers, cu)
		}
	}
	var deletedFulfillments []domain.Fulfillment
	for _, f := range allFulfillments {
		if f.DeletedAt != nil {
			deletedFulfillments = append(deletedFulfillments, f)
		}
	}
	var deletedTemplates []domain.MessageTemplate
	for _, t := range allTemplates {
		if t.DeletedAt != nil {
			deletedTemplates = append(deletedTemplates, t)
		}
	}

	renderPage(c, http.StatusOK, adview.Recovery(pageCtx(c, h.store), adview.RecoveryData{
		Tiers:        deletedTiers,
		Customers:    deletedCustomers,
		Fulfillments: deletedFulfillments,
		Templates:    deletedTemplates,
	}))
}

func (h *PageAdminHandler) ShowBackups(c *gin.Context) {
	var entries []adview.BackupEntry
	if h.backupSvc != nil {
		backups, _ := h.backupSvc.ListBackups(c.Request.Context())
		for _, b := range backups {
			size := fmt.Sprintf("%d bytes", b.FileSize)
			if b.FileSize > 1024*1024 {
				size = fmt.Sprintf("%.1f MB", float64(b.FileSize)/1024/1024)
			} else if b.FileSize > 1024 {
				size = fmt.Sprintf("%.1f KB", float64(b.FileSize)/1024)
			}
			entries = append(entries, adview.BackupEntry{
				ID:        b.ID,
				CreatedAt: b.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
				FileSize:  size,
				Status:    b.Status,
			})
		}
	}
	renderPage(c, http.StatusOK, adview.Backups(pageCtx(c, h.store), adview.BackupsData{Entries: entries}))
}

func (h *PageAdminHandler) PostRunBackup(c *gin.Context) {
	actorRaw, _ := c.Get("userID")
	actorID, _ := actorRaw.(uuid.UUID)

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "backups", uuid.Nil, "BACKUP_TRIGGER",
			nil, map[string]any{"triggered_by": actorID})
	}

	if h.backupSvc != nil {
		go func() {
			if _, err := h.backupSvc.RunBackup(context.Background()); err != nil {
				log.Printf("backup: run failed: %v", err)
			}
		}()
	}
	redirectWithFlash(c, h.store, "/admin/backups", "queued", "Backup started in background.")
}

// ShowJobSchedules renders the admin job-schedule management page.
func (h *PageAdminHandler) ShowJobSchedules(c *gin.Context) {
	ctx := c.Request.Context()
	var schedules []interface{}
	if h.jobScheduleRepo != nil {
		if items, err := h.jobScheduleRepo.List(ctx); err == nil {
			for _, item := range items {
				schedules = append(schedules, item)
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": schedules})
}

// PostUpdateJobSchedule updates a job schedule's cadence.
func (h *PageAdminHandler) PostUpdateJobSchedule(c *gin.Context) {
	if h.jobScheduleRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule repo not available"})
		return
	}
	ctx := c.Request.Context()
	jobKey := c.Param("key")

	schedule, err := h.jobScheduleRepo.GetByKey(ctx, jobKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job schedule not found"})
		return
	}

	type updateReq struct {
		IntervalSeconds *int  `json:"interval_seconds"`
		DailyHour       *int  `json:"daily_hour"`
		DailyMinute     *int  `json:"daily_minute"`
		Enabled         bool  `json:"enabled"`
		Version         int   `json:"version"`
	}
	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	actorRaw, _ := c.Get("userID")
	actorID, _ := actorRaw.(uuid.UUID)

	before := *schedule
	schedule.IntervalSeconds = req.IntervalSeconds
	schedule.DailyHour = req.DailyHour
	schedule.DailyMinute = req.DailyMinute
	schedule.Enabled = req.Enabled
	schedule.Version = req.Version
	schedule.UpdatedBy = &actorID

	updated, err := h.jobScheduleRepo.Update(ctx, schedule)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(ctx, "job_schedules", updated.ID, "UPDATE", before, updated)
	}

	c.JSON(http.StatusOK, updated)
}

// ListDRDrills returns all DR drill records.
func (h *PageAdminHandler) ListDRDrills(c *gin.Context) {
	if h.drDrillRepo == nil {
		c.JSON(http.StatusOK, gin.H{"items": []interface{}{}})
		return
	}
	page := 1
	pageSize := 50
	items, total, err := h.drDrillRepo.List(c.Request.Context(), domain.PageRequest{Page: page, PageSize: pageSize})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, domain.PageResponse[domain.DRDrill]{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	})
}

// PostCreateDRDrill records a new DR drill.
func (h *PageAdminHandler) PostCreateDRDrill(c *gin.Context) {
	if h.drDrillRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dr drill repo not available"})
		return
	}
	type createReq struct {
		ScheduledFor string  `json:"scheduled_for" binding:"required"`
		Notes        *string `json:"notes"`
	}
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sf, err := parseBlackoutDate(req.ScheduledFor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scheduled_for; use RFC3339 or YYYY-MM-DD"})
		return
	}

	actorRaw, _ := c.Get("userID")
	actorID, _ := actorRaw.(uuid.UUID)

	pending := "PENDING"
	drill := &domain.DRDrill{
		ScheduledFor: sf,
		Notes:        req.Notes,
		ExecutedBy:   &actorID,
		Outcome:      &pending,
	}
	created, err := h.drDrillRepo.Create(c.Request.Context(), drill)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "dr_drills", created.ID, "CREATE", nil, created)
	}

	c.JSON(http.StatusCreated, created)
}

// PostUpdateDRDrill records the outcome of a DR drill.
func (h *PageAdminHandler) PostUpdateDRDrill(c *gin.Context) {
	if h.drDrillRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dr drill repo not available"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid drill ID"})
		return
	}

	type updateReq struct {
		Outcome      string  `json:"outcome" binding:"required"`
		Notes        *string `json:"notes"`
		ArtifactPath *string `json:"artifact_path"`
	}
	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	drill, err := h.drDrillRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "drill not found"})
		return
	}

	actorRaw, _ := c.Get("userID")
	actorID, _ := actorRaw.(uuid.UUID)
	now := time.Now().UTC()

	before := *drill
	drill.Outcome = &req.Outcome
	drill.Notes = req.Notes
	drill.ArtifactPath = req.ArtifactPath
	drill.ExecutedBy = &actorID
	drill.ExecutedAt = &now

	updated, err := h.drDrillRepo.Update(c.Request.Context(), drill)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.auditSvc != nil {
		_ = h.auditSvc.Log(c.Request.Context(), "dr_drills", id, "UPDATE", before, updated)
	}

	c.JSON(http.StatusOK, updated)
}

func (h *PageAdminHandler) PostRestoreBackup(c *gin.Context) {
	backupID := c.Param("id")
	if backupID == "" {
		backupID = c.PostForm("backup_id")
	}
	// Fail-safe: integrity verification is on by default and only disabled
	// when the operator explicitly opts out with verify_integrity=off.
	verifyIntegrity := c.PostForm("verify_integrity") != "off"

	if h.backupSvc != nil && backupID != "" {
		if err := h.backupSvc.RestoreFromBackup(c.Request.Context(), backupID, verifyIntegrity); err != nil {
			log.Printf("backup: restore failed: %v", err)
			redirectWithFlash(c, h.store, "/admin/backups", "error", "Restore failed: "+err.Error())
			return
		}
	}
	redirectWithFlash(c, h.store, "/admin/backups", "success", "Database restored successfully.")
}

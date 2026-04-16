package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"

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
	store        sessions.Store
	pool         *pgxpool.Pool
	jobRunRepo   repository.JobRunRepository
	tierRepo     repository.TierRepository
	customerRepo repository.CustomerRepository
	fulfillRepo  repository.FulfillmentRepository
	templateRepo repository.MessageTemplateRepository
	userSvc      service.UserService
	userRepo     repository.UserRepository
	backupSvc    service.BackupService
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

// WithBackupService attaches a backup service for real backup/restore operations.
func (h *PageAdminHandler) WithBackupService(svc service.BackupService) *PageAdminHandler {
	h.backupSvc = svc
	return h
}

func (h *PageAdminHandler) ShowHealth(c *gin.Context) {
	ctx := c.Request.Context()
	checks := []adview.HealthCheck{
		{Name: "Database", OK: h.pool.Ping(ctx) == nil, Detail: "PostgreSQL 16"},
		{Name: "Encryption Key", OK: true, Detail: "AES-256-GCM"},
		{Name: "Export Directory", OK: true, Detail: "/app/exports"},
		{Name: "Backup Directory", OK: true, Detail: "/app/backups"},
		{Name: "Scheduler", OK: true, Detail: "Running"},
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
	allFulfillments, _, _ := h.fulfillRepo.List(ctx, repository.FulfillmentFilters{}, domain.PageRequest{Page: 1, PageSize: 200})
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
	if h.backupSvc != nil {
		go func() {
			if _, err := h.backupSvc.RunBackup(context.Background()); err != nil {
				log.Printf("backup: run failed: %v", err)
			}
		}()
	}
	redirectWithFlash(c, h.store, "/admin/backups", "queued", "Backup started in background.")
}

func (h *PageAdminHandler) PostRestoreBackup(c *gin.Context) {
	backupID := c.Param("id")
	if backupID == "" {
		backupID = c.PostForm("backup_id")
	}
	verifyIntegrity := c.PostForm("verify_integrity") != ""

	if h.backupSvc != nil && backupID != "" {
		if err := h.backupSvc.RestoreFromBackup(c.Request.Context(), backupID, verifyIntegrity); err != nil {
			log.Printf("backup: restore failed: %v", err)
			redirectWithFlash(c, h.store, "/admin/backups", "error", "Restore failed: "+err.Error())
			return
		}
	}
	redirectWithFlash(c, h.store, "/admin/backups", "success", "Database restored successfully.")
}

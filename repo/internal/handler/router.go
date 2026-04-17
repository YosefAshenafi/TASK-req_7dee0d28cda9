package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/middleware"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

// JobScheduler is the minimal interface the router needs to trigger jobs.
type JobScheduler interface {
	RunOnce(ctx context.Context, name string) error
}

type Deps struct {
	Pool         *pgxpool.Pool
	Store        sessions.Store
	UserSvc      service.UserService
	FulfillSvc   service.FulfillmentService
	ExceptionSvc service.ExceptionService
	MessagingSvc service.MessagingService
	AuditSvc     service.AuditService
	EncSvc       service.EncryptionService
	ExportSvc    service.ExportService
	BackupSvc    service.BackupService
	Scheduler    JobScheduler
	TierRepo     repository.TierRepository
	CustomerRepo repository.CustomerRepository
	FulfillRepo  repository.FulfillmentRepository
	TimelineRepo repository.TimelineRepository
	ShippingRepo repository.ShippingAddressRepository
	ExceptionRepo repository.ExceptionRepository
	ExEventRepo  repository.ExceptionEventRepository
	TemplateRepo repository.MessageTemplateRepository
	SendLogRepo  repository.SendLogRepository
	NotifRepo    repository.NotificationRepository
	ReportRepo   repository.ReportExportRepository
	AuditRepo    repository.AuditRepository
	SettingRepo  repository.SystemSettingRepository
	BlackoutRepo repository.BlackoutDateRepository
	JobRunRepo   repository.JobRunRepository
	UserRepo     repository.UserRepository

	// Config paths for real health checks
	EncKeyPath string
	ExportDir  string
	BackupDir  string
}

func RegisterRoutes(r *gin.Engine, d Deps) {
	// Global middleware
	r.Use(middleware.RequestID(), middleware.ClientIP(), middleware.Logger())

	// Static files
	r.Static("/static", "./static")

	// ── Page handlers (server-rendered UI) ────────────────────────────────────
	pageAuth := NewPageAuthHandler(d.UserSvc, d.Store)
	pageTier := NewPageTierHandler(d.Store, d.TierRepo)
	pageCustomer := NewPageCustomerHandler(d.Store, d.CustomerRepo, d.FulfillRepo, d.EncSvc)
	pageFulfillment := NewPageFulfillmentHandler(
		d.Store, d.FulfillSvc, d.FulfillRepo, d.TierRepo, d.CustomerRepo,
		d.TimelineRepo, d.ShippingRepo, d.ExceptionRepo, d.EncSvc,
	)
	pageException := NewPageExceptionHandler(d.Store, d.ExceptionRepo, d.ExEventRepo).WithExceptionService(d.ExceptionSvc)
	pageMessage := NewPageMessageHandler(d.Store, d.TemplateRepo, d.SendLogRepo, d.NotifRepo)
	pageReport := NewPageReportHandler(d.Store, d.ReportRepo, d.ExportSvc)
	pageAudit := NewPageAuditHandler(d.Store, d.AuditRepo)
	pageSettings := NewPageSettingsHandler(d.Store, d.SettingRepo, d.BlackoutRepo)
	pageAdmin := NewPageAdminHandler(
		d.Store, d.Pool, d.JobRunRepo,
		d.TierRepo, d.CustomerRepo, d.FulfillRepo, d.TemplateRepo,
		d.UserSvc, d.UserRepo,
	).WithBackupService(d.BackupSvc).
		WithHealthConfig(d.EncKeyPath, d.ExportDir, d.BackupDir, d.Scheduler)

	// Public page routes
	r.GET("/auth/login", pageAuth.ShowLogin)
	r.POST("/auth/login", pageAuth.PostLogin)
	r.POST("/auth/logout", pageAuth.PostLogout)

	// Authenticated page routes
	authedPage := r.Group("", middleware.PageSessionAuth(d.Store))
	adminOnlyPage := r.Group("", middleware.PageSessionAuth(d.Store), middleware.PageRequireRole(d.Store, domain.RoleAdministrator))
	adminOrSpecPage := r.Group("", middleware.PageSessionAuth(d.Store), middleware.PageRequireRole(d.Store, domain.RoleAdministrator, domain.RoleFulfillmentSpecialist))
	adminOrAuditPage := r.Group("", middleware.PageSessionAuth(d.Store), middleware.PageRequireRole(d.Store, domain.RoleAdministrator, domain.RoleAuditor))

	pageDashboard := NewPageDashboardHandler(d.Store, d.FulfillRepo, d.TierRepo, d.SendLogRepo, d.ExceptionRepo)

	// Dashboard
	authedPage.GET("/", pageDashboard.Show)

	// Tiers
	authedPage.GET("/tiers", pageTier.List)
	authedPage.GET("/tiers/:id", pageTier.ShowDetail)
	adminOnlyPage.GET("/tiers/new", pageTier.ShowCreate)
	adminOnlyPage.POST("/tiers", pageTier.PostCreate)
	adminOnlyPage.GET("/tiers/:id/edit", pageTier.ShowEdit)
	adminOnlyPage.POST("/tiers/:id", pageTier.PostUpdate)
	adminOnlyPage.POST("/tiers/:id/delete", pageTier.PostDelete)
	adminOnlyPage.POST("/tiers/:id/restore", pageTier.PostRestore)

	// Customers
	authedPage.GET("/customers", pageCustomer.List)
	authedPage.GET("/customers/:id", pageCustomer.ShowDetail)
	adminOrSpecPage.GET("/customers/new", pageCustomer.ShowCreate)
	adminOrSpecPage.POST("/customers", pageCustomer.PostCreate)
	adminOrSpecPage.GET("/customers/:id/edit", pageCustomer.ShowEdit)
	adminOrSpecPage.POST("/customers/:id", pageCustomer.PostUpdate)
	adminOnlyPage.POST("/customers/:id/delete", pageCustomer.PostDelete)

	// Fulfillments
	authedPage.GET("/fulfillments", pageFulfillment.List)
	authedPage.GET("/fulfillments/:id", pageFulfillment.ShowDetail)
	adminOrSpecPage.GET("/fulfillments/new", pageFulfillment.ShowCreate)
	adminOrSpecPage.POST("/fulfillments", pageFulfillment.PostCreate)
	adminOrSpecPage.POST("/fulfillments/:id/transition", pageFulfillment.PostTransition)
	adminOnlyPage.POST("/fulfillments/:id/delete", pageFulfillment.PostDelete)
	adminOnlyPage.POST("/fulfillments/:id/restore", pageFulfillment.PostRestore)

	// Exceptions
	adminOrSpecPage.GET("/exceptions", pageException.List)
	adminOrSpecPage.GET("/exceptions/:id", pageException.ShowDetail)
	adminOrSpecPage.POST("/exceptions/:id/status", pageException.PostUpdateStatus)
	adminOrSpecPage.POST("/exceptions/:id/events", pageException.PostAddEvent)

	// Messages
	adminOrSpecPage.GET("/messages", pageMessage.ListTemplates)
	adminOnlyPage.GET("/messages/templates/new", pageMessage.ShowCreateTemplate)
	adminOnlyPage.POST("/messages/templates", pageMessage.PostCreateTemplate)
	adminOnlyPage.GET("/messages/templates/:id/edit", pageMessage.ShowEditTemplate)
	adminOnlyPage.POST("/messages/templates/:id", pageMessage.PostUpdateTemplate)
	adminOnlyPage.POST("/messages/templates/:id/delete", pageMessage.PostDeleteTemplate)
	adminOnlyPage.POST("/messages/templates/:id/restore", pageMessage.PostRestoreTemplate)
	adminOrSpecPage.GET("/messages/send-logs", pageMessage.ShowSendLogs)
	adminOrSpecPage.GET("/messages/handoff", pageMessage.ShowHandoffQueue)
	adminOrSpecPage.POST("/messages/send-logs/:id/printed", pageMessage.PostMarkPrinted)

	// Notifications
	authedPage.GET("/notifications", pageMessage.ListNotifications)
	authedPage.POST("/notifications/:id/read", pageMessage.PostMarkNotificationRead)

	// Reports
	adminOrAuditPage.GET("/reports", pageReport.ShowWorkspace)
	adminOrAuditPage.POST("/reports/exports", pageReport.PostGenerateExport)
	adminOrAuditPage.GET("/reports/history", pageReport.ShowHistory)
	adminOrAuditPage.POST("/reports/exports/:id/verify", pageReport.PostVerifyExport)
	adminOrAuditPage.GET("/reports/exports/:id/download", pageReport.DownloadExport)

	// Audit
	adminOrAuditPage.GET("/audit", pageAudit.List)

	// Settings
	adminOnlyPage.GET("/settings", pageSettings.ShowBusinessHours)
	adminOnlyPage.POST("/settings/business-hours", pageSettings.PostBusinessHours)
	adminOnlyPage.GET("/settings/blackout-dates", pageSettings.ShowBlackoutDates)
	adminOnlyPage.POST("/settings/blackout-dates", pageSettings.PostCreateBlackoutDate)
	adminOnlyPage.POST("/settings/blackout-dates/:id/delete", pageSettings.PostDeleteBlackoutDate)

	// Admin
	adminOnlyPage.GET("/admin/health", pageAdmin.ShowHealth)
	adminOnlyPage.GET("/admin/users", pageAdmin.ShowUsers)
	adminOnlyPage.GET("/admin/users/new", pageAdmin.ShowCreateUser)
	adminOnlyPage.POST("/admin/users", pageAdmin.PostCreateUser)
	adminOnlyPage.GET("/admin/users/:id/edit", pageAdmin.ShowEditUser)
	adminOnlyPage.POST("/admin/users/:id", pageAdmin.PostUpdateUser)
	adminOnlyPage.POST("/admin/users/:id/deactivate", pageAdmin.PostDeactivateUser)
	adminOnlyPage.GET("/admin/recovery", pageAdmin.ShowRecovery)
	adminOnlyPage.GET("/admin/backups", pageAdmin.ShowBackups)
	adminOnlyPage.POST("/admin/backups/run", pageAdmin.PostRunBackup)
	adminOnlyPage.POST("/admin/backups/:id/restore", pageAdmin.PostRestoreBackup)
	// Admin job triggers (page: manual)
	if d.Scheduler != nil {
		adminOnlyPage.POST("/admin/jobs/:name/run", func(c *gin.Context) {
			name := c.Param("name")
			_ = d.Scheduler.RunOnce(context.Background(), name)
			c.Redirect(http.StatusSeeOther, "/admin/health")
		})
	}

	api := r.Group("/api/v1")

	// Public auth
	auth := NewAuthHandler(d.UserSvc, d.Store)
	api.POST("/auth/login", auth.Login)
	api.POST("/auth/logout", auth.Logout)

	// Authenticated routes
	authed := api.Group("", middleware.SessionAuth(d.Store))
	authed.GET("/auth/me", auth.Me)

	adminOnly := authed.Group("", middleware.RequireRole(domain.RoleAdministrator))
	adminOrSpecialist := authed.Group("", middleware.RequireRole(domain.RoleAdministrator, domain.RoleFulfillmentSpecialist))
	adminOrAuditor := authed.Group("", middleware.RequireRole(domain.RoleAdministrator, domain.RoleAuditor))

	// Tiers
	tiers := NewTierHandler(d.TierRepo, d.AuditSvc)
	authed.GET("/tiers", tiers.List)
	adminOnly.POST("/tiers", tiers.Create)
	authed.GET("/tiers/:id", tiers.Get)
	adminOnly.PUT("/tiers/:id", tiers.Update)
	adminOnly.DELETE("/tiers/:id", tiers.SoftDelete)
	adminOnly.POST("/tiers/:id/restore", tiers.Restore)

	// Customers
	customers := NewCustomerHandler(d.CustomerRepo, d.EncSvc).WithAudit(d.AuditSvc)
	authed.GET("/customers", customers.List)
	adminOrSpecialist.POST("/customers", customers.Create)
	authed.GET("/customers/:id", customers.Get)
	adminOrSpecialist.PUT("/customers/:id", customers.Update)
	adminOnly.DELETE("/customers/:id", customers.SoftDelete)
	adminOnly.POST("/customers/:id/restore", customers.Restore)

	// Fulfillments
	fulfillments := NewFulfillmentHandler(d.FulfillSvc, d.FulfillRepo, d.TimelineRepo, d.EncSvc)
	authed.GET("/fulfillments", fulfillments.List)
	adminOrSpecialist.POST("/fulfillments", fulfillments.Create)
	authed.GET("/fulfillments/:id", fulfillments.Get)
	adminOrSpecialist.POST("/fulfillments/:id/transition", fulfillments.Transition)
	authed.GET("/fulfillments/:id/timeline", fulfillments.Timeline)
	adminOnly.DELETE("/fulfillments/:id", fulfillments.SoftDelete)
	adminOnly.POST("/fulfillments/:id/restore", fulfillments.Restore)

	// Exceptions
	exceptions := NewExceptionHandler(d.ExceptionSvc, d.ExceptionRepo, d.ExEventRepo)
	adminOrSpecialist.GET("/exceptions", exceptions.List)
	adminOrSpecialist.POST("/exceptions", exceptions.Create)
	adminOrSpecialist.GET("/exceptions/:id", exceptions.Get)
	adminOrSpecialist.PUT("/exceptions/:id/status", exceptions.UpdateStatus)
	adminOrSpecialist.POST("/exceptions/:id/events", exceptions.AddEvent)

	// Messages
	messages := NewMessageHandler(d.MessagingSvc, d.TemplateRepo, d.SendLogRepo, d.NotifRepo)
	authed.GET("/message-templates", messages.ListTemplates)
	adminOnly.POST("/message-templates", messages.CreateTemplate)
	authed.GET("/message-templates/:id", messages.GetTemplate)
	adminOnly.PUT("/message-templates/:id", messages.UpdateTemplate)
	adminOnly.DELETE("/message-templates/:id", messages.DeleteTemplate)
	authed.GET("/send-logs", messages.ListSendLogs)
	adminOrSpecialist.PUT("/send-logs/:id/printed", messages.MarkPrinted)
	authed.GET("/notifications", messages.ListNotifications)
	authed.PUT("/notifications/:id/read", messages.MarkNotificationRead)
	adminOrSpecialist.POST("/dispatch", messages.Dispatch)

	// Reports
	reports := NewReportHandler(d.ReportRepo, d.ExportSvc, d.AuditSvc)
	adminOrAuditor.GET("/reports/exports", reports.List)
	adminOrAuditor.POST("/reports/exports", reports.Create)
	adminOrAuditor.GET("/reports/exports/:id", reports.Get)
	adminOrAuditor.POST("/reports/exports/:id/verify-checksum", reports.VerifyChecksum)
	adminOnly.DELETE("/reports/exports/:id", reports.Delete)

	// Settings
	settings := NewSettingsHandler(d.SettingRepo, d.BlackoutRepo).WithAudit(d.AuditSvc)
	authed.GET("/settings", settings.GetAll)
	adminOnly.PUT("/settings/:key", settings.Set)
	authed.GET("/settings/blackout-dates", settings.ListBlackoutDates)
	adminOnly.POST("/settings/blackout-dates", settings.CreateBlackoutDate)
	adminOnly.DELETE("/settings/blackout-dates/:id", settings.DeleteBlackoutDate)

	// Audit
	audit := NewAuditHandler(d.AuditRepo)
	adminOrAuditor.GET("/audit", audit.List)

	// Admin
	admin := NewAdminHandler(d.Pool, d.JobRunRepo, d.EncKeyPath, d.ExportDir, d.BackupDir).WithScheduler(d.Scheduler)
	adminOnly.GET("/admin/health", admin.Health)
	adminOnly.GET("/admin/jobs/runs", admin.ListJobRuns)
	adminOnly.POST("/admin/jobs/:name/run", admin.TriggerJob)

	// User management
	users := NewUserHandler(d.UserSvc)
	adminOnly.GET("/admin/users", users.List)
	adminOnly.POST("/admin/users", users.Create)
	adminOnly.GET("/admin/users/:id", users.Get)
	adminOnly.PUT("/admin/users/:id", users.Update)
	adminOnly.DELETE("/admin/users/:id", users.Deactivate)
}

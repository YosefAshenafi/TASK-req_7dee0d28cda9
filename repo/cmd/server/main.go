package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/config"
	"github.com/fulfillops/fulfillops/internal/handler"
	"github.com/fulfillops/fulfillops/internal/job"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config validation: %v", err)
	}

	gin.SetMode(cfg.GinMode)

	db, err := connectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := runMigrations(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	enc, err := service.NewEncryptionService(cfg.EncryptionKeyPath)
	if err != nil {
		log.Fatalf("encryption service: %v", err)
	}

	// ── Repositories ──────────────────────────────────────────────────────────
	tierRepo := repository.NewTierRepository(db)
	customerRepo := repository.NewCustomerRepository(db)
	fulfillRepo := repository.NewFulfillmentRepository(db)
	timelineRepo := repository.NewTimelineRepository(db)
	reservationRepo := repository.NewReservationRepository()
	shippingRepo := repository.NewShippingAddressRepository(db)
	exceptionRepo := repository.NewExceptionRepository(db)
	exEventRepo := repository.NewExceptionEventRepository(db)
	templateRepo := repository.NewMessageTemplateRepository(db)
	sendLogRepo := repository.NewSendLogRepository(db)
	notifRepo := repository.NewNotificationRepository(db)
	reportRepo := repository.NewReportExportRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	settingRepo := repository.NewSystemSettingRepository(db)
	blackoutRepo := repository.NewBlackoutDateRepository(db)
	jobRunRepo := repository.NewJobRunRepository(db)
	userRepo := repository.NewUserRepository(db)
	jobScheduleRepo := repository.NewJobScheduleRepository(db)
	drDrillRepo := repository.NewDRDrillRepository(db)

	// ── Services ──────────────────────────────────────────────────────────────
	txMgr := repository.NewTxManager(db)
	auditSvc := service.NewAuditService(auditRepo)
	userSvc := service.NewUserService(userRepo, auditSvc)
	invSvc := service.NewInventoryService(tierRepo, reservationRepo)
	fulfillSvc := service.NewFulfillmentService(
		txMgr, fulfillRepo, tierRepo, customerRepo, timelineRepo,
		shippingRepo, notifRepo, invSvc, auditSvc,
	)
	exceptionSvc := service.NewExceptionService(exceptionRepo, exEventRepo, auditSvc)
	messagingSvc := service.NewMessagingService(templateRepo, sendLogRepo, notifRepo, auditSvc)

	// Bootstrap the first administrator from env vars (no-op if one already exists).
	if cfg.BootstrapAdminEmail != "" && cfg.BootstrapAdminPassword != "" {
		if err := userSvc.BootstrapAdmin(context.Background(), "admin", cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword); err != nil {
			log.Printf("bootstrap admin: %v", err)
		} else {
			log.Println("bootstrap admin: complete")
		}
	}
	exportSvc := service.NewExportService(reportRepo, fulfillRepo, customerRepo, auditRepo, enc, cfg.ExportDir, auditSvc)
	backupSvc := service.NewBackupService(cfg.DatabaseURL, cfg.BackupDir, cfg.AssetsDir, auditSvc)

	slaSvc := service.NewSLAService(settingRepo, blackoutRepo)

	// ── Scheduler — load cadences from DB ────────────────────────────────────
	sched := job.NewScheduler(jobRunRepo)
	if cfg.SchedulerTimezone != "" {
		if loc, err := time.LoadLocation(cfg.SchedulerTimezone); err == nil {
			sched.WithTimezone(loc)
		} else {
			log.Printf("scheduler: unknown timezone %q, defaulting to UTC: %v", cfg.SchedulerTimezone, err)
		}
	}
	registerScheduledJobs(context.Background(), sched, jobScheduleRepo,
		fulfillRepo, tierRepo, exceptionRepo, slaSvc, messagingSvc, db, reportRepo, exportSvc, backupSvc)

	// ── Session store ─────────────────────────────────────────────────────────
	store := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   cfg.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Health endpoint (unauthenticated)
	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			log.Printf("healthz: db ping failed: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "error",
				"db":     "unreachable",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"db":     "connected",
		})
	})

	handler.RegisterRoutes(r, handler.Deps{
		Pool:            db,
		Store:           store,
		Scheduler:       sched,
		EncKeyPath:      cfg.EncryptionKeyPath,
		ExportDir:       cfg.ExportDir,
		BackupDir:       cfg.BackupDir,
		UserSvc:         userSvc,
		FulfillSvc:      fulfillSvc,
		ExceptionSvc:    exceptionSvc,
		MessagingSvc:    messagingSvc,
		AuditSvc:        auditSvc,
		EncSvc:          enc,
		ExportSvc:       exportSvc,
		BackupSvc:       backupSvc,
		TierRepo:        tierRepo,
		CustomerRepo:    customerRepo,
		FulfillRepo:     fulfillRepo,
		TimelineRepo:    timelineRepo,
		ShippingRepo:    shippingRepo,
		ExceptionRepo:   exceptionRepo,
		ExEventRepo:     exEventRepo,
		TemplateRepo:    templateRepo,
		SendLogRepo:     sendLogRepo,
		NotifRepo:       notifRepo,
		ReportRepo:      reportRepo,
		AuditRepo:       auditRepo,
		SettingRepo:     settingRepo,
		BlackoutRepo:    blackoutRepo,
		JobRunRepo:      jobRunRepo,
		UserRepo:        userRepo,
		JobScheduleRepo: jobScheduleRepo,
		DRDrillRepo:     drDrillRepo,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Start scheduler
	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	sched.Start(schedCtx)

	go func() {
		log.Printf("FulfillOps starting on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down...")

	sched.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("Server stopped.")
}

// registerScheduledJobs reads cadences from job_schedules and registers each
// job with the scheduler. Falls back to hard-coded defaults when the DB row is
// absent or the table is unavailable.
func registerScheduledJobs(
	ctx context.Context,
	sched *job.Scheduler,
	jobScheduleRepo repository.JobScheduleRepository,
	fulfillRepo repository.FulfillmentRepository,
	tierRepo repository.TierRepository,
	exceptionRepo repository.ExceptionRepository,
	slaSvc service.SLAService,
	messagingSvc service.MessagingService,
	db *pgxpool.Pool,
	reportRepo repository.ReportExportRepository,
	exportSvc service.ExportService,
	backupSvc service.BackupService,
) {
	schedules, err := jobScheduleRepo.List(ctx)
	if err != nil {
		log.Printf("scheduler: could not load job_schedules (%v); using hard-coded defaults", err)
	}

	// cadenceFor returns (isDaily, interval, hour, minute, enabled).
	cadenceFor := func(key string, defInterval time.Duration, defH, defM int) (bool, time.Duration, int, int, bool) {
		for _, s := range schedules {
			if s.JobKey != key {
				continue
			}
			if !s.Enabled {
				return false, 0, 0, 0, false
			}
			if s.IntervalSeconds != nil {
				return false, time.Duration(*s.IntervalSeconds) * time.Second, 0, 0, true
			}
			if s.DailyHour != nil && s.DailyMinute != nil {
				return true, 0, *s.DailyHour, *s.DailyMinute, true
			}
		}
		// Default.
		if defInterval > 0 {
			return false, defInterval, 0, 0, true
		}
		return true, 0, defH, defM, true
	}

	register := func(name string, defInterval time.Duration, defH, defM int, fn job.JobFunc) {
		daily, interval, h, m, enabled := cadenceFor(name, defInterval, defH, defM)
		if !enabled {
			log.Printf("scheduler: job %s is disabled by job_schedules", name)
			return
		}
		if daily {
			sched.RegisterDaily(name, h, m, fn)
		} else {
			sched.Register(name, interval, fn)
		}
	}

	register("overdue-check", 15*time.Minute, 0, 0,
		job.NewOverdueJob(fulfillRepo, exceptionRepo, slaSvc).Run)
	register("notify-retry", 10*time.Minute, 0, 0,
		job.NewNotifyJob(messagingSvc, 3).Run)
	register("cleanup", 0, 3, 0,
		job.NewCleanupJob(db, 30).Run)
	register("export-cleanup", 0, 3, 30,
		job.NewExportCleanupJob(reportRepo, exportSvc).Run)
	register("stats", 0, 2, 0,
		job.NewStatsJob(fulfillRepo, tierRepo).Run)
	register("backup", 0, 1, 0,
		job.NewBackupJob(backupSvc).Run)
	register("scheduled-reports", 0, 2, 30,
		job.NewScheduledReportJob(reportRepo, exportSvc).Run)
}

func connectDB(databaseURL string) (*pgxpool.Pool, error) {
	ctx := context.Background()
	var pool *pgxpool.Pool
	var err error

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		pool, err = pgxpool.New(ctx, databaseURL)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				return pool, nil
			}
			pool.Close()
		}
		log.Println("Waiting for database...")
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("could not connect to database after 30s: %w", err)
}

func runMigrations(databaseURL, migrationsPath string) error {
	m, err := migrate.New("file://"+migrationsPath, databaseURL)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("running migrations: %w", err)
	}
	log.Println("Migrations: up to date")
	return nil
}

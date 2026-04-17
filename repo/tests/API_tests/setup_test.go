// Package api_tests validates every HTTP endpoint against a live PostgreSQL
// database. All tests are skipped when DATABASE_URL is not set.
package api_tests

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/handler"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

var (
	testPool   *pgxpool.Pool
	testRouter *gin.Engine
	adminCookie *http.Cookie
)

func TestMain(m *testing.M) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("DATABASE_URL not set — skipping API tests")
		os.Exit(0)
	}

	gin.SetMode(gin.TestMode)
	ctx := context.Background()

	var err error
	testPool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("pgxpool.New: %v\n", err)
		os.Exit(1)
	}
	if err := testPool.Ping(ctx); err != nil {
		fmt.Printf("ping: %v\n", err)
		os.Exit(1)
	}

	testRouter = buildRouter(dbURL)
	adminCookie = mustAdminLogin()

	code := m.Run()
	testPool.Close()
	os.Exit(code)
}

func buildRouter(dbURL string) *gin.Engine {
	tierRepo := repository.NewTierRepository(testPool)
	customerRepo := repository.NewCustomerRepository(testPool)
	fulfillRepo := repository.NewFulfillmentRepository(testPool)
	timelineRepo := repository.NewTimelineRepository(testPool)
	reservationRepo := repository.NewReservationRepository()
	shippingRepo := repository.NewShippingAddressRepository(testPool)
	exceptionRepo := repository.NewExceptionRepository(testPool)
	exEventRepo := repository.NewExceptionEventRepository(testPool)
	templateRepo := repository.NewMessageTemplateRepository(testPool)
	sendLogRepo := repository.NewSendLogRepository(testPool)
	notifRepo := repository.NewNotificationRepository(testPool)
	reportRepo := repository.NewReportExportRepository(testPool)
	auditRepo := repository.NewAuditRepository(testPool)
	settingRepo := repository.NewSystemSettingRepository(testPool)
	blackoutRepo := repository.NewBlackoutDateRepository(testPool)
	jobRunRepo := repository.NewJobRunRepository(testPool)
	userRepo := repository.NewUserRepository(testPool)

	txMgr := repository.NewTxManager(testPool)
	auditSvc := service.NewAuditService(auditRepo)
	userSvc := service.NewUserService(userRepo, auditSvc)
	invSvc := service.NewInventoryService(tierRepo, reservationRepo)
	fulfillSvc := service.NewFulfillmentService(txMgr, fulfillRepo, tierRepo, timelineRepo, shippingRepo, notifRepo, invSvc, auditSvc)
	exceptionSvc := service.NewExceptionService(exceptionRepo, exEventRepo, auditSvc)
	messagingSvc := service.NewMessagingService(templateRepo, sendLogRepo, notifRepo)

	keyPath := os.Getenv("FULFILLOPS_ENCRYPTION_KEY_PATH")
	if keyPath == "" {
		keyPath = "/tmp/api_test_enc.key"
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			raw := make([]byte, 32)
			for i := range raw {
				raw[i] = byte(i + 1)
			}
			encoded := base64.StdEncoding.EncodeToString(raw)
			_ = os.WriteFile(keyPath, []byte(encoded+"\n"), 0600)
		}
	}
	enc, err := service.NewEncryptionService(keyPath)
	if err != nil {
		panic(fmt.Sprintf("encryption service: %v", err))
	}

	exportSvc := service.NewExportService(reportRepo, fulfillRepo, customerRepo, auditRepo, enc, os.TempDir())
	backupSvc := service.NewBackupService(dbURL, os.TempDir(), auditSvc)

	store := sessions.NewCookieStore([]byte("testsessionsecretchars32bytes000"))
	store.Options = &sessions.Options{Path: "/", MaxAge: 86400 * 7, HttpOnly: true}

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	handler.RegisterRoutes(r, handler.Deps{
		Pool: testPool, Store: store,
		UserSvc: userSvc, FulfillSvc: fulfillSvc, ExceptionSvc: exceptionSvc,
		MessagingSvc: messagingSvc, AuditSvc: auditSvc, EncSvc: enc,
		ExportSvc: exportSvc, BackupSvc: backupSvc,
		TierRepo: tierRepo, CustomerRepo: customerRepo, FulfillRepo: fulfillRepo,
		TimelineRepo: timelineRepo, ShippingRepo: shippingRepo,
		ExceptionRepo: exceptionRepo, ExEventRepo: exEventRepo,
		TemplateRepo: templateRepo, SendLogRepo: sendLogRepo, NotifRepo: notifRepo,
		ReportRepo: reportRepo, AuditRepo: auditRepo, SettingRepo: settingRepo,
		BlackoutRepo: blackoutRepo, JobRunRepo: jobRunRepo, UserRepo: userRepo,
	})
	return r
}

func mustAdminLogin() *http.Cookie {
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "Admin@FulfillOps1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		panic(fmt.Sprintf("admin login failed: %d %s", rr.Code, rr.Body.String()))
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == "fulfillops_session" {
			return c
		}
	}
	panic("session cookie not found after admin login")
}

// as sends an authenticated request using the provided cookie.
func as(cookie *http.Cookie, method, path string, body any) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)
	return rr
}

// admin is a shortcut for an admin-authenticated request.
func admin(method, path string, body any) *httptest.ResponseRecorder {
	return as(adminCookie, method, path, body)
}

// unauthed sends a request with no session cookie.
func unauthed(method, path string) *httptest.ResponseRecorder {
	return as(nil, method, path, nil)
}

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode JSON (status=%d): %v\nbody: %s", rr.Code, err, rr.Body.String())
	}
	return m
}

func mustStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Fatalf("expected HTTP %d, got %d\nbody: %s", want, rr.Code, rr.Body.String())
	}
}

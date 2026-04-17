// Package integration provides end-to-end tests against a live PostgreSQL
// database. All tests are skipped if DATABASE_URL is not set.
package integration_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/handler"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

// ── Test fixtures ─────────────────────────────────────────────────────────────

var (
	testPool   *pgxpool.Pool
	testRouter *gin.Engine
	testCookie *http.Cookie // session cookie for the admin user
)

func TestMain(m *testing.M) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("DATABASE_URL not set, skipping integration tests")
		os.Exit(0)
	}

	gin.SetMode(gin.TestMode)
	ctx := context.Background()

	var err error
	testPool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("connecting: %v\n", err)
		os.Exit(1)
	}
	if err := testPool.Ping(ctx); err != nil {
		fmt.Printf("ping: %v\n", err)
		os.Exit(1)
	}

	// Build a minimal router wired to the real DB.
	testRouter = buildRouter(dbURL)

	// Login as admin and capture session cookie.
	testCookie = adminLogin()

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
	fulfillSvc := service.NewFulfillmentService(txMgr, fulfillRepo, tierRepo, customerRepo, timelineRepo, shippingRepo, notifRepo, invSvc, auditSvc)
	exceptionSvc := service.NewExceptionService(exceptionRepo, exEventRepo, auditSvc)
	messagingSvc := service.NewMessagingService(templateRepo, sendLogRepo, notifRepo)

	keyPath := os.Getenv("FULFILLOPS_ENCRYPTION_KEY_PATH")
	if keyPath == "" {
		keyPath = "/tmp/test_encryption.key"
		// Generate a test key if it doesn't exist (base64-encoded 32 bytes).
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			key := make([]byte, 32)
			for i := range key {
				key[i] = byte(i + 1)
			}
			encoded := make([]byte, base64.StdEncoding.EncodedLen(32))
			base64.StdEncoding.Encode(encoded, key)
			_ = os.WriteFile(keyPath, encoded, 0600)
		}
	}
	enc, err := service.NewEncryptionService(keyPath)
	if err != nil {
		panic(fmt.Sprintf("encryption service: %v", err))
	}

	exportDir := os.TempDir()
	backupDir := os.TempDir()
	exportSvc := service.NewExportService(reportRepo, fulfillRepo, customerRepo, auditRepo, enc, exportDir)
	backupSvc := service.NewBackupService(dbURL, backupDir, auditSvc)

	store := sessions.NewCookieStore([]byte("testsessionsecretchars32bytes000"))
	store.Options = &sessions.Options{
		Path: "/", MaxAge: 86400 * 7, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	handler.RegisterRoutes(r, handler.Deps{
		Pool:          testPool,
		Store:         store,
		UserSvc:       userSvc,
		FulfillSvc:    fulfillSvc,
		ExceptionSvc:  exceptionSvc,
		MessagingSvc:  messagingSvc,
		AuditSvc:      auditSvc,
		EncSvc:        enc,
		ExportSvc:     exportSvc,
		BackupSvc:     backupSvc,
		TierRepo:      tierRepo,
		CustomerRepo:  customerRepo,
		FulfillRepo:   fulfillRepo,
		TimelineRepo:  timelineRepo,
		ShippingRepo:  shippingRepo,
		ExceptionRepo: exceptionRepo,
		ExEventRepo:   exEventRepo,
		TemplateRepo:  templateRepo,
		SendLogRepo:   sendLogRepo,
		NotifRepo:     notifRepo,
		ReportRepo:    reportRepo,
		AuditRepo:     auditRepo,
		SettingRepo:   settingRepo,
		BlackoutRepo:  blackoutRepo,
		JobRunRepo:    jobRunRepo,
		UserRepo:      userRepo,
	})
	return r
}

// adminLogin logs in as the seeded admin and returns the session cookie.
func adminLogin() *http.Cookie {
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

// authedRequest creates a test request with the admin session cookie.
func authedRequest(method, path string, body any) *http.Request {
	var bodyReader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(testCookie)
	return req
}

func do(req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)
	return rr
}

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("decoding JSON: %v\nbody: %s", err, rr.Body.String())
	}
	return m
}

func transitionAuthed(t *testing.T, ffID string, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	rr := do(authedRequest(http.MethodGet, "/api/v1/fulfillments/"+ffID, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("load fulfillment for version: %d %s", rr.Code, rr.Body.String())
	}
	version := int(decodeJSON(t, rr)["version"].(float64))
	if payload == nil {
		payload = map[string]any{}
	}
	payload["version"] = version
	return do(authedRequest(http.MethodPost, "/api/v1/fulfillments/"+ffID+"/transition", payload))
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestHealthz verifies the unauthenticated health endpoint.
func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := do(req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// TestFulfillmentLifecycle walks a fulfillment through DRAFT → READY_TO_SHIP →
// SHIPPED → DELIVERED → COMPLETED, verifying state at each step.
func TestFulfillmentLifecycle(t *testing.T) {
	ctx := context.Background()

	// Create tier.
	rr := do(authedRequest(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name":            fmt.Sprintf("lifecycle-tier-%d", time.Now().UnixNano()),
		"inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1,
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create tier: %d %s", rr.Code, rr.Body.String())
	}
	tier := decodeJSON(t, rr)
	tierID := tier["id"].(string)

	// Create customer.
	rr = do(authedRequest(http.MethodPost, "/api/v1/customers", map[string]any{
		"name": "Lifecycle Customer", "phone": "5551112222", "email": "lifecycle@test.com",
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create customer: %d %s", rr.Code, rr.Body.String())
	}
	cust := decodeJSON(t, rr)
	custID := cust["id"].(string)

	// Create fulfillment.
	rr = do(authedRequest(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "PHYSICAL",
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create fulfillment: %d %s", rr.Code, rr.Body.String())
	}
	ff := decodeJSON(t, rr)
	ffID := ff["id"].(string)
	if ff["status"] != "DRAFT" {
		t.Fatalf("expected DRAFT, got %s", ff["status"])
	}

	// DRAFT → READY_TO_SHIP
	rr = transitionAuthed(t, ffID, map[string]any{"to_status": "READY_TO_SHIP"})
	if rr.Code != http.StatusOK {
		t.Fatalf("→ READY_TO_SHIP: %d %s", rr.Code, rr.Body.String())
	}

	// READY_TO_SHIP → SHIPPED (requires tracking)
	rr = transitionAuthed(t, ffID, map[string]any{"to_status": "SHIPPED", "carrier_name": "FedEx", "tracking_number": "1Z999AA10123456784"})
	if rr.Code != http.StatusOK {
		t.Fatalf("→ SHIPPED: %d %s", rr.Code, rr.Body.String())
	}

	// SHIPPED → DELIVERED
	rr = transitionAuthed(t, ffID, map[string]any{"to_status": "DELIVERED"})
	if rr.Code != http.StatusOK {
		t.Fatalf("→ DELIVERED: %d %s", rr.Code, rr.Body.String())
	}

	// DELIVERED → COMPLETED
	rr = transitionAuthed(t, ffID, map[string]any{"to_status": "COMPLETED"})
	if rr.Code != http.StatusOK {
		t.Fatalf("→ COMPLETED: %d %s", rr.Code, rr.Body.String())
	}
	ff = decodeJSON(t, rr)
	if ff["status"] != "COMPLETED" {
		t.Fatalf("expected COMPLETED, got %s", ff["status"])
	}

	// Inventory was decremented.
	rr = do(authedRequest(http.MethodGet, "/api/v1/tiers/"+tierID, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get tier: %d", rr.Code)
	}
	tierData := decodeJSON(t, rr)
	if inv := int(tierData["inventory_count"].(float64)); inv != 4 {
		t.Fatalf("expected inventory 4, got %d", inv)
	}

	// Timeline should have multiple entries.
	rr = do(authedRequest(http.MethodGet, "/api/v1/fulfillments/"+ffID+"/timeline", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get timeline: %d", rr.Code)
	}
	var timelineResp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &timelineResp)
	items, _ := timelineResp["items"].([]any)
	if len(items) < 4 {
		t.Fatalf("expected ≥4 timeline entries, got %d (body: %s)", len(items), rr.Body.String())
	}
	_ = ctx
}

// TestCancelFlow verifies that canceling a fulfillment restores inventory.
func TestCancelFlow(t *testing.T) {
	// Create tier.
	rr := do(authedRequest(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name":            fmt.Sprintf("cancel-tier-%d", time.Now().UnixNano()),
		"inventory_count": 3, "purchase_limit": 2, "alert_threshold": 1,
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create tier: %d", rr.Code)
	}
	tier := decodeJSON(t, rr)
	tierID := tier["id"].(string)

	rr = do(authedRequest(http.MethodPost, "/api/v1/customers", map[string]any{
		"name": "Cancel Customer", "phone": "5553334444",
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create customer: %d", rr.Code)
	}
	custID := decodeJSON(t, rr)["id"].(string)

	rr = do(authedRequest(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "PHYSICAL",
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create fulfillment: %d", rr.Code)
	}
	ffID := decodeJSON(t, rr)["id"].(string)

	// Inventory decremented to 2.
	_ = transitionAuthed(t, ffID, map[string]any{"to_status": "READY_TO_SHIP"})

	// Cancel from READY_TO_SHIP.
	rr = transitionAuthed(t, ffID, map[string]any{"to_status": "CANCELED", "reason": "Customer request"})
	if rr.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", rr.Code, rr.Body.String())
	}

	// Inventory restored to 3.
	rr = do(authedRequest(http.MethodGet, "/api/v1/tiers/"+tierID, nil))
	tierData := decodeJSON(t, rr)
	if inv := int(tierData["inventory_count"].(float64)); inv != 3 {
		t.Fatalf("expected inventory 3 after cancel, got %d", inv)
	}
}

// TestConcurrentInventory verifies that two concurrent creates against
// inventory=1 result in exactly one success and one INVENTORY_UNAVAILABLE error.
func TestConcurrentInventory(t *testing.T) {
	rr := do(authedRequest(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name":            fmt.Sprintf("concurrent-tier-%d", time.Now().UnixNano()),
		"inventory_count": 1, "purchase_limit": 5, "alert_threshold": 0,
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create tier: %d", rr.Code)
	}
	tierID := decodeJSON(t, rr)["id"].(string)

	// Create two customers.
	custIDs := make([]string, 2)
	for i := range custIDs {
		rr = do(authedRequest(http.MethodPost, "/api/v1/customers", map[string]any{
			"name": fmt.Sprintf("Concurrent Customer %d", i),
		}))
		if rr.Code != http.StatusCreated {
			t.Fatalf("create customer %d: %d", i, rr.Code)
		}
		custIDs[i] = decodeJSON(t, rr)["id"].(string)
	}

	type result struct {
		code int
		body string
	}
	results := make([]result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rr := do(authedRequest(http.MethodPost, "/api/v1/fulfillments", map[string]any{
				"tier_id": tierID, "customer_id": custIDs[idx], "type": "PHYSICAL",
			}))
			results[idx] = result{code: rr.Code, body: rr.Body.String()}
		}(i)
	}
	wg.Wait()

	successes := 0
	failures := 0
	for _, r := range results {
		if r.code == http.StatusCreated {
			successes++
		} else if r.code == http.StatusUnprocessableEntity {
			failures++
		}
	}

	if successes != 1 || failures != 1 {
		t.Fatalf("expected 1 success + 1 INVENTORY_UNAVAILABLE, got %d/%d\n  results: %+v", successes, failures, results)
	}
}

// TestPurchaseLimitReached verifies the 2-per-tier-per-30-days limit.
func TestPurchaseLimitReached(t *testing.T) {
	rr := do(authedRequest(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name":            fmt.Sprintf("limit-tier-%d", time.Now().UnixNano()),
		"inventory_count": 10, "purchase_limit": 2, "alert_threshold": 0,
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create tier: %d", rr.Code)
	}
	tierID := decodeJSON(t, rr)["id"].(string)

	rr = do(authedRequest(http.MethodPost, "/api/v1/customers", map[string]any{
		"name": "Limit Customer",
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create customer: %d", rr.Code)
	}
	custID := decodeJSON(t, rr)["id"].(string)

	create := func() int {
		rr := do(authedRequest(http.MethodPost, "/api/v1/fulfillments", map[string]any{
			"tier_id": tierID, "customer_id": custID, "type": "VOUCHER",
		}))
		return rr.Code
	}

	if c := create(); c != http.StatusCreated {
		t.Fatalf("1st create: %d", c)
	}
	if c := create(); c != http.StatusCreated {
		t.Fatalf("2nd create: %d", c)
	}
	// 3rd should fail with PURCHASE_LIMIT_REACHED (422).
	if c := create(); c != http.StatusUnprocessableEntity {
		t.Fatalf("3rd create expected 422, got %d", c)
	}
}

// TestSoftDeleteRestore verifies soft-delete and restore of a tier.
func TestSoftDeleteRestore(t *testing.T) {
	rr := do(authedRequest(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name":            fmt.Sprintf("delete-restore-tier-%d", time.Now().UnixNano()),
		"inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1,
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create tier: %d", rr.Code)
	}
	tierID := decodeJSON(t, rr)["id"].(string)

	// Delete → 204
	rr = do(authedRequest(http.MethodDelete, "/api/v1/tiers/"+tierID, nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete tier: %d", rr.Code)
	}

	// GET → 404
	rr = do(authedRequest(http.MethodGet, "/api/v1/tiers/"+tierID, nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rr.Code)
	}

	// Restore → 200
	rr = do(authedRequest(http.MethodPost, "/api/v1/tiers/"+tierID+"/restore", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("restore tier: %d %s", rr.Code, rr.Body.String())
	}

	// GET → 200
	rr = do(authedRequest(http.MethodGet, "/api/v1/tiers/"+tierID, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 after restore, got %d", rr.Code)
	}
}

// TestRBACAccessControl verifies role-based access control for all three roles.
func TestRBACAccessControl(t *testing.T) {
	ctx := context.Background()
	userRepo := repository.NewUserRepository(testPool)
	auditSvc := service.NewAuditService(repository.NewAuditRepository(testPool))
	userSvc := service.NewUserService(userRepo, auditSvc)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create a fulfillment specialist user.
	specUser := "spec_" + suffix
	specialist, err := userSvc.CreateUser(ctx, specUser, specUser+"@test.com", "Spec@Test1!", domain.RoleFulfillmentSpecialist)
	if err != nil {
		t.Fatalf("create specialist: %v", err)
	}
	defer func() { _ = userSvc.DeactivateUser(ctx, specialist.ID) }()

	// Create an auditor user.
	audUser := "aud_" + suffix
	auditor, err := userSvc.CreateUser(ctx, audUser, audUser+"@test.com", "Audit@Test1!", domain.RoleAuditor)
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}
	defer func() { _ = userSvc.DeactivateUser(ctx, auditor.ID) }()

	// Login as specialist.
	specBody, _ := json.Marshal(map[string]string{"username": specUser, "password": "Spec@Test1!"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(specBody))
	req.Header.Set("Content-Type", "application/json")
	rr := do(req)
	if rr.Code != http.StatusOK {
		t.Fatalf("specialist login: %d", rr.Code)
	}
	var specCookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "fulfillops_session" {
			specCookie = c
		}
	}

	specReq := func(method, path string) *http.Request {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(specCookie)
		return req
	}

	// Specialist can GET tiers.
	rr = do(specReq(http.MethodGet, "/api/v1/tiers"))
	if rr.Code != http.StatusOK {
		t.Fatalf("specialist GET tiers: %d", rr.Code)
	}

	// Specialist cannot DELETE tiers.
	rr = do(specReq(http.MethodDelete, "/api/v1/tiers/00000000-0000-0000-0000-000000000001"))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("specialist DELETE tier: expected 403, got %d", rr.Code)
	}

	// Specialist cannot access admin endpoints.
	rr = do(specReq(http.MethodGet, "/api/v1/admin/health"))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("specialist GET admin/health: expected 403, got %d", rr.Code)
	}

	// Login as auditor.
	audBody, _ := json.Marshal(map[string]string{"username": audUser, "password": "Audit@Test1!"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(audBody))
	req.Header.Set("Content-Type", "application/json")
	rr = do(req)
	if rr.Code != http.StatusOK {
		t.Fatalf("auditor login: %d", rr.Code)
	}
	var audCookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "fulfillops_session" {
			audCookie = c
		}
	}

	audReq := func(method, path string) *http.Request {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(audCookie)
		return req
	}

	// Auditor can list audit logs.
	rr = do(audReq(http.MethodGet, "/api/v1/audit"))
	if rr.Code != http.StatusOK {
		t.Fatalf("auditor GET audit: %d", rr.Code)
	}

	// Auditor cannot create tiers.
	rr = do(audReq(http.MethodPost, "/api/v1/tiers"))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("auditor POST tier: expected 403, got %d", rr.Code)
	}

	// Unauthenticated → 401.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tiers", nil)
	rr = do(req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth GET tiers: expected 401, got %d", rr.Code)
	}
}

// TestExceptionFlow verifies exception creation and status update.
func TestExceptionFlow(t *testing.T) {
	// Create tier + customer + fulfillment.
	rr := do(authedRequest(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name":            fmt.Sprintf("exception-tier-%d", time.Now().UnixNano()),
		"inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1,
	}))
	tierID := decodeJSON(t, rr)["id"].(string)

	rr = do(authedRequest(http.MethodPost, "/api/v1/customers", map[string]any{"name": "Exception Cust"}))
	custID := decodeJSON(t, rr)["id"].(string)

	rr = do(authedRequest(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "VOUCHER",
	}))
	ffID := decodeJSON(t, rr)["id"].(string)

	// Create exception.
	rr = do(authedRequest(http.MethodPost, "/api/v1/exceptions", map[string]any{
		"fulfillment_id": ffID,
		"type":           "MANUAL",
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create exception: %d %s", rr.Code, rr.Body.String())
	}
	excID := decodeJSON(t, rr)["id"].(string)

	// Add event.
	rr = do(authedRequest(http.MethodPost, "/api/v1/exceptions/"+excID+"/events", map[string]any{
		"event_type": "NOTE",
		"content":    "Inspecting item",
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("add event: %d %s", rr.Code, rr.Body.String())
	}

	// Resolve with note.
	rr = do(authedRequest(http.MethodPut, "/api/v1/exceptions/"+excID+"/status", map[string]any{
		"status":          "RESOLVED",
		"resolution_note": "Replacement sent",
	}))
	if rr.Code != http.StatusOK {
		t.Fatalf("resolve exception: %d %s", rr.Code, rr.Body.String())
	}
	exc := decodeJSON(t, rr)
	if exc["status"] != "RESOLVED" {
		t.Fatalf("expected RESOLVED, got %s", exc["status"])
	}
}

// TestReportExportFlow verifies CSV generation and checksum verification.
func TestReportExportFlow(t *testing.T) {
	rr := do(authedRequest(http.MethodPost, "/api/v1/reports/exports", map[string]any{
		"report_type":       "fulfillments",
		"include_sensitive": false,
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create export: %d %s", rr.Code, rr.Body.String())
	}
	exp := decodeJSON(t, rr)
	exportID := exp["id"].(string)

	// Wait for background generation.
	var completed bool
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		rr = do(authedRequest(http.MethodGet, "/api/v1/reports/exports/"+exportID, nil))
		exp = decodeJSON(t, rr)
		if exp["status"] == "COMPLETED" {
			completed = true
			break
		}
	}
	if !completed {
		t.Fatalf("export did not complete: status=%s", exp["status"])
	}
	if exp["checksum_sha256"] == nil {
		t.Fatal("checksum missing from completed export")
	}

	// Verify checksum.
	rr = do(authedRequest(http.MethodPost, "/api/v1/reports/exports/"+exportID+"/verify-checksum", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("verify checksum: %d", rr.Code)
	}
	result := decodeJSON(t, rr)
	if result["verified"] != true {
		t.Fatal("expected verified=true")
	}
}

// TestConflictOnStaleVersion verifies that optimistic locking prevents
// concurrent overwrites via version mismatch.
func TestConflictOnStaleVersion(t *testing.T) {
	rr := do(authedRequest(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name":            fmt.Sprintf("conflict-tier-%d", time.Now().UnixNano()),
		"inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1,
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create tier: %d", rr.Code)
	}
	tier := decodeJSON(t, rr)
	tierID := tier["id"].(string)

	// Update successfully with version=1.
	rr = do(authedRequest(http.MethodPut, "/api/v1/tiers/"+tierID, map[string]any{
		"name": "Updated Tier", "inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1,
		"version": 1,
	}))
	if rr.Code != http.StatusOK {
		t.Fatalf("first update: %d %s", rr.Code, rr.Body.String())
	}

	// Second update with stale version=1 should conflict.
	rr = do(authedRequest(http.MethodPut, "/api/v1/tiers/"+tierID, map[string]any{
		"name": "Stale Update", "inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1,
		"version": 1,
	}))
	if rr.Code != http.StatusConflict {
		t.Fatalf("stale version: expected 409, got %d %s", rr.Code, rr.Body.String())
	}
}

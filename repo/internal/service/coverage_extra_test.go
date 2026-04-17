package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// ── validateShippingAddress (pure) ───────────────────────────────────────────

func TestValidateShippingAddress_AllBranches(t *testing.T) {
	cases := []struct {
		name    string
		addr    ShippingAddressEncrypted
		wantErr bool
	}{
		{"missing line1", ShippingAddressEncrypted{City: "X", State: "CA", ZipCode: "94105"}, true},
		{"missing city", ShippingAddressEncrypted{Line1Encrypted: []byte{1}, State: "CA", ZipCode: "94105"}, true},
		{"bad state", ShippingAddressEncrypted{Line1Encrypted: []byte{1}, City: "X", State: "California", ZipCode: "94105"}, true},
		{"bad zip", ShippingAddressEncrypted{Line1Encrypted: []byte{1}, City: "X", State: "CA", ZipCode: "9410"}, true},
		{"ok 5-digit", ShippingAddressEncrypted{Line1Encrypted: []byte{1}, City: "X", State: "CA", ZipCode: "94105"}, false},
		{"ok 9-digit", ShippingAddressEncrypted{Line1Encrypted: []byte{1}, City: "X", State: "CA", ZipCode: "94105-1234"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateShippingAddress(&tc.addr)
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v got=%v", tc.wantErr, err)
			}
		})
	}
}

// ── Messaging.RetryPending ───────────────────────────────────────────────────

type retryStubSendLog struct {
	retryables []domain.SendLog
	updates    map[uuid.UUID]domain.SendLogStatus
	nextRetry  map[uuid.UUID]time.Time
	updateErr  error
}

func (s *retryStubSendLog) Create(context.Context, *domain.SendLog) (*domain.SendLog, error) {
	return nil, nil
}
func (s *retryStubSendLog) UpdateStatus(_ context.Context, id uuid.UUID, status domain.SendLogStatus, _ *string) error {
	if s.updates == nil {
		s.updates = map[uuid.UUID]domain.SendLogStatus{}
	}
	s.updates[id] = status
	return s.updateErr
}
func (s *retryStubSendLog) UpdateNextRetry(_ context.Context, id uuid.UUID, at time.Time) error {
	if s.nextRetry == nil {
		s.nextRetry = map[uuid.UUID]time.Time{}
	}
	s.nextRetry[id] = at
	return nil
}
func (s *retryStubSendLog) MarkPrinted(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (s *retryStubSendLog) List(context.Context, repository.SendLogFilters, domain.PageRequest) ([]domain.SendLog, int, error) {
	return nil, 0, nil
}
func (s *retryStubSendLog) GetRetryable(context.Context, time.Time) ([]domain.SendLog, error) {
	return s.retryables, nil
}

type retryStubNotif struct {
	created []*domain.Notification
	err     error
}

func (s *retryStubNotif) Create(_ context.Context, n *domain.Notification) (*domain.Notification, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.created = append(s.created, n)
	return n, nil
}
func (s *retryStubNotif) CreateTx(context.Context, pgx.Tx, *domain.Notification) error {
	return nil
}
func (s *retryStubNotif) ListByUserID(context.Context, uuid.UUID, *bool, domain.PageRequest) ([]domain.Notification, int, error) {
	return nil, 0, nil
}
func (s *retryStubNotif) MarkRead(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func TestMessagingRetryPending(t *testing.T) {
	tmplID := uuid.New()
	tmpl := &domain.MessageTemplate{
		ID:           tmplID,
		Name:         "Tmpl",
		Channel:      domain.ChannelInApp,
		BodyTemplate: "hello",
	}
	templRepo := &stubTemplateRepo{tmpl: tmpl}

	// Build pending send logs: 1 in-app (retryable), 1 SMS (retryable), 1 over max attempts.
	inAppID := uuid.New()
	smsID := uuid.New()
	deadID := uuid.New()
	sendRepo := &retryStubSendLog{
		retryables: []domain.SendLog{
			{ID: inAppID, Channel: domain.ChannelInApp, TemplateID: &tmplID, RecipientID: uuid.New(), AttemptCount: 0},
			{ID: smsID, Channel: domain.ChannelSMS, RecipientID: uuid.New(), AttemptCount: 1},
			{ID: deadID, Channel: domain.ChannelEmail, RecipientID: uuid.New(), AttemptCount: 10},
		},
	}
	notifRepo := &stubNotificationRepo{}
	svc := NewMessagingService(templRepo, sendRepo, notifRepo)

	retried, err := svc.RetryPending(context.Background(), 3)
	if err != nil {
		t.Fatalf("RetryPending: %v", err)
	}
	if retried != 2 {
		t.Fatalf("retried = %d, want 2", retried)
	}
	if sendRepo.updates[deadID] != domain.SendFailed {
		t.Fatalf("over-attempt entry should be marked failed, got %v", sendRepo.updates[deadID])
	}
	if sendRepo.updates[inAppID] != domain.SendSent {
		t.Fatalf("in-app retry should be marked sent, got %v", sendRepo.updates[inAppID])
	}
	if sendRepo.updates[smsID] != domain.SendQueued {
		t.Fatalf("sms retry should be re-queued, got %v", sendRepo.updates[smsID])
	}
	if _, ok := sendRepo.nextRetry[smsID]; !ok {
		t.Fatalf("sms retry should schedule a next-retry timestamp")
	}
}

func TestMessagingRetryPending_InAppFallbackSchedules(t *testing.T) {
	tmplID := uuid.New()
	tmpl := &domain.MessageTemplate{ID: tmplID, Name: "N", Channel: domain.ChannelInApp, BodyTemplate: "b"}
	templRepo := &stubTemplateRepo{tmpl: tmpl}

	inAppID := uuid.New()
	sendRepo := &retryStubSendLog{
		retryables: []domain.SendLog{
			{ID: inAppID, Channel: domain.ChannelInApp, TemplateID: &tmplID, RecipientID: uuid.New(), AttemptCount: 0},
		},
	}
	notifRepo := &failingNotifRepo{}
	svc := NewMessagingService(templRepo, sendRepo, notifRepo)

	retried, err := svc.RetryPending(context.Background(), 3)
	if err != nil {
		t.Fatalf("RetryPending: %v", err)
	}
	if retried != 1 {
		t.Fatalf("retried = %d, want 1", retried)
	}
	if sendRepo.updates[inAppID] != domain.SendQueued {
		t.Fatalf("failed in-app retry should re-queue, got %v", sendRepo.updates[inAppID])
	}
	if _, ok := sendRepo.nextRetry[inAppID]; !ok {
		t.Fatalf("failed in-app retry should schedule next-retry")
	}
}

// failingNotifRepo is a NotificationRepository whose Create always errors.
type failingNotifRepo struct{}

func (f *failingNotifRepo) Create(context.Context, *domain.Notification) (*domain.Notification, error) {
	return nil, errors.New("notify down")
}
func (f *failingNotifRepo) CreateTx(context.Context, pgx.Tx, *domain.Notification) error {
	return nil
}
func (f *failingNotifRepo) ListByUserID(context.Context, uuid.UUID, *bool, domain.PageRequest) ([]domain.Notification, int, error) {
	return nil, 0, nil
}
func (f *failingNotifRepo) MarkRead(context.Context, uuid.UUID, uuid.UUID) error { return nil }

// ── Export: writeCustomersCSV + writeAuditCSV + failExport ───────────────────

type stubAuditListRepo struct {
	items []domain.AuditLog
	err   error
}

func (s *stubAuditListRepo) Create(context.Context, *domain.AuditLog) error { return nil }
func (s *stubAuditListRepo) List(context.Context, repository.AuditFilters, domain.PageRequest) ([]domain.AuditLog, int, error) {
	return s.items, len(s.items), s.err
}

func TestExportWriteCustomersCSV_Masked(t *testing.T) {
	enc := newTestEncryptionService(t)
	phoneCipher, _ := enc.EncryptString("5551234567")
	emailCipher, _ := enc.EncryptString("person@example.com")

	report := &domain.ReportExport{ID: uuid.New(), ReportType: "customers", Filters: []byte(`{}`), IncludeSensitive: false}
	reportRepo := &stubReportRepo{export: report}

	custRepo := &stubCustomerRepo{listItems: []domain.Customer{
		{ID: uuid.New(), Name: "Pat", PhoneEncrypted: phoneCipher, EmailEncrypted: emailCipher, CreatedAt: time.Now().UTC()},
	}}
	auditRepo := &stubAuditListRepo{}
	fulfillRepo := &stubFulfillmentRepo{}
	tmp := t.TempDir()
	svc := NewExportService(reportRepo, fulfillRepo, custRepo, auditRepo, enc, tmp)

	if err := svc.GenerateExport(context.Background(), report.ID); err != nil {
		t.Fatalf("GenerateExport: %v", err)
	}
	if report.Status != domain.ExportCompleted || report.FilePath == nil {
		t.Fatalf("expected completed export, got status=%s file=%v", report.Status, report.FilePath)
	}
	content, err := os.ReadFile(*report.FilePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "***-***-4567") {
		t.Fatalf("expected masked phone in csv, got:\n%s", s)
	}
	if !strings.Contains(s, "p***@example.com") {
		t.Fatalf("expected masked email in csv, got:\n%s", s)
	}
}

func TestExportWriteAuditCSV(t *testing.T) {
	enc := newTestEncryptionService(t)
	report := &domain.ReportExport{ID: uuid.New(), ReportType: "audit", Filters: []byte(`{}`)}
	reportRepo := &stubReportRepo{export: report}
	ip := "10.0.0.1"
	reqID := "req-42"
	actor := uuid.New()
	rec := uuid.New()
	auditRepo := &stubAuditListRepo{items: []domain.AuditLog{
		{ID: uuid.New(), TableName: "users", RecordID: &rec, Operation: "UPDATE", PerformedBy: &actor, IPAddress: &ip, RequestID: &reqID, CreatedAt: time.Now().UTC()},
		{ID: uuid.New(), TableName: "tiers", Operation: "DELETE", CreatedAt: time.Now().UTC()},
	}}
	tmp := t.TempDir()
	svc := NewExportService(reportRepo, &stubFulfillmentRepo{}, &stubCustomerRepo{}, auditRepo, enc, tmp)

	if err := svc.GenerateExport(context.Background(), report.ID); err != nil {
		t.Fatalf("GenerateExport: %v", err)
	}
	content, _ := os.ReadFile(*report.FilePath)
	s := string(content)
	if !strings.Contains(s, "10.0.0.1") || !strings.Contains(s, "req-42") || !strings.Contains(s, "UPDATE") {
		t.Fatalf("expected audit rows in csv, got:\n%s", s)
	}
}

func TestExportFailExport_UnknownReportType(t *testing.T) {
	enc := newTestEncryptionService(t)
	report := &domain.ReportExport{ID: uuid.New(), ReportType: "bogus", Filters: []byte(`{}`)}
	reportRepo := &stubReportRepo{export: report}
	tmp := t.TempDir()
	svc := NewExportService(reportRepo, &stubFulfillmentRepo{}, &stubCustomerRepo{}, &stubAuditListRepo{}, enc, tmp)

	err := svc.GenerateExport(context.Background(), report.ID)
	if err == nil {
		t.Fatalf("expected unknown-report-type error")
	}
	if report.Status != domain.ExportFailed {
		t.Fatalf("expected ExportFailed, got %s", report.Status)
	}
}

func TestExportFailExport_OnReportGetError(t *testing.T) {
	enc := newTestEncryptionService(t)
	reportRepo := &stubReportRepo{} // export=nil → GetByID returns NotFound
	tmp := t.TempDir()
	svc := NewExportService(reportRepo, &stubFulfillmentRepo{}, &stubCustomerRepo{}, &stubAuditListRepo{}, enc, tmp)
	if err := svc.GenerateExport(context.Background(), uuid.New()); err == nil {
		t.Fatalf("expected error when report record missing")
	}
}

// ── User GetByID / List ──────────────────────────────────────────────────────

func TestUserService_GetByIDAndList(t *testing.T) {
	u := &domain.User{ID: uuid.New(), Username: "u", IsActive: true, Role: domain.RoleAuditor}
	repo := &stubUserRepo{
		byID:   map[uuid.UUID]*domain.User{u.ID: u},
		byName: map[string]*domain.User{u.Username: u},
		list:   []domain.User{*u},
	}
	svc := NewUserService(repo, nil)

	got, err := svc.GetByID(context.Background(), u.ID)
	if err != nil || got == nil || got.ID != u.ID {
		t.Fatalf("GetByID = (%v, %v)", got, err)
	}
	items, err := svc.List(context.Background(), "", nil)
	if err != nil || len(items) != 1 {
		t.Fatalf("List = (%d, %v)", len(items), err)
	}
}

func TestUserService_ValidationErrors(t *testing.T) {
	repo := &stubUserRepo{}
	svc := NewUserService(repo, nil)

	if _, err := svc.CreateUser(context.Background(), "u", "u@x", "Password123", "INVALID"); err == nil {
		t.Fatal("expected invalid role error")
	}
	if _, err := svc.CreateUser(context.Background(), "u", "u@x", "short", domain.RoleAuditor); err == nil {
		t.Fatal("expected weak password error")
	}
	if _, err := svc.UpdateUser(context.Background(), uuid.New(), "u@x", "INVALID"); err == nil {
		t.Fatal("expected invalid role on update")
	}

	// Authenticate: inactive user
	inactive := &domain.User{ID: uuid.New(), Username: "inact", IsActive: false, PasswordHash: "x"}
	repo.byName = map[string]*domain.User{"inact": inactive}
	if _, err := svc.Authenticate(context.Background(), "inact", "x"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for inactive user, got %v", err)
	}
	// Authenticate: missing user
	if _, err := svc.Authenticate(context.Background(), "nope", "x"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for missing user, got %v", err)
	}
}

// ── Exception Service validation branches ────────────────────────────────────

func TestExceptionService_ValidationAndErrors(t *testing.T) {
	exRepo := &stubExceptionRepo{}
	evRepo := &stubExceptionEventRepo{}
	svc := NewExceptionService(exRepo, evRepo, nil)

	if _, err := svc.Create(context.Background(), uuid.New(), "INVALID", ""); err == nil {
		t.Fatal("expected invalid type error")
	}
	if _, err := svc.UpdateStatus(context.Background(), uuid.New(), "BOGUS", ""); err == nil {
		t.Fatal("expected invalid status error")
	}
	// Need existing exception for UpdateStatus to pass validation but fail on missing note for RESOLVED
	created, _ := svc.Create(context.Background(), uuid.New(), domain.ExceptionManual, "n")
	if _, err := svc.UpdateStatus(context.Background(), created.ID, domain.ExceptionResolved, ""); err == nil {
		t.Fatal("expected resolution note required error")
	}
	// AddEvent validation
	if _, err := svc.AddEvent(context.Background(), uuid.New(), "", "body"); err == nil {
		t.Fatal("expected missing event_type error")
	}
	if _, err := svc.AddEvent(context.Background(), uuid.New(), "NOTE", ""); err == nil {
		t.Fatal("expected missing content error")
	}
	// GetByID remap to NotFound
	missingRepo := &stubExceptionRepo{err: domain.ErrNotFound}
	s2 := NewExceptionService(missingRepo, evRepo, nil)
	if _, err := s2.GetByID(context.Background(), uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ── Encryption error paths ───────────────────────────────────────────────────

func TestEncryptionService_DecryptErrors(t *testing.T) {
	enc := newTestEncryptionService(t)
	// Too-short ciphertext (less than nonce size)
	if _, err := enc.Decrypt([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected too-short error")
	}
	// Garbage ciphertext (right prefix length, wrong bytes)
	garbage := make([]byte, 32)
	if _, err := enc.Decrypt(garbage); err == nil {
		t.Fatal("expected GCM open error")
	}
	if _, err := enc.DecryptToString(garbage); err == nil {
		t.Fatal("expected GCM open error (string)")
	}
}

// ── Backup: RunBackup hits pg_dump error path when binary missing ────────────

func TestBackupService_RunBackupPgDumpMissing(t *testing.T) {
	dir := t.TempDir()
	svc := NewBackupService("postgres://invalid", dir, nil)
	// In the golang:1.23-alpine test image pg_dump is not installed, so this
	// exercises: file create → pg_dump spawn error → cleanup → error return.
	_, err := svc.RunBackup(context.Background())
	if err == nil {
		t.Fatal("expected RunBackup to fail without pg_dump")
	}
	// Ensure no orphan backup file was left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql.gz") {
			t.Fatalf("expected failed backup to remove partial file, found %s", e.Name())
		}
	}
}

func TestBackupService_RestoreCorruptFile(t *testing.T) {
	dir := t.TempDir()
	// Write a non-gzip file under the expected backup name.
	bk := "bad_backup_20260101_010101"
	if err := os.WriteFile(filepath.Join(dir, bk+".sql.gz"), []byte("not gzip"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := NewBackupService("postgres://invalid", dir, nil)
	if err := svc.RestoreFromBackup(context.Background(), bk, false); err == nil {
		t.Fatal("expected restore to fail on non-gzip backup")
	}
}

// ── Backup: ListBackups with missing dir, RestoreFromBackup with missing id ──

func TestBackupService_ListAndRestore(t *testing.T) {
	// Missing backup dir → nil result, no error
	svc := NewBackupService("postgres://x", filepath.Join(t.TempDir(), "nonexistent"), nil)
	if items, err := svc.ListBackups(context.Background()); err != nil || items != nil {
		t.Fatalf("ListBackups(missing) = (%v, %v)", items, err)
	}
	// Dir exists but empty → empty list
	dir := t.TempDir()
	svc2 := NewBackupService("postgres://x", dir, nil)
	if items, err := svc2.ListBackups(context.Background()); err != nil || len(items) != 0 {
		t.Fatalf("ListBackups(empty) = (%d, %v)", len(items), err)
	}
	// Place a fake backup file + a non-backup file; expect only the .sql.gz one
	bkPath := filepath.Join(dir, "backup_20260101_010101.sql.gz")
	if err := os.WriteFile(bkPath, []byte{0, 1, 2}, 0600); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("x"), 0600)
	items, err := svc2.ListBackups(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("ListBackups(populated) = (%d, %v)", len(items), err)
	}
	if items[0].ID != "backup_20260101_010101" {
		t.Fatalf("expected ID stripped of .sql.gz, got %s", items[0].ID)
	}
	// Restore missing file
	if err := svc2.RestoreFromBackup(context.Background(), "does-not-exist", false); err == nil {
		t.Fatal("expected missing-backup error")
	}
}

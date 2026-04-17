package service

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

type stubAuditRepo struct {
	entry *domain.AuditLog
	err   error
}

func (s *stubAuditRepo) Create(_ context.Context, log *domain.AuditLog) error {
	s.entry = log
	return s.err
}

func (s *stubAuditRepo) List(context.Context, repository.AuditFilters, domain.PageRequest) ([]domain.AuditLog, int, error) {
	return nil, 0, nil
}

type stubUserRepo struct {
	byName      map[string]*domain.User
	byID        map[uuid.UUID]*domain.User
	created     *domain.User
	updated     *domain.User
	deactivated uuid.UUID
	list        []domain.User
	err         error
}

func (s *stubUserRepo) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	if u, ok := s.byName[username]; ok {
		return u, nil
	}
	return nil, domain.NewNotFoundError("user")
}

func (s *stubUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	if u, ok := s.byID[id]; ok {
		return u, nil
	}
	return nil, domain.NewNotFoundError("user")
}

func (s *stubUserRepo) List(context.Context, domain.UserRole, *bool) ([]domain.User, error) {
	return s.list, nil
}

func (s *stubUserRepo) Create(_ context.Context, u *domain.User) (*domain.User, error) {
	s.created = u
	if s.byID == nil {
		s.byID = map[uuid.UUID]*domain.User{}
	}
	if s.byName == nil {
		s.byName = map[string]*domain.User{}
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	s.byID[u.ID] = u
	s.byName[u.Username] = u
	return u, s.err
}

func (s *stubUserRepo) Update(_ context.Context, u *domain.User) (*domain.User, error) {
	s.updated = u
	s.byID[u.ID] = u
	return u, s.err
}

func (s *stubUserRepo) Deactivate(_ context.Context, id uuid.UUID) error {
	s.deactivated = id
	return s.err
}

func (s *stubUserRepo) UpdatePassword(_ context.Context, id uuid.UUID, hash string, _ bool) error {
	if u, ok := s.byID[id]; ok {
		u.PasswordHash = hash
	}
	return s.err
}

func (s *stubUserRepo) CountByRole(_ context.Context, role domain.UserRole) (int, error) {
	count := 0
	for _, u := range s.byID {
		if u.Role == role && u.IsActive {
			count++
		}
	}
	return count, s.err
}

type stubTierRepo struct {
	decremented int
	incremented int
	err         error
}

func (s *stubTierRepo) List(context.Context, string, bool) ([]domain.RewardTier, error) {
	return nil, nil
}

func (s *stubTierRepo) GetByID(context.Context, uuid.UUID) (*domain.RewardTier, error) {
	return nil, domain.NewNotFoundError("tier")
}

func (s *stubTierRepo) GetByIDForUpdate(context.Context, pgx.Tx, uuid.UUID) (*domain.RewardTier, error) {
	return nil, domain.NewNotFoundError("tier")
}

func (s *stubTierRepo) Create(context.Context, *domain.RewardTier) (*domain.RewardTier, error) {
	return nil, nil
}

func (s *stubTierRepo) Update(context.Context, *domain.RewardTier) (*domain.RewardTier, error) {
	return nil, nil
}

func (s *stubTierRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (s *stubTierRepo) Restore(context.Context, uuid.UUID) error {
	return nil
}

func (s *stubTierRepo) DecrementInventory(context.Context, pgx.Tx, uuid.UUID, int) error {
	s.decremented++
	return s.err
}

func (s *stubTierRepo) IncrementInventory(context.Context, pgx.Tx, uuid.UUID, int) error {
	s.incremented++
	return s.err
}

type stubReservationRepo struct {
	created int
	voided  int
	err     error
}

func (s *stubReservationRepo) Create(context.Context, pgx.Tx, *domain.Reservation) (*domain.Reservation, error) {
	s.created++
	return &domain.Reservation{}, s.err
}

func (s *stubReservationRepo) VoidByFulfillmentID(context.Context, pgx.Tx, uuid.UUID) error {
	s.voided++
	return s.err
}

func (s *stubReservationRepo) GetActiveByFulfillmentID(context.Context, pgx.Tx, uuid.UUID) (*domain.Reservation, error) {
	return nil, nil
}

type stubExceptionRepo struct {
	item    *domain.FulfillmentException
	created *domain.FulfillmentException
	updated domain.ExceptionStatus
	list    []domain.FulfillmentException
	err     error
}

func (s *stubExceptionRepo) List(context.Context, repository.ExceptionFilters) ([]domain.FulfillmentException, error) {
	return s.list, nil
}

func (s *stubExceptionRepo) GetByID(context.Context, uuid.UUID) (*domain.FulfillmentException, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.item, nil
}

func (s *stubExceptionRepo) Create(_ context.Context, e *domain.FulfillmentException) (*domain.FulfillmentException, error) {
	s.created = e
	s.item = e
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return e, s.err
}

func (s *stubExceptionRepo) UpdateStatus(_ context.Context, _ uuid.UUID, status domain.ExceptionStatus, note *string, resolvedBy *uuid.UUID) error {
	s.updated = status
	if s.item != nil {
		s.item.Status = status
		s.item.ResolutionNote = note
		s.item.ResolvedBy = resolvedBy
	}
	return s.err
}

func (s *stubExceptionRepo) ExistsOpenForFulfillment(context.Context, uuid.UUID, domain.ExceptionType) (bool, error) {
	return false, nil
}

type stubExceptionEventRepo struct {
	event *domain.ExceptionEvent
	err   error
}

func (s *stubExceptionEventRepo) Create(_ context.Context, event *domain.ExceptionEvent) error {
	s.event = event
	return s.err
}

func (s *stubExceptionEventRepo) ListByExceptionID(context.Context, uuid.UUID) ([]domain.ExceptionEvent, error) {
	return nil, nil
}

type stubTemplateRepo struct {
	tmpl *domain.MessageTemplate
}

func (s *stubTemplateRepo) List(context.Context, domain.TemplateCategory, domain.SendLogChannel, bool) ([]domain.MessageTemplate, error) {
	return nil, nil
}

func (s *stubTemplateRepo) GetByID(context.Context, uuid.UUID) (*domain.MessageTemplate, error) {
	return s.tmpl, nil
}

func (s *stubTemplateRepo) Create(context.Context, *domain.MessageTemplate) (*domain.MessageTemplate, error) {
	return nil, nil
}

func (s *stubTemplateRepo) Update(context.Context, *domain.MessageTemplate) (*domain.MessageTemplate, error) {
	return nil, nil
}

func (s *stubTemplateRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (s *stubTemplateRepo) Restore(context.Context, uuid.UUID) error {
	return nil
}

type stubSendLogRepo struct {
	created    *domain.SendLog
	printedID  uuid.UUID
	retryables []domain.SendLog
	statuses   []domain.SendLogStatus
	nextRetry  int
}

func (s *stubSendLogRepo) Create(_ context.Context, log *domain.SendLog) (*domain.SendLog, error) {
	s.created = log
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	return log, nil
}

func (s *stubSendLogRepo) UpdateStatus(context.Context, uuid.UUID, domain.SendLogStatus, *string) error {
	return nil
}

func (s *stubSendLogRepo) UpdateNextRetry(context.Context, uuid.UUID, time.Time) error {
	s.nextRetry++
	return nil
}

func (s *stubSendLogRepo) MarkPrinted(_ context.Context, id uuid.UUID, _ uuid.UUID) error {
	s.printedID = id
	return nil
}

func (s *stubSendLogRepo) List(context.Context, repository.SendLogFilters, domain.PageRequest) ([]domain.SendLog, int, error) {
	return nil, 0, nil
}

func (s *stubSendLogRepo) GetRetryable(context.Context, time.Time) ([]domain.SendLog, error) {
	return s.retryables, nil
}

func (s *stubSendLogRepo) GetByID(context.Context, uuid.UUID) (*domain.SendLog, error) {
	return nil, domain.NewNotFoundError("send log")
}

func (s *stubSendLogRepo) ClearNextRetry(context.Context, uuid.UUID) error { return nil }

type stubNotificationRepo struct {
	created []*domain.Notification
}

func (s *stubNotificationRepo) Create(_ context.Context, n *domain.Notification) (*domain.Notification, error) {
	s.created = append(s.created, n)
	return n, nil
}

func (s *stubNotificationRepo) CreateTx(context.Context, pgx.Tx, *domain.Notification) error {
	return nil
}

func (s *stubNotificationRepo) ListByUserID(context.Context, uuid.UUID, *bool, domain.PageRequest) ([]domain.Notification, int, error) {
	return nil, 0, nil
}

func (s *stubNotificationRepo) MarkRead(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

type stubReportRepo struct {
	export *domain.ReportExport
	status []domain.ExportStatus
}

func (s *stubReportRepo) Create(context.Context, *domain.ReportExport) (*domain.ReportExport, error) {
	return nil, nil
}

func (s *stubReportRepo) GetByID(context.Context, uuid.UUID) (*domain.ReportExport, error) {
	if s.export == nil {
		return nil, domain.NewNotFoundError("report export")
	}
	return s.export, nil
}

func (s *stubReportRepo) List(context.Context, repository.ReportExportFilters, domain.PageRequest) ([]domain.ReportExport, int, error) {
	return nil, 0, nil
}

func (s *stubReportRepo) UpdateStatus(_ context.Context, _ uuid.UUID, status domain.ExportStatus, filePath *string, fileSize *int64, checksum *string, expiresAt *time.Time) error {
	s.status = append(s.status, status)
	if s.export != nil {
		s.export.Status = status
		s.export.FilePath = filePath
		s.export.FileSizeBytes = fileSize
		s.export.ChecksumSHA256 = checksum
		s.export.ExpiresAt = expiresAt
	}
	return nil
}

func (s *stubReportRepo) GetExpired(context.Context, time.Time) ([]domain.ReportExport, error) {
	return nil, nil
}

func (s *stubReportRepo) Delete(context.Context, uuid.UUID) error {
	return nil
}

type stubFulfillmentRepo struct {
	listItems []domain.Fulfillment
}

func (s *stubFulfillmentRepo) List(context.Context, repository.FulfillmentFilters, domain.PageRequest) ([]domain.Fulfillment, int, error) {
	return s.listItems, len(s.listItems), nil
}

func (s *stubFulfillmentRepo) GetByID(context.Context, uuid.UUID) (*domain.Fulfillment, error) {
	return nil, domain.NewNotFoundError("fulfillment")
}

func (s *stubFulfillmentRepo) GetByIDForUpdate(context.Context, pgx.Tx, uuid.UUID) (*domain.Fulfillment, error) {
	return nil, domain.NewNotFoundError("fulfillment")
}

func (s *stubFulfillmentRepo) Create(context.Context, pgx.Tx, *domain.Fulfillment) (*domain.Fulfillment, error) {
	return nil, nil
}

func (s *stubFulfillmentRepo) Update(context.Context, pgx.Tx, *domain.Fulfillment) (*domain.Fulfillment, error) {
	return nil, nil
}

func (s *stubFulfillmentRepo) BumpVersion(context.Context, pgx.Tx, uuid.UUID, int) error {
	return nil
}

func (s *stubFulfillmentRepo) CountByCustomerAndTier(context.Context, pgx.Tx, uuid.UUID, uuid.UUID, time.Time) (int, error) {
	return 0, nil
}

func (s *stubFulfillmentRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (s *stubFulfillmentRepo) Restore(context.Context, uuid.UUID) error {
	return nil
}

func (s *stubFulfillmentRepo) ListOverdue(context.Context) ([]domain.Fulfillment, error) {
	return nil, nil
}

type stubCustomerRepo struct {
	listItems []domain.Customer
}

func (s *stubCustomerRepo) List(context.Context, string, domain.PageRequest, bool) ([]domain.Customer, int, error) {
	return s.listItems, len(s.listItems), nil
}

func (s *stubCustomerRepo) GetByID(context.Context, uuid.UUID) (*domain.Customer, error) {
	return nil, domain.NewNotFoundError("customer")
}

func (s *stubCustomerRepo) Create(context.Context, *domain.Customer) (*domain.Customer, error) {
	return nil, nil
}

func (s *stubCustomerRepo) Update(context.Context, *domain.Customer) (*domain.Customer, error) {
	return nil, nil
}

func (s *stubCustomerRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (s *stubCustomerRepo) Restore(context.Context, uuid.UUID) error {
	return nil
}

func newTestEncryptionService(t *testing.T) EncryptionService {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "key.txt")
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(raw)), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	svc, err := NewEncryptionService(keyPath)
	if err != nil {
		t.Fatalf("NewEncryptionService() error = %v", err)
	}
	return svc
}

func TestAuditServiceLogIncludesContext(t *testing.T) {
	repo := &stubAuditRepo{}
	svc := NewAuditService(repo)

	recordID := uuid.New()
	actorID := uuid.New()
	ctx := WithRequestID(WithIPAddress(WithUserID(context.Background(), actorID), "127.0.0.1"), "req-1")
	err := svc.Log(ctx, "users", recordID, "UPDATE", map[string]string{"before": "a"}, map[string]string{"after": "b"})
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}
	if repo.entry == nil || repo.entry.PerformedBy == nil || *repo.entry.PerformedBy != actorID {
		t.Fatal("expected performed by to be recorded")
	}
	if repo.entry.IPAddress == nil || *repo.entry.IPAddress != "127.0.0.1" {
		t.Fatal("expected IP address to be recorded")
	}
	if repo.entry.RequestID == nil || *repo.entry.RequestID != "req-1" {
		t.Fatal("expected request id to be recorded")
	}
	if len(repo.entry.BeforeState) == 0 || len(repo.entry.AfterState) == 0 {
		t.Fatal("expected before and after state JSON")
	}

	if _, ok := UserIDFromContext(ctx); !ok || IPFromContext(ctx) != "127.0.0.1" || RequestIDFromContext(ctx) != "req-1" {
		t.Fatal("context helpers did not round-trip values")
	}
}

func TestUserServiceAndInventoryService(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	existing := &domain.User{
		ID:           uuid.New(),
		Username:     "alex",
		PasswordHash: string(hash),
		Role:         domain.RoleAdministrator,
		IsActive:     true,
	}
	userRepo := &stubUserRepo{
		byName: map[string]*domain.User{"alex": existing},
		byID:   map[uuid.UUID]*domain.User{existing.ID: existing},
		list:   []domain.User{*existing},
	}
	auditRepo := &stubAuditRepo{}
	userSvc := NewUserService(userRepo, NewAuditService(auditRepo))

	if _, err := userSvc.Authenticate(context.Background(), "alex", "Password123"); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if _, err := userSvc.Authenticate(context.Background(), "alex", "wrong"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for bad password, got %v", err)
	}
	if _, err := userSvc.CreateUser(context.Background(), "new", "new@example.com", "Password123", domain.RoleAuditor); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if userRepo.created == nil || userRepo.created.PasswordHash == "Password123" {
		t.Fatal("expected created user password to be hashed")
	}

	if _, err := userSvc.UpdateUser(context.Background(), existing.ID, "updated@example.com", domain.RoleAuditor); err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if err := userSvc.DeactivateUser(context.Background(), existing.ID); err != nil {
		t.Fatalf("DeactivateUser() error = %v", err)
	}
	if got, err := userSvc.List(context.Background(), "", nil); err != nil || len(got) != 1 {
		t.Fatalf("List() = (%d, %v)", len(got), err)
	}

	tierRepo := &stubTierRepo{}
	resRepo := &stubReservationRepo{}
	inventorySvc := NewInventoryService(tierRepo, resRepo)
	if err := inventorySvc.Reserve(context.Background(), nil, uuid.New(), uuid.New()); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := inventorySvc.Release(context.Background(), nil, uuid.New(), uuid.New()); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if tierRepo.decremented != 1 || tierRepo.incremented != 1 || resRepo.created != 1 || resRepo.voided != 1 {
		t.Fatalf("unexpected inventory calls: %+v %+v", tierRepo, resRepo)
	}
}

func TestExceptionAndMessagingServices(t *testing.T) {
	exRepo := &stubExceptionRepo{}
	exEventRepo := &stubExceptionEventRepo{}
	auditRepo := &stubAuditRepo{}
	svc := NewExceptionService(exRepo, exEventRepo, NewAuditService(auditRepo))

	actorID := uuid.New()
	ctx := WithUserID(context.Background(), actorID)
	created, err := svc.Create(ctx, uuid.New(), domain.ExceptionManual, "opened")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.OpenedBy == nil || *created.OpenedBy != actorID {
		t.Fatal("expected OpenedBy to be populated")
	}
	updated, err := svc.UpdateStatus(ctx, created.ID, domain.ExceptionResolved, "fixed")
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if updated.ResolvedBy == nil || *updated.ResolvedBy != actorID {
		t.Fatal("expected ResolvedBy to be populated")
	}
	if _, err := svc.AddEvent(ctx, created.ID, "NOTE", "hello"); err != nil {
		t.Fatalf("AddEvent() error = %v", err)
	}
	if exEventRepo.event == nil || exEventRepo.event.CreatedBy == nil {
		t.Fatal("expected event creator to be set")
	}
	if _, err := svc.GetByID(context.Background(), created.ID); err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if _, err := svc.List(context.Background(), repository.ExceptionFilters{}); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	templateRepo := &stubTemplateRepo{tmpl: &domain.MessageTemplate{
		ID:           uuid.New(),
		Name:         "Welcome",
		Channel:      domain.ChannelInApp,
		BodyTemplate: "Hello",
	}}
	sendLogRepo := &stubSendLogRepo{}
	notifRepo := &stubNotificationRepo{}
	messagingSvc := NewMessagingService(templateRepo, sendLogRepo, notifRepo, nil)

	if _, err := messagingSvc.Dispatch(context.Background(), templateRepo.tmpl.ID, actorID, nil, map[string]any{"x": 1}); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if sendLogRepo.created == nil || sendLogRepo.created.Status != domain.SendSent {
		t.Fatal("expected in-app dispatch to create a sent log")
	}

	templateRepo.tmpl.Channel = domain.ChannelEmail
	if _, err := messagingSvc.Dispatch(context.Background(), templateRepo.tmpl.ID, actorID, nil, map[string]any{"x": 1}); err != nil {
		t.Fatalf("Dispatch(email) error = %v", err)
	}
	if sendLogRepo.created.NextRetryAt == nil {
		t.Fatal("expected offline channel dispatch to schedule retry")
	}

	ctx = WithUserID(context.Background(), actorID)
	printID := uuid.New()
	if err := messagingSvc.MarkPrinted(ctx, printID); err != nil {
		t.Fatalf("MarkPrinted() error = %v", err)
	}
	if sendLogRepo.printedID != printID {
		t.Fatal("expected printed log ID to be forwarded")
	}
}

func TestBackupAndExportServices(t *testing.T) {
	backupDir := t.TempDir()
	backupSvc := NewBackupService("postgres://example", backupDir, "", nil)
	if backups, err := backupSvc.ListBackups(context.Background()); err != nil || len(backups) != 0 {
		t.Fatalf("ListBackups() = (%d, %v), want empty nil", len(backups), err)
	}
	if err := backupSvc.RestoreFromBackup(context.Background(), "missing", true); err == nil {
		t.Fatal("expected restore to fail for missing backup")
	}

	encSvc := newTestEncryptionService(t)
	voucherCipher, err := encSvc.EncryptString("SECRET-VOUCHER")
	if err != nil {
		t.Fatalf("EncryptString() error = %v", err)
	}
	customerPhone, err := encSvc.EncryptString("5551234567")
	if err != nil {
		t.Fatalf("EncryptString() phone error = %v", err)
	}
	customerEmail, err := encSvc.EncryptString("person@example.com")
	if err != nil {
		t.Fatalf("EncryptString() email error = %v", err)
	}

	tmp := t.TempDir()
	report := &domain.ReportExport{
		ID:               uuid.New(),
		ReportType:       "fulfillments",
		Filters:          []byte(`{}`),
		Status:           domain.ExportQueued,
		IncludeSensitive: true,
	}
	reportRepo := &stubReportRepo{export: report}
	exportSvc := NewExportService(
		reportRepo,
		&stubFulfillmentRepo{listItems: []domain.Fulfillment{{
			ID: uuid.New(), TierID: uuid.New(), CustomerID: uuid.New(), Type: domain.TypeVoucher,
			Status: domain.StatusVoucherIssued, VoucherCodeEncrypted: voucherCipher, CreatedAt: time.Now().UTC(),
		}}},
		&stubCustomerRepo{listItems: []domain.Customer{{
			ID: uuid.New(), Name: "Pat", PhoneEncrypted: customerPhone, EmailEncrypted: customerEmail, CreatedAt: time.Now().UTC(),
		}}},
		&stubAuditRepo{},
		encSvc,
		tmp,
		nil,
	)
	if err := exportSvc.GenerateExport(context.Background(), report.ID); err != nil {
		t.Fatalf("GenerateExport() error = %v", err)
	}
	if len(reportRepo.status) < 2 || report.Status != domain.ExportCompleted || report.FilePath == nil {
		t.Fatalf("expected export to complete, statuses=%v file=%v", reportRepo.status, report.FilePath)
	}
	ok, err := exportSvc.VerifyChecksum(context.Background(), report.ID)
	if err != nil || !ok {
		t.Fatalf("VerifyChecksum() = (%v, %v)", ok, err)
	}
}

func TestPageRequestAndSLAHelpers(t *testing.T) {
	req := domain.PageRequest{Page: 0, PageSize: 1000}
	req.Normalize()
	if req.Page != 1 || req.PageSize != 20 || req.Offset() != 0 {
		t.Fatalf("normalized request = %+v", req)
	}

	settingRepo := &stubSettingRepo{values: map[string][]byte{
		"business_hours_start": []byte(`"08:00"`),
		"business_hours_end":   []byte(`"18:00"`),
		"business_days":        []byte(`[1,2,3,4,5]`),
		"timezone":             []byte(`"America/New_York"`),
	}}
	blackoutRepo := &stubBlackoutRepo{}
	slaSvc := NewSLAService(settingRepo, blackoutRepo)
	readyAt := time.Date(2026, 4, 17, 17, 0, 0, 0, time.FixedZone("EDT", -4*3600))
	deadline, err := slaSvc.CalculateDeadline(context.Background(), domain.TypeVoucher, readyAt)
	if err != nil {
		t.Fatalf("CalculateDeadline() error = %v", err)
	}
	if deadline.IsZero() || !slaSvc.IsOverdue(time.Now().UTC().Add(-time.Minute)) {
		t.Fatal("expected SLA helpers to return a deadline and overdue status")
	}
}

type stubSettingRepo struct {
	values map[string][]byte
}

func (s *stubSettingRepo) Get(_ context.Context, key string) (*domain.SystemSetting, error) {
	if v, ok := s.values[key]; ok {
		return &domain.SystemSetting{Key: key, Value: v}, nil
	}
	return nil, domain.ErrNotFound
}

func (s *stubSettingRepo) Set(context.Context, string, []byte, *uuid.UUID) error {
	return nil
}

func (s *stubSettingRepo) GetAll(context.Context) ([]domain.SystemSetting, error) {
	return nil, nil
}

type stubBlackoutRepo struct{}

func (s *stubBlackoutRepo) List(context.Context) ([]domain.BlackoutDate, error) {
	return nil, nil
}

func (s *stubBlackoutRepo) Create(context.Context, *domain.BlackoutDate) (*domain.BlackoutDate, error) {
	return nil, nil
}

func (s *stubBlackoutRepo) Delete(context.Context, uuid.UUID) error {
	return nil
}

func (s *stubBlackoutRepo) GetBetween(context.Context, time.Time, time.Time) ([]domain.BlackoutDate, error) {
	return nil, nil
}

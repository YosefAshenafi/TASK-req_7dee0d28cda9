package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// helper: make a reward tier + customer + fulfillment quickly for dependent-row tests.
func seedFulfillment(t *testing.T) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tierRepo := repository.NewTierRepository(testPool)
	custRepo := repository.NewCustomerRepository(testPool)
	tier, err := tierRepo.Create(ctx, &domain.RewardTier{
		Name: "extra " + uuid.New().String()[:8], InventoryCount: 5, PurchaseLimit: 3, AlertThreshold: 1,
	})
	if err != nil {
		t.Fatalf("tier create: %v", err)
	}
	cust, err := custRepo.Create(ctx, &domain.Customer{Name: "extra " + uuid.New().String()[:8]})
	if err != nil {
		t.Fatalf("customer create: %v", err)
	}
	var fID uuid.UUID
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)
	ff, err := repository.NewFulfillmentRepository(testPool).Create(ctx, tx, &domain.Fulfillment{
		TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical, Status: domain.StatusDraft,
	})
	if err != nil {
		t.Fatalf("ff create: %v", err)
	}
	fID = ff.ID
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return tier.ID, cust.ID, fID
}

func TestSendLogRepository_EndToEnd(t *testing.T) {
	ctx := context.Background()
	tmplRepo := repository.NewMessageTemplateRepository(testPool)
	tmpl, err := tmplRepo.Create(ctx, &domain.MessageTemplate{
		Name:         "sl " + uuid.New().String()[:8],
		Category:     domain.CategoryFulfillmentProgress,
		Channel:      domain.ChannelSMS,
		BodyTemplate: "x",
	})
	if err != nil {
		t.Fatalf("tmpl create: %v", err)
	}
	repo := repository.NewSendLogRepository(testPool)

	custRepo := repository.NewCustomerRepository(testPool)
	cust, err := custRepo.Create(ctx, &domain.Customer{Name: "sl cust " + uuid.New().String()[:8]})
	if err != nil {
		t.Fatalf("customer create: %v", err)
	}
	recipient := cust.ID
	tmplID := tmpl.ID
	log, err := repo.Create(ctx, &domain.SendLog{
		TemplateID:  &tmplID,
		RecipientID: recipient,
		Channel:     domain.ChannelSMS,
		Status:      domain.SendQueued,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdateStatus(ctx, log.ID, domain.SendFailed, nil); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	next := time.Now().UTC().Add(time.Hour)
	if err := repo.UpdateNextRetry(ctx, log.ID, next); err != nil {
		t.Fatalf("UpdateNextRetry: %v", err)
	}

	// Re-queue for MarkPrinted
	if err := repo.UpdateStatus(ctx, log.ID, domain.SendQueued, nil); err != nil {
		t.Fatalf("UpdateStatus(queued): %v", err)
	}
	actor := seedAdminID(t)
	if err := repo.MarkPrinted(ctx, log.ID, actor); err != nil {
		t.Fatalf("MarkPrinted: %v", err)
	}
	// MarkPrinted on missing ID → NotFound (already printed, won't match status='QUEUED').
	if err := repo.MarkPrinted(ctx, log.ID, actor); err == nil {
		t.Fatal("expected NotFound for already-printed log")
	}

	// GetRetryable returns failed-with-retry logs.
	if _, err := repo.GetRetryable(ctx, time.Now().UTC()); err != nil {
		t.Fatalf("GetRetryable: %v", err)
	}
	// List with filters.
	status := domain.SendPrinted
	if _, _, err := repo.List(ctx, repository.SendLogFilters{
		RecipientID: &recipient, Channel: domain.ChannelSMS, Status: status,
		DateFrom: &log.CreatedAt, DateTo: &log.CreatedAt,
	}, domain.PageRequest{Page: 1, PageSize: 10}); err != nil {
		t.Fatalf("List: %v", err)
	}
}

func TestShippingAddressRepository_EndToEnd(t *testing.T) {
	ctx := context.Background()
	_, _, fID := seedFulfillment(t)
	repo := repository.NewShippingAddressRepository(testPool)

	addr := &domain.ShippingAddress{
		FulfillmentID:  fID,
		Line1Encrypted: []byte{1, 2, 3},
		City:           "Townville",
		State:          "CA",
		ZipCode:        "94105",
	}
	if _, err := repo.CreateNoTx(ctx, addr); err != nil {
		t.Fatalf("CreateNoTx: %v", err)
	}
	got, err := repo.GetByFulfillmentID(ctx, fID)
	if err != nil {
		t.Fatalf("GetByFulfillmentID: %v", err)
	}
	if got == nil || got.City != "Townville" {
		t.Fatalf("unexpected GetByFulfillmentID result: %+v", got)
	}
	// Update via tx.
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	got.City = "NewTown"
	if err := repo.Update(ctx, tx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	// Missing address returns nil, nil.
	missing, err := repo.GetByFulfillmentID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("GetByFulfillmentID(missing): %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for missing shipping address")
	}
}

func TestMessageTemplateRepository_EndToEnd(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMessageTemplateRepository(testPool)

	tmpl, err := repo.Create(ctx, &domain.MessageTemplate{
		Name:         "Extra " + uuid.New().String()[:8],
		Category:     domain.CategoryFulfillmentProgress,
		Channel:      domain.ChannelEmail,
		BodyTemplate: "hi",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	tmpl.BodyTemplate = "hi updated"
	if _, err := repo.Update(ctx, tmpl); err != nil {
		t.Fatalf("Update: %v", err)
	}

	actor := seedAdminID(t)
	if err := repo.SoftDelete(ctx, tmpl.ID, actor); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if err := repo.Restore(ctx, tmpl.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	// List with filter.
	if _, err := repo.List(ctx, domain.CategoryFulfillmentProgress, domain.ChannelEmail, false); err != nil {
		t.Fatalf("List: %v", err)
	}
}

func TestSystemSettingRepository_EndToEnd(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewSystemSettingRepository(testPool)
	actor := seedAdminID(t)
	key := "extra_cov_" + uuid.New().String()[:8]

	if err := repo.Set(ctx, key, []byte(`"hello"`), &actor); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || string(got.Value) != `"hello"` {
		t.Fatalf("unexpected Get: %+v", got)
	}
	// Upsert update
	if err := repo.Set(ctx, key, []byte(`"world"`), &actor); err != nil {
		t.Fatalf("Set(update): %v", err)
	}
	if _, err := repo.GetAll(ctx); err != nil {
		t.Fatalf("GetAll: %v", err)
	}
}

func TestBlackoutDateRepository_EndToEnd(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewBlackoutDateRepository(testPool)

	desc := "extra coverage"
	d, err := repo.Create(ctx, &domain.BlackoutDate{
		Date:        time.Date(2030, 12, 25, 0, 0, 0, 0, time.UTC),
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := repo.List(ctx); err != nil {
		t.Fatalf("List: %v", err)
	}
	if _, err := repo.GetBetween(ctx, time.Date(2030, 12, 1, 0, 0, 0, 0, time.UTC), time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("GetBetween: %v", err)
	}
	if err := repo.Delete(ctx, d.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestJobRunRepository_EndToEnd(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewJobRunRepository(testPool)

	run, err := repo.Create(ctx, &domain.JobRunHistory{JobName: "extra_cov", Status: domain.JobRunning, StartedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	errStr := "test msg"
	if err := repo.Finish(ctx, run.ID, domain.JobFailed, 0, &errStr); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	// Finish completed state too.
	run2, _ := repo.Create(ctx, &domain.JobRunHistory{JobName: "extra_cov_ok", Status: domain.JobRunning, StartedAt: time.Now().UTC()})
	if err := repo.Finish(ctx, run2.ID, domain.JobCompleted, 5, nil); err != nil {
		t.Fatalf("Finish ok: %v", err)
	}
	if _, _, err := repo.List(ctx, repository.JobRunFilters{JobName: "extra_cov"}, domain.PageRequest{Page: 1, PageSize: 10}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if _, _, err := repo.List(ctx, repository.JobRunFilters{}, domain.PageRequest{Page: 1, PageSize: 10}); err != nil {
		t.Fatalf("List(all): %v", err)
	}
}

func TestNotificationRepository_EndToEnd(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewNotificationRepository(testPool)
	actor := seedAdminID(t)

	body := "body"
	n, err := repo.Create(ctx, &domain.Notification{UserID: actor, Title: "hi", Body: &body})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// CreateTx in a transaction.
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := repo.CreateTx(ctx, tx, &domain.Notification{UserID: actor, Title: "tx", Body: &body}); err != nil {
		t.Fatalf("CreateTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	f := false
	if _, _, err := repo.ListByUserID(ctx, actor, &f, domain.PageRequest{Page: 1, PageSize: 10}); err != nil {
		t.Fatalf("ListByUserID(unread): %v", err)
	}
	if _, _, err := repo.ListByUserID(ctx, actor, nil, domain.PageRequest{Page: 1, PageSize: 10}); err != nil {
		t.Fatalf("ListByUserID(all): %v", err)
	}
	if err := repo.MarkRead(ctx, n.ID, actor); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
}

func TestReportExportRepository_EndToEnd(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewReportExportRepository(testPool)

	past := time.Now().UTC().Add(-time.Hour)
	rep, err := repo.Create(ctx, &domain.ReportExport{
		ReportType: "fulfillments",
		Filters:    []byte(`{}`),
		Status:     domain.ExportCompleted,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	expired, err := repo.Create(ctx, &domain.ReportExport{
		ReportType: "fulfillments",
		Filters:    []byte(`{}`),
		Status:     domain.ExportCompleted,
		ExpiresAt:  &past,
	})
	if err != nil {
		t.Fatalf("Create expired: %v", err)
	}
	fp := "/tmp/whatever.csv"
	size := int64(42)
	cs := "abc"
	later := time.Now().UTC().Add(24 * time.Hour)
	if err := repo.UpdateStatus(ctx, rep.ID, domain.ExportCompleted, &fp, &size, &cs, &later); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	if _, err := repo.GetExpired(ctx, time.Now().UTC()); err != nil {
		t.Fatalf("GetExpired: %v", err)
	}
	if err := repo.Delete(ctx, expired.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestUserRepository_DeactivateAndListByRole(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepository(testPool)
	suffix := uuid.New().String()[:8]
	u, err := repo.Create(ctx, &domain.User{
		Username:     "extra_cov_" + suffix,
		Email:        "extra_cov_" + suffix + "@example.com",
		PasswordHash: "x",
		Role:         domain.RoleAuditor,
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Deactivate(ctx, u.ID); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if _, err := repo.List(ctx, domain.RoleAuditor, nil); err != nil {
		t.Fatalf("List(all auditors): %v", err)
	}
	inactive := false
	if _, err := repo.List(ctx, "", &inactive); err != nil {
		t.Fatalf("List(inactive): %v", err)
	}
}

func TestConnection_NewDB_BadURL(t *testing.T) {
	// Bad URL returns parsing error.
	if _, err := repository.NewDB(context.Background(), "not-a-valid-url://"); err == nil {
		t.Fatal("expected NewDB to fail on bad URL")
	}
	// Unreachable host returns ping error.
	if _, err := repository.NewDB(context.Background(), "postgres://user:pass@127.0.0.1:9/db?sslmode=disable&connect_timeout=1"); err == nil {
		t.Fatal("expected NewDB to fail on unreachable host")
	}
}

func TestReservationRepository_Lifecycle(t *testing.T) {
	ctx := context.Background()
	tierID, _, fID := seedFulfillment(t)
	repo := repository.NewReservationRepository()

	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	res, err := repo.Create(ctx, tx, &domain.Reservation{
		TierID: tierID, FulfillmentID: fID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.GetActiveByFulfillmentID(ctx, tx, fID)
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if got == nil || got.ID != res.ID {
		t.Fatal("unexpected GetActive result")
	}
	if err := repo.VoidByFulfillmentID(ctx, tx, fID); err != nil {
		t.Fatalf("Void: %v", err)
	}
	got2, _ := repo.GetActiveByFulfillmentID(ctx, tx, fID)
	if got2 != nil {
		t.Fatal("expected no active reservation after Void")
	}
}

func TestAuditRepository_ListWithFilters(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewAuditRepository(testPool)
	actor := seedAdminID(t)

	rec := uuid.New()
	ip := "10.0.0.1"
	req := "req-" + uuid.New().String()[:8]
	if err := repo.Create(ctx, &domain.AuditLog{
		TableName:   "cov_test",
		RecordID:    &rec,
		Operation:   "UPDATE",
		PerformedBy: &actor,
		IPAddress:   &ip,
		RequestID:   &req,
		BeforeState: []byte(`{"a":1}`),
		AfterState:  []byte(`{"a":2}`),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	from := time.Now().UTC().Add(-time.Hour)
	to := time.Now().UTC().Add(time.Hour)
	if _, _, err := repo.List(ctx, repository.AuditFilters{
		TableName: "cov_test", RecordID: &rec, Operation: "UPDATE",
		PerformedBy: &actor, DateFrom: &from, DateTo: &to,
	}, domain.PageRequest{Page: 1, PageSize: 10}); err != nil {
		t.Fatalf("List(filters): %v", err)
	}
	if _, _, err := repo.List(ctx, repository.AuditFilters{}, domain.PageRequest{Page: 1, PageSize: 10}); err != nil {
		t.Fatalf("List(all): %v", err)
	}
}

func TestTierRepository_RestoreAndList(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewTierRepository(testPool)
	actor := seedAdminID(t)

	tier, err := repo.Create(ctx, &domain.RewardTier{
		Name: "restore tier " + uuid.New().String()[:8], InventoryCount: 1, PurchaseLimit: 1, AlertThreshold: 1,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.SoftDelete(ctx, tier.ID, actor); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if err := repo.Restore(ctx, tier.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	tier.Name = "restore tier updated"
	if _, err := repo.Update(ctx, tier); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := repo.List(ctx, "restore", false); err != nil {
		t.Fatalf("List(search): %v", err)
	}
	if _, err := repo.List(ctx, "", true); err != nil {
		t.Fatalf("List(include deleted): %v", err)
	}
}

func TestCustomerRepository_RestoreAndSearch(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewCustomerRepository(testPool)
	actor := seedAdminID(t)

	c, err := repo.Create(ctx, &domain.Customer{Name: "search " + uuid.New().String()[:8]})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.SoftDelete(ctx, c.ID, actor); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	// SoftDelete again → NotFound.
	if err := repo.SoftDelete(ctx, c.ID, actor); err == nil {
		t.Fatal("expected NotFound on double SoftDelete")
	}
	if err := repo.Restore(ctx, c.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	// Restore again → ErrSoftDeleteExpired.
	if err := repo.Restore(ctx, c.ID); err == nil {
		t.Fatal("expected error restoring non-deleted record")
	}
	c.Name = "search updated"
	if _, err := repo.Update(ctx, c); err != nil {
		t.Fatalf("Update: %v", err)
	}
	// Version conflict on Update.
	c.Version = 999
	if _, err := repo.Update(ctx, c); err == nil {
		t.Fatal("expected Conflict on stale version")
	}
	if _, _, err := repo.List(ctx, "search", domain.PageRequest{Page: 1, PageSize: 10}, false); err != nil {
		t.Fatalf("List(search): %v", err)
	}
	if _, _, err := repo.List(ctx, "", domain.PageRequest{Page: 1, PageSize: 10}, true); err != nil {
		t.Fatalf("List(include deleted): %v", err)
	}
}

func TestFulfillmentRepository_ListFiltersAndSoftDelete(t *testing.T) {
	ctx := context.Background()
	tierID, custID, fID := seedFulfillment(t)
	fulfillRepo := repository.NewFulfillmentRepository(testPool)

	from := time.Now().UTC().Add(-time.Hour)
	to := time.Now().UTC().Add(time.Hour)
	if _, _, err := fulfillRepo.List(ctx, repository.FulfillmentFilters{
		Status: domain.StatusDraft, TierID: &tierID, CustomerID: &custID,
		Type: domain.TypePhysical, DateFrom: &from, DateTo: &to,
	}, domain.PageRequest{Page: 1, PageSize: 10}); err != nil {
		t.Fatalf("List(filters): %v", err)
	}

	actor := seedAdminID(t)
	if err := fulfillRepo.SoftDelete(ctx, fID, actor); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	// SoftDelete again returns NotFound.
	if err := fulfillRepo.SoftDelete(ctx, fID, actor); err == nil {
		t.Fatal("expected NotFound on double soft-delete")
	}
	if err := fulfillRepo.Restore(ctx, fID); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	// Restore again → soft-delete window expired.
	if err := fulfillRepo.Restore(ctx, fID); err == nil {
		t.Fatal("expected error restoring non-deleted record")
	}

	if _, err := fulfillRepo.ListOverdue(ctx); err != nil {
		t.Fatalf("ListOverdue: %v", err)
	}
}

func TestExceptionRepository_ListFiltersAndLifecycle(t *testing.T) {
	ctx := context.Background()
	_, _, fID := seedFulfillment(t)
	repo := repository.NewExceptionRepository(testPool)
	actor := seedAdminID(t)

	note := "opened"
	created, err := repo.Create(ctx, &domain.FulfillmentException{
		FulfillmentID: fID,
		Type:          domain.ExceptionManual,
		Status:        domain.ExceptionOpen,
		ResolutionNote: &note,
		OpenedBy:      &actor,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	from := time.Now().UTC().Add(-time.Hour)
	to := time.Now().UTC().Add(time.Hour)
	if _, err := repo.List(ctx, repository.ExceptionFilters{
		Status: domain.ExceptionOpen, Type: domain.ExceptionManual,
		FulfillmentID: &fID, OpenedFrom: &from, OpenedTo: &to,
	}); err != nil {
		t.Fatalf("List(filters): %v", err)
	}
	resNote := "fixed"
	if err := repo.UpdateStatus(ctx, created.ID, domain.ExceptionResolved, &resNote, &actor); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	exists, err := repo.ExistsOpenForFulfillment(ctx, fID, domain.ExceptionManual)
	if err != nil {
		t.Fatalf("ExistsOpen: %v", err)
	}
	if exists {
		t.Fatal("expected no open exception after resolve")
	}
}

func TestShippingAddressRepository_TxCreateAndUpdate(t *testing.T) {
	ctx := context.Background()
	_, _, fID := seedFulfillment(t)
	repo := repository.NewShippingAddressRepository(testPool)

	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := repo.Create(ctx, tx, &domain.ShippingAddress{
		FulfillmentID: fID, Line1Encrypted: []byte{1}, City: "C", State: "CA", ZipCode: "94105",
	}); err != nil {
		t.Fatalf("Create(tx): %v", err)
	}
	if err := repo.Update(ctx, tx, &domain.ShippingAddress{
		FulfillmentID: fID, Line1Encrypted: []byte{2}, City: "D", State: "NY", ZipCode: "10001",
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestUserRepository_ListAllBranches(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepository(testPool)
	// No filter
	if _, err := repo.List(ctx, "", nil); err != nil {
		t.Fatalf("List(no filter): %v", err)
	}
	active := true
	if _, err := repo.List(ctx, "", &active); err != nil {
		t.Fatalf("List(active): %v", err)
	}
	if _, err := repo.List(ctx, domain.RoleAdministrator, nil); err != nil {
		t.Fatalf("List(role): %v", err)
	}
	if _, err := repo.List(ctx, domain.RoleAdministrator, &active); err != nil {
		t.Fatalf("List(role+active): %v", err)
	}
}

func TestJobRunRepository_ListAllFilters(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewJobRunRepository(testPool)
	from := time.Now().UTC().Add(-time.Hour)
	to := time.Now().UTC().Add(time.Hour)
	status := domain.JobCompleted
	if _, _, err := repo.List(ctx, repository.JobRunFilters{
		JobName: "extra_cov_ok", Status: status, StartedFrom: &from, StartedTo: &to,
	}, domain.PageRequest{Page: 1, PageSize: 10}); err != nil {
		t.Fatalf("List(filters): %v", err)
	}
}

func TestTxManager_Rollback(t *testing.T) {
	ctx := context.Background()
	txMgr := repository.NewTxManager(testPool)
	// Intentional error inside WithTx triggers rollback.
	err := txMgr.WithTx(ctx, func(tx pgx.Tx) error { return errInternal })
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestTxManager_PanicRecovery(t *testing.T) {
	ctx := context.Background()
	txMgr := repository.NewTxManager(testPool)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to be re-thrown after rollback")
		}
	}()
	_ = txMgr.WithTx(ctx, func(tx pgx.Tx) error {
		panic("boom")
	})
}

func TestTxManager_BeginError(t *testing.T) {
	// Build a throwaway pool then close it → Begin fails.
	ctx := context.Background()
	pool, err := repository.NewDB(ctx, "postgres://fulfillops:fulfillops_dev@db:5432/fulfillops?sslmode=disable")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	pool.Close()
	txMgr := repository.NewTxManager(pool)
	if err := txMgr.WithTx(ctx, func(tx pgx.Tx) error { return nil }); err == nil {
		t.Fatal("expected begin error on closed pool")
	}
}

var errInternal = fmt.Errorf("test rollback")

// TestClosedPool_ErrorPaths exercises the "fmt.Errorf(...): %w" SQL-error wrappers
// by using a pool that has been closed before the calls land — every Exec/Query
// then fails with a pool-closed error, hitting the otherwise-unreachable branches.
func TestClosedPool_ErrorPaths(t *testing.T) {
	ctx := context.Background()
	pool, err := repository.NewDB(ctx, "postgres://fulfillops:fulfillops_dev@db:5432/fulfillops?sslmode=disable")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	pool.Close()

	actor := uuid.New()
	id := uuid.New()

	// customer
	custRepo := repository.NewCustomerRepository(pool)
	_, _, _ = custRepo.List(ctx, "", domain.PageRequest{Page: 1, PageSize: 10}, false)
	_, _ = custRepo.GetByID(ctx, id)
	_, _ = custRepo.Create(ctx, &domain.Customer{Name: "x"})
	_, _ = custRepo.Update(ctx, &domain.Customer{ID: id, Name: "x", Version: 1})
	_ = custRepo.SoftDelete(ctx, id, actor)
	_ = custRepo.Restore(ctx, id)

	// tier
	tierRepo := repository.NewTierRepository(pool)
	_, _ = tierRepo.List(ctx, "", false)
	_, _ = tierRepo.GetByID(ctx, id)
	_, _ = tierRepo.Create(ctx, &domain.RewardTier{Name: "x"})
	_, _ = tierRepo.Update(ctx, &domain.RewardTier{ID: id, Name: "x", Version: 1})
	_ = tierRepo.SoftDelete(ctx, id, actor)
	_ = tierRepo.Restore(ctx, id)

	// fulfillment
	ffRepo := repository.NewFulfillmentRepository(pool)
	_, _, _ = ffRepo.List(ctx, repository.FulfillmentFilters{}, domain.PageRequest{Page: 1, PageSize: 10})
	_, _ = ffRepo.GetByID(ctx, id)
	_ = ffRepo.SoftDelete(ctx, id, actor)
	_ = ffRepo.Restore(ctx, id)
	_, _ = ffRepo.ListOverdue(ctx)

	// audit
	auditRepo := repository.NewAuditRepository(pool)
	_ = auditRepo.Create(ctx, &domain.AuditLog{TableName: "t", Operation: "CREATE"})
	_, _, _ = auditRepo.List(ctx, repository.AuditFilters{}, domain.PageRequest{Page: 1, PageSize: 10})

	// exception
	exRepo := repository.NewExceptionRepository(pool)
	_, _ = exRepo.List(ctx, repository.ExceptionFilters{})
	_, _ = exRepo.GetByID(ctx, id)
	_, _ = exRepo.Create(ctx, &domain.FulfillmentException{FulfillmentID: id, Type: domain.ExceptionManual, Status: domain.ExceptionOpen})
	_ = exRepo.UpdateStatus(ctx, id, domain.ExceptionResolved, nil, nil)
	_, _ = exRepo.ExistsOpenForFulfillment(ctx, id, domain.ExceptionManual)

	// exception_event
	exEvRepo := repository.NewExceptionEventRepository(pool)
	_ = exEvRepo.Create(ctx, &domain.ExceptionEvent{ExceptionID: id, EventType: "NOTE", Content: "x"})

	// blackout
	bdRepo := repository.NewBlackoutDateRepository(pool)
	_, _ = bdRepo.Create(ctx, &domain.BlackoutDate{Date: time.Now().UTC()})
	_, _ = bdRepo.List(ctx)
	_, _ = bdRepo.GetBetween(ctx, time.Now().UTC(), time.Now().UTC().Add(time.Hour))
	_ = bdRepo.Delete(ctx, id)

	// notification
	notifRepo := repository.NewNotificationRepository(pool)
	_, _ = notifRepo.Create(ctx, &domain.Notification{UserID: actor, Title: "x"})
	_, _, _ = notifRepo.ListByUserID(ctx, actor, nil, domain.PageRequest{Page: 1, PageSize: 10})
	_ = notifRepo.MarkRead(ctx, id, actor)

	// message_template
	mtRepo := repository.NewMessageTemplateRepository(pool)
	_, _ = mtRepo.Create(ctx, &domain.MessageTemplate{Name: "x", Category: domain.CategoryFulfillmentProgress, Channel: domain.ChannelEmail, BodyTemplate: "x"})
	_, _ = mtRepo.Update(ctx, &domain.MessageTemplate{ID: id, Name: "x", BodyTemplate: "x", Category: domain.CategoryFulfillmentProgress, Channel: domain.ChannelEmail, Version: 1})
	_ = mtRepo.SoftDelete(ctx, id, actor)
	_ = mtRepo.Restore(ctx, id)
	_, _ = mtRepo.List(ctx, "", "", false)

	// send_log
	slRepo := repository.NewSendLogRepository(pool)
	_, _ = slRepo.Create(ctx, &domain.SendLog{RecipientID: id, Channel: domain.ChannelSMS, Status: domain.SendQueued})
	_ = slRepo.UpdateStatus(ctx, id, domain.SendQueued, nil)
	_ = slRepo.UpdateNextRetry(ctx, id, time.Now().UTC())
	_ = slRepo.MarkPrinted(ctx, id, actor)
	_, _, _ = slRepo.List(ctx, repository.SendLogFilters{}, domain.PageRequest{Page: 1, PageSize: 10})
	_, _ = slRepo.GetRetryable(ctx, time.Now().UTC())

	// system_setting
	ssRepo := repository.NewSystemSettingRepository(pool)
	_, _ = ssRepo.Get(ctx, "x")
	_ = ssRepo.Set(ctx, "x", []byte(`"y"`), nil)
	_, _ = ssRepo.GetAll(ctx)

	// job_run
	jrRepo := repository.NewJobRunRepository(pool)
	_, _ = jrRepo.Create(ctx, &domain.JobRunHistory{JobName: "x", Status: domain.JobRunning, StartedAt: time.Now().UTC()})
	_ = jrRepo.Finish(ctx, id, domain.JobCompleted, 0, nil)
	_, _, _ = jrRepo.List(ctx, repository.JobRunFilters{}, domain.PageRequest{Page: 1, PageSize: 10})

	// user
	uRepo := repository.NewUserRepository(pool)
	_, _ = uRepo.GetByUsername(ctx, "x")
	_, _ = uRepo.GetByID(ctx, id)
	_, _ = uRepo.Create(ctx, &domain.User{Username: "x", Email: "x@x", PasswordHash: "x", Role: domain.RoleAuditor, IsActive: true})
	_, _ = uRepo.Update(ctx, &domain.User{ID: id, Username: "x", Email: "x@x", Role: domain.RoleAuditor})
	_ = uRepo.Deactivate(ctx, id)
	_, _ = uRepo.List(ctx, "", nil)

	// report_export
	reRepo := repository.NewReportExportRepository(pool)
	_, _ = reRepo.Create(ctx, &domain.ReportExport{ReportType: "x", Filters: []byte(`{}`), Status: domain.ExportQueued})
	_, _ = reRepo.GetByID(ctx, id)
	_, _, _ = reRepo.List(ctx, repository.ReportExportFilters{SensitiveVisible: true}, domain.PageRequest{Page: 1, PageSize: 10})
	_ = reRepo.UpdateStatus(ctx, id, domain.ExportCompleted, nil, nil, nil, nil)
	_, _ = reRepo.GetExpired(ctx, time.Now().UTC())
	_ = reRepo.Delete(ctx, id)

	// timeline
	tlRepo := repository.NewTimelineRepository(pool)
	_, _ = tlRepo.ListByFulfillmentID(ctx, id)
}

func TestExceptionRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	exRepo := repository.NewExceptionRepository(testPool)
	_, _, fID := seedFulfillment(t)
	created, err := exRepo.Create(ctx, &domain.FulfillmentException{
		FulfillmentID: fID,
		Type:          domain.ExceptionManual,
		Status:        domain.ExceptionOpen,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := exRepo.GetByID(ctx, created.ID); err != nil {
		t.Fatalf("GetByID: %v", err)
	}
}

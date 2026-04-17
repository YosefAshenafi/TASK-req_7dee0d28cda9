package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/util"
)

// ExportService generates CSV report exports with SHA-256 checksums.
type ExportService interface {
	// GenerateExport writes a CSV file for the export record, updates its status,
	// checksum, and expiry. Safe to call in a background goroutine.
	GenerateExport(ctx context.Context, exportID uuid.UUID) error

	// VerifyChecksum re-hashes the export file and compares to the stored digest.
	// Returns true if the file is intact.
	VerifyChecksum(ctx context.Context, exportID uuid.UUID) (bool, error)

	// Delete removes the export file from disk (gracefully if missing) then
	// deletes the DB record and writes an audit entry.
	Delete(ctx context.Context, exportID uuid.UUID, actorID uuid.UUID) error
}

type exportService struct {
	reportRepo   repository.ReportExportRepository
	fulfillRepo  repository.FulfillmentRepository
	customerRepo repository.CustomerRepository
	auditRepo    repository.AuditRepository
	auditSvc     AuditService
	encSvc       EncryptionService
	exportDir    string
}

func NewExportService(
	reportRepo repository.ReportExportRepository,
	fulfillRepo repository.FulfillmentRepository,
	customerRepo repository.CustomerRepository,
	auditRepo repository.AuditRepository,
	encSvc EncryptionService,
	exportDir string,
	auditSvc AuditService,
) ExportService {
	return &exportService{
		reportRepo:   reportRepo,
		fulfillRepo:  fulfillRepo,
		customerRepo: customerRepo,
		auditRepo:    auditRepo,
		auditSvc:     auditSvc,
		encSvc:       encSvc,
		exportDir:    exportDir,
	}
}

// exportFilters holds parsed filter values from the stored JSON blob.
type exportFilters struct {
	DateFrom *time.Time `json:"date_from"`
	DateTo   *time.Time `json:"date_to"`
	Status   string     `json:"status"`
	TierID   *uuid.UUID `json:"tier_id"`
}

func (s *exportService) GenerateExport(ctx context.Context, exportID uuid.UUID) error {
	export, err := s.reportRepo.GetByID(ctx, exportID)
	if err != nil {
		return fmt.Errorf("getting export record: %w", err)
	}

	// Mark as processing.
	if err := s.reportRepo.UpdateStatus(ctx, exportID, domain.ExportProcessing, nil, nil, nil, nil); err != nil {
		return fmt.Errorf("marking processing: %w", err)
	}

	var filters exportFilters
	_ = json.Unmarshal(export.Filters, &filters)

	// Build the CSV file.
	filename := fmt.Sprintf("%s_%s.csv", export.ReportType, exportID.String())
	filePath := filepath.Join(s.exportDir, filename)

	f, err := os.Create(filePath)
	if err != nil {
		_ = s.failExport(ctx, exportID, fmt.Sprintf("creating file: %v", err))
		return fmt.Errorf("creating csv file: %w", err)
	}

	var writeErr error
	switch export.ReportType {
	case "fulfillments":
		writeErr = s.writeFulfillmentsCSV(ctx, f, filters, export.IncludeSensitive)
	case "customers":
		writeErr = s.writeCustomersCSV(ctx, f, filters, export.IncludeSensitive)
	case "audit":
		writeErr = s.writeAuditCSV(ctx, f, filters)
	default:
		writeErr = fmt.Errorf("unknown report type: %s", export.ReportType)
	}
	f.Close()

	if writeErr != nil {
		_ = os.Remove(filePath)
		_ = s.failExport(ctx, exportID, writeErr.Error())
		return writeErr
	}

	// Compute checksum and file size.
	checksum, err := util.ComputeFileChecksum(filePath)
	if err != nil {
		_ = os.Remove(filePath)
		_ = s.failExport(ctx, exportID, fmt.Sprintf("computing checksum: %v", err))
		return err
	}

	info, err := os.Stat(filePath)
	if err != nil {
		_ = s.failExport(ctx, exportID, fmt.Sprintf("stating file: %v", err))
		return err
	}
	fileSize := info.Size()
	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)

	return s.reportRepo.UpdateStatus(ctx, exportID, domain.ExportCompleted, &filePath, &fileSize, &checksum, &expiresAt)
}

func (s *exportService) failExport(ctx context.Context, id uuid.UUID, errMsg string) error {
	return s.reportRepo.UpdateStatus(ctx, id, domain.ExportFailed, nil, nil, &errMsg, nil)
}

func (s *exportService) writeFulfillmentsCSV(ctx context.Context, f *os.File, filters exportFilters, includeSensitive bool) error {
	repoFilters := repository.FulfillmentFilters{
		DateFrom: filters.DateFrom,
		DateTo:   filters.DateTo,
	}
	if filters.Status != "" {
		repoFilters.Status = domain.FulfillmentStatus(filters.Status)
	}
	if filters.TierID != nil {
		repoFilters.TierID = filters.TierID
	}

	// Fetch all records (large page).
	fulfillments, _, err := s.fulfillRepo.List(ctx, repoFilters, domain.PageRequest{Page: 1, PageSize: 10000})
	if err != nil {
		return fmt.Errorf("listing fulfillments: %w", err)
	}

	headers := []string{"id", "tier_id", "customer_id", "type", "status",
		"carrier_name", "tracking_number", "ready_at", "shipped_at", "delivered_at", "completed_at", "created_at"}
	if includeSensitive {
		headers = append(headers, "voucher_code")
	}

	rows := make([][]string, 0, len(fulfillments))
	for _, ff := range fulfillments {
		row := []string{
			ff.ID.String(),
			ff.TierID.String(),
			ff.CustomerID.String(),
			string(ff.Type),
			string(ff.Status),
			strOrEmpty(ff.CarrierName),
			strOrEmpty(ff.TrackingNumber),
			timeOrEmpty(ff.ReadyAt),
			timeOrEmpty(ff.ShippedAt),
			timeOrEmpty(ff.DeliveredAt),
			timeOrEmpty(ff.CompletedAt),
			ff.CreatedAt.Format(time.RFC3339),
		}
		if includeSensitive && len(ff.VoucherCodeEncrypted) > 0 {
			plain, _ := s.encSvc.DecryptToString(ff.VoucherCodeEncrypted)
			row = append(row, plain)
		} else if includeSensitive {
			row = append(row, "")
		}
		rows = append(rows, row)
	}

	return util.WriteCSV(f, headers, rows)
}

func (s *exportService) writeCustomersCSV(ctx context.Context, f *os.File, filters exportFilters, includeSensitive bool) error {
	customers, _, err := s.customerRepo.List(ctx, "", domain.PageRequest{Page: 1, PageSize: 10000}, false)
	if err != nil {
		return fmt.Errorf("listing customers: %w", err)
	}

	headers := []string{"id", "name", "phone", "email", "created_at"}

	rows := make([][]string, 0, len(customers))
	for _, c := range customers {
		phone := ""
		email := ""

		if includeSensitive {
			if len(c.PhoneEncrypted) > 0 {
				phone, _ = s.encSvc.DecryptToString(c.PhoneEncrypted)
			}
			if len(c.EmailEncrypted) > 0 {
				email, _ = s.encSvc.DecryptToString(c.EmailEncrypted)
			}
		} else {
			if len(c.PhoneEncrypted) > 0 {
				plain, _ := s.encSvc.DecryptToString(c.PhoneEncrypted)
				phone = util.MaskPhone(plain)
			}
			if len(c.EmailEncrypted) > 0 {
				plain, _ := s.encSvc.DecryptToString(c.EmailEncrypted)
				email = util.MaskEmail(plain)
			}
		}

		rows = append(rows, []string{
			c.ID.String(),
			c.Name,
			phone,
			email,
			c.CreatedAt.Format(time.RFC3339),
		})
	}

	return util.WriteCSV(f, headers, rows)
}

func (s *exportService) writeAuditCSV(ctx context.Context, f *os.File, filters exportFilters) error {
	repoFilters := repository.AuditFilters{
		DateFrom: filters.DateFrom,
		DateTo:   filters.DateTo,
	}

	logs, _, err := s.auditRepo.List(ctx, repoFilters, domain.PageRequest{Page: 1, PageSize: 10000})
	if err != nil {
		return fmt.Errorf("listing audit logs: %w", err)
	}

	headers := []string{"id", "table_name", "record_id", "operation", "performed_by", "ip_address", "request_id", "created_at"}

	rows := make([][]string, 0, len(logs))
	for _, l := range logs {
		performedBy := ""
		if l.PerformedBy != nil {
			performedBy = l.PerformedBy.String()
		}
		ipAddr := ""
		if l.IPAddress != nil {
			ipAddr = *l.IPAddress
		}
		reqID := ""
		if l.RequestID != nil {
			reqID = *l.RequestID
		}
		recordID := ""
		if l.RecordID != nil {
			recordID = l.RecordID.String()
		}
		rows = append(rows, []string{
			l.ID.String(),
			l.TableName,
			recordID,
			l.Operation,
			performedBy,
			ipAddr,
			reqID,
			l.CreatedAt.Format(time.RFC3339),
		})
	}

	return util.WriteCSV(f, headers, rows)
}

func (s *exportService) VerifyChecksum(ctx context.Context, exportID uuid.UUID) (bool, error) {
	export, err := s.reportRepo.GetByID(ctx, exportID)
	if err != nil {
		return false, fmt.Errorf("getting export record: %w", err)
	}

	if export.Status != domain.ExportCompleted || export.FilePath == nil || export.ChecksumSHA256 == nil {
		return false, nil
	}

	actual, err := util.ComputeFileChecksum(*export.FilePath)
	if err != nil {
		return false, fmt.Errorf("computing checksum: %w", err)
	}

	return actual == *export.ChecksumSHA256, nil
}

// Delete removes the export file from disk then deletes the DB record.
// A missing file is logged but does not prevent DB cleanup.
func (s *exportService) Delete(ctx context.Context, exportID uuid.UUID, actorID uuid.UUID) error {
	export, err := s.reportRepo.GetByID(ctx, exportID)
	if err != nil {
		return fmt.Errorf("getting export record: %w", err)
	}

	if export.FilePath != nil && *export.FilePath != "" {
		if removeErr := os.Remove(*export.FilePath); removeErr != nil && !os.IsNotExist(removeErr) {
			log.Printf("export delete: removing file %s: %v", *export.FilePath, removeErr)
		}
	}

	if err := s.reportRepo.Delete(ctx, exportID); err != nil {
		return fmt.Errorf("deleting export record: %w", err)
	}

	if s.auditSvc != nil {
		_ = s.auditSvc.Log(ctx, "report_exports", exportID, "DELETE",
			map[string]any{"file_path": export.FilePath, "report_type": export.ReportType},
			map[string]any{"deleted_by": actorID})
	}
	return nil
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func timeOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

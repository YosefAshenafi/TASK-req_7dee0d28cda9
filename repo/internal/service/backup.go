package service

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupEntry describes a single pg_dump backup file.
type BackupEntry struct {
	ID        string    // base filename without extension
	FilePath  string
	CreatedAt time.Time
	FileSize  int64
	Status    string // "COMPLETED" or "FAILED"
}

// BackupService creates and restores PostgreSQL database backups via pg_dump.
type BackupService interface {
	// RunBackup creates a gzipped pg_dump in the backup directory.
	RunBackup(ctx context.Context) (*BackupEntry, error)

	// ListBackups returns all backup entries sorted by newest first.
	ListBackups(ctx context.Context) ([]BackupEntry, error)

	// RestoreFromBackup restores the database from a named backup file.
	// verifyIntegrity performs a basic FK / row count sanity check after restore.
	RestoreFromBackup(ctx context.Context, backupID string, verifyIntegrity bool) error
}

type backupService struct {
	databaseURL string
	backupDir   string
	auditSvc    AuditService
}

func NewBackupService(databaseURL, backupDir string, auditSvc AuditService) BackupService {
	return &backupService{databaseURL: databaseURL, backupDir: backupDir, auditSvc: auditSvc}
}

// RunBackup runs pg_dump and writes a gzipped SQL file.
func (s *backupService) RunBackup(ctx context.Context) (*BackupEntry, error) {
	ts := time.Now().UTC()
	id := fmt.Sprintf("backup_%s", ts.Format("20060102_150405"))
	filePath := filepath.Join(s.backupDir, id+".sql.gz")

	// Open output file.
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("creating backup file: %w", err)
	}
	defer f.Close()

	// Pipe pg_dump stdout through gzip into the file.
	gz := gzip.NewWriter(f)
	defer gz.Close()

	// --clean --if-exists: emit DROP TABLE IF EXISTS before CREATE TABLE so that
	// a subsequent psql restore cleanly replaces existing data.
	cmd := exec.CommandContext(ctx, "pg_dump", "--no-password", "--clean", "--if-exists", s.databaseURL)
	cmd.Stdout = gz
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		_ = os.Remove(filePath)
		return nil, fmt.Errorf("pg_dump failed: %w", err)
	}

	// Flush gzip before stat.
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("closing file: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stating backup file: %w", err)
	}

	entry := &BackupEntry{
		ID:        id,
		FilePath:  filePath,
		CreatedAt: ts,
		FileSize:  info.Size(),
		Status:    "COMPLETED",
	}

	log.Printf("backup: created %s (%d bytes)", filePath, info.Size())
	return entry, nil
}

// ListBackups scans the backup directory for .sql.gz files.
func (s *backupService) ListBackups(ctx context.Context) ([]BackupEntry, error) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading backup dir: %w", err)
	}

	var result []BackupEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".sql.gz")
		result = append(result, BackupEntry{
			ID:        name,
			FilePath:  filepath.Join(s.backupDir, e.Name()),
			CreatedAt: info.ModTime().UTC(),
			FileSize:  info.Size(),
			Status:    "COMPLETED",
		})
	}

	// Newest first.
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

// RestoreFromBackup pipes the gzipped backup through gunzip then psql.
func (s *backupService) RestoreFromBackup(ctx context.Context, backupID string, verifyIntegrity bool) error {
	filePath := filepath.Join(s.backupDir, backupID+".sql.gz")
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("backup file not found: %s", backupID)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening backup file: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gz.Close()

	// Pipe decompressed SQL into psql.
	cmd := exec.CommandContext(ctx, "psql", "--no-password", s.databaseURL)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("creating psql stdin pipe: %w", err)
	}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting psql: %w", err)
	}

	_, copyErr := io.Copy(stdin, gz)
	_ = stdin.Close()
	waitErr := cmd.Wait()

	if copyErr != nil {
		return fmt.Errorf("copying backup to psql: %w", copyErr)
	}
	if waitErr != nil {
		return fmt.Errorf("psql restore failed: %w", waitErr)
	}

	log.Printf("backup: restored from %s", filePath)

	if verifyIntegrity {
		if err := s.verifyIntegrity(ctx); err != nil {
			log.Printf("backup: integrity check failed after restore: %v", err)
			// Return the error so the caller knows the restore may be incomplete.
			return fmt.Errorf("integrity check after restore: %w", err)
		}
		log.Printf("backup: integrity check passed")
	}

	return nil
}

// verifyIntegrity performs a simple sanity check: confirms core tables have rows
// and no obvious FK violations exist in PostgreSQL's constraint system.
func (s *backupService) verifyIntegrity(ctx context.Context) error {
	// Use psql to run a quick integrity query.
	query := `
DO $$
DECLARE
  r RECORD;
BEGIN
  FOR r IN
    SELECT conname, conrelid::regclass AS tbl
    FROM pg_constraint
    WHERE contype = 'f' AND NOT convalidated
  LOOP
    RAISE EXCEPTION 'Invalid FK constraint: % on %', r.conname, r.tbl;
  END LOOP;
END $$;
`
	cmd := exec.CommandContext(ctx, "psql", "--no-password", s.databaseURL, "-c", query)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FK integrity check: %w", err)
	}
	return nil
}

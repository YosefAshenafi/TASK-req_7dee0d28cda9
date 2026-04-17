package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BackupEntry describes a single backup archive.
type BackupEntry struct {
	ID        string    // base filename without extension
	FilePath  string
	CreatedAt time.Time
	FileSize  int64
	Status    string // "COMPLETED" or "FAILED"
}

// BackupService creates and restores PostgreSQL database backups (plus optional
// attached assets) bundled as a tar.gz archive with a sha256 checksum file.
type BackupService interface {
	// RunBackup creates a tar.gz containing the DB dump and asset directory.
	RunBackup(ctx context.Context) (*BackupEntry, error)

	// ListBackups returns all backup entries sorted by newest first.
	ListBackups(ctx context.Context) ([]BackupEntry, error)

	// RestoreFromBackup restores the database (and assets if present) from a
	// named backup archive. verifyIntegrity performs a FK / row-count check.
	RestoreFromBackup(ctx context.Context, backupID string, verifyIntegrity bool) error
}

type backupService struct {
	databaseURL string
	backupDir   string
	assetsDir   string
	auditSvc    AuditService
}

func NewBackupService(databaseURL, backupDir, assetsDir string, auditSvc AuditService) BackupService {
	return &backupService{
		databaseURL: databaseURL,
		backupDir:   backupDir,
		assetsDir:   assetsDir,
		auditSvc:    auditSvc,
	}
}

// RunBackup creates a tar.gz bundling db.sql.gz (pg_dump) and any assets.
// A sha256 checksum file is written alongside the archive.
func (s *backupService) RunBackup(ctx context.Context) (*BackupEntry, error) {
	ts := time.Now().UTC()
	id := fmt.Sprintf("backup_%s", ts.Format("20060102_150405"))
	archivePath := filepath.Join(s.backupDir, id+".tar.gz")
	checksumPath := archivePath + ".sha256"

	if err := os.MkdirAll(s.backupDir, 0755); err != nil {
		return nil, fmt.Errorf("creating backup dir: %w", err)
	}

	f, err := os.Create(archivePath)
	if err != nil {
		return nil, fmt.Errorf("creating archive file: %w", err)
	}

	h := sha256.New()
	mw := io.MultiWriter(f, h)

	gz := gzip.NewWriter(mw)
	tw := tar.NewWriter(gz)

	// Dump the database into a temporary file first (pg_dump → temp gzip).
	tmpDump, err := os.CreateTemp("", "fulfillops_dump_*.sql.gz")
	if err != nil {
		f.Close()
		_ = os.Remove(archivePath)
		return nil, fmt.Errorf("creating temp dump file: %w", err)
	}
	tmpDumpPath := tmpDump.Name()
	defer os.Remove(tmpDumpPath)

	dumpGz := gzip.NewWriter(tmpDump)
	cmd := exec.CommandContext(ctx, "pg_dump", "--no-password", "--clean", "--if-exists", s.databaseURL)
	cmd.Stdout = dumpGz
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		tmpDump.Close()
		f.Close()
		_ = os.Remove(archivePath)
		return nil, fmt.Errorf("pg_dump failed: %w", err)
	}
	if err := dumpGz.Close(); err != nil {
		tmpDump.Close()
		f.Close()
		_ = os.Remove(archivePath)
		return nil, fmt.Errorf("closing dump gzip: %w", err)
	}
	if err := tmpDump.Close(); err != nil {
		f.Close()
		_ = os.Remove(archivePath)
		return nil, fmt.Errorf("closing temp dump: %w", err)
	}

	// Add db.sql.gz to the archive.
	if err := addFileToTar(tw, tmpDumpPath, "db.sql.gz"); err != nil {
		f.Close()
		_ = os.Remove(archivePath)
		return nil, fmt.Errorf("adding db dump to archive: %w", err)
	}

	// Add assets directory if it exists and is non-empty.
	if s.assetsDir != "" {
		if info, statErr := os.Stat(s.assetsDir); statErr == nil && info.IsDir() {
			if err := addDirToTar(tw, s.assetsDir, "assets"); err != nil {
				log.Printf("backup: adding assets dir: %v", err)
				// Non-fatal — continue without assets.
			}
		}
	}

	if err := tw.Close(); err != nil {
		f.Close()
		_ = os.Remove(archivePath)
		return nil, fmt.Errorf("closing tar writer: %w", err)
	}
	if err := gz.Close(); err != nil {
		f.Close()
		_ = os.Remove(archivePath)
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(archivePath)
		return nil, fmt.Errorf("closing archive file: %w", err)
	}

	checksum := hex.EncodeToString(h.Sum(nil))
	if err := os.WriteFile(checksumPath, []byte(checksum+"\n"), 0644); err != nil {
		log.Printf("backup: writing checksum file: %v", err)
	}

	info, err := os.Stat(archivePath)
	if err != nil {
		return nil, fmt.Errorf("stating archive: %w", err)
	}

	entry := &BackupEntry{
		ID:        id,
		FilePath:  archivePath,
		CreatedAt: ts,
		FileSize:  info.Size(),
		Status:    "COMPLETED",
	}

	log.Printf("backup: created %s (%d bytes, sha256=%s)", archivePath, info.Size(), checksum)

	if s.auditSvc != nil {
		_ = s.auditSvc.Log(context.Background(), "backups", uuid.Nil, "BACKUP_RUN", nil, map[string]any{
			"backup_id": id,
			"file_size": info.Size(),
			"checksum":  checksum,
		})
	}

	return entry, nil
}

// ListBackups scans the backup directory for .tar.gz files (and legacy .sql.gz).
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
		if e.IsDir() {
			continue
		}
		name := e.Name()
		var id string
		switch {
		case strings.HasSuffix(name, ".tar.gz"):
			id = strings.TrimSuffix(name, ".tar.gz")
		case strings.HasSuffix(name, ".sql.gz"):
			id = strings.TrimSuffix(name, ".sql.gz")
		default:
			continue
		}
		if strings.HasSuffix(id, "backup_") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, BackupEntry{
			ID:        id,
			FilePath:  filepath.Join(s.backupDir, name),
			CreatedAt: info.ModTime().UTC(),
			FileSize:  info.Size(),
			Status:    "COMPLETED",
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

// RestoreFromBackup unpacks a .tar.gz archive, restores the DB dump, and
// optionally restores the assets directory via atomic staged swap.
func (s *backupService) RestoreFromBackup(ctx context.Context, backupID string, verifyIntegrity bool) error {
	// Support both new (.tar.gz) and legacy (.sql.gz) formats.
	tarPath := filepath.Join(s.backupDir, backupID+".tar.gz")
	legacyPath := filepath.Join(s.backupDir, backupID+".sql.gz")

	if _, err := os.Stat(tarPath); err == nil {
		return s.restoreFromTarGz(ctx, tarPath, backupID, verifyIntegrity)
	}
	if _, err := os.Stat(legacyPath); err == nil {
		return s.restoreFromLegacySqlGz(ctx, legacyPath, backupID, verifyIntegrity)
	}
	return fmt.Errorf("backup file not found: %s", backupID)
}

func (s *backupService) restoreFromTarGz(ctx context.Context, archivePath, backupID string, verifyIntegrity bool) error {
	// Verify checksum if a .sha256 file exists alongside the archive.
	checksumPath := archivePath + ".sha256"
	if storedChecksum, err := os.ReadFile(checksumPath); err == nil {
		actual, err := fileChecksum(archivePath)
		if err != nil {
			return fmt.Errorf("computing archive checksum: %w", err)
		}
		if actual != strings.TrimSpace(string(storedChecksum)) {
			return fmt.Errorf("archive checksum mismatch: stored=%s actual=%s", strings.TrimSpace(string(storedChecksum)), actual)
		}
	}

	// Unpack into a temp directory.
	tmpDir, err := os.MkdirTemp("", "fulfillops_restore_*")
	if err != nil {
		return fmt.Errorf("creating restore temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractTarGz(archivePath, tmpDir); err != nil {
		return fmt.Errorf("extracting archive: %w", err)
	}

	// Restore the database dump.
	dbDumpPath := filepath.Join(tmpDir, "db.sql.gz")
	if err := s.restoreDBFromSqlGz(ctx, dbDumpPath); err != nil {
		return fmt.Errorf("restoring database: %w", err)
	}

	log.Printf("backup: restored DB from %s", archivePath)

	// Restore assets atomically (staged → swap).
	srcAssets := filepath.Join(tmpDir, "assets")
	if s.assetsDir != "" {
		if _, err := os.Stat(srcAssets); err == nil {
			staged := s.assetsDir + ".restore_stage"
			if err := copyDir(srcAssets, staged); err != nil {
				log.Printf("backup: staging assets failed: %v", err)
			} else {
				backup := s.assetsDir + ".pre_restore"
				_ = os.Rename(s.assetsDir, backup)
				if err := os.Rename(staged, s.assetsDir); err != nil {
					_ = os.Rename(backup, s.assetsDir) // rollback
					log.Printf("backup: swapping assets failed: %v", err)
				} else {
					os.RemoveAll(backup)
					log.Printf("backup: restored assets to %s", s.assetsDir)
				}
			}
		}
	}

	if verifyIntegrity {
		if err := s.verifyIntegrity(ctx); err != nil {
			log.Printf("backup: integrity check failed after restore: %v", err)
			return fmt.Errorf("integrity check after restore: %w", err)
		}
		log.Printf("backup: integrity check passed")
	}

	if s.auditSvc != nil {
		_ = s.auditSvc.Log(context.Background(), "backups", uuid.Nil, "RESTORE", nil, map[string]string{
			"backup_id": backupID,
			"verified":  fmt.Sprintf("%v", verifyIntegrity),
		})
	}
	return nil
}

func (s *backupService) restoreFromLegacySqlGz(ctx context.Context, filePath, backupID string, verifyIntegrity bool) error {
	if err := s.restoreDBFromSqlGz(ctx, filePath); err != nil {
		return err
	}
	log.Printf("backup: restored from legacy %s", filePath)

	if verifyIntegrity {
		if err := s.verifyIntegrity(ctx); err != nil {
			return fmt.Errorf("integrity check after restore: %w", err)
		}
	}
	if s.auditSvc != nil {
		_ = s.auditSvc.Log(context.Background(), "backups", uuid.Nil, "RESTORE", nil, map[string]string{
			"backup_id": backupID,
			"verified":  fmt.Sprintf("%v", verifyIntegrity),
		})
	}
	return nil
}

func (s *backupService) restoreDBFromSqlGz(ctx context.Context, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening dump file: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gz.Close()

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
		return fmt.Errorf("copying dump to psql: %w", copyErr)
	}
	return waitErr
}

// verifyIntegrity performs post-restore FK and table sanity checks.
func (s *backupService) verifyIntegrity(ctx context.Context) error {
	query := `
DO $$
DECLARE
  r RECORD;
  violations INT;
  sql TEXT;
BEGIN
  -- 1. No unvalidated FK constraints.
  FOR r IN
    SELECT conname, conrelid::regclass AS tbl
    FROM pg_constraint
    WHERE contype = 'f' AND NOT convalidated
  LOOP
    RAISE EXCEPTION 'Invalid FK constraint: % on %', r.conname, r.tbl;
  END LOOP;

  -- 2. Walk every FK and detect dangling rows.
  FOR r IN
    SELECT
      tc.table_name        AS child_table,
      kcu.column_name      AS child_col,
      ccu.table_name       AS parent_table,
      ccu.column_name      AS parent_col,
      tc.constraint_name   AS cname
    FROM information_schema.table_constraints tc
    JOIN information_schema.key_column_usage kcu
      ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
    JOIN information_schema.constraint_column_usage ccu
      ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
    WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = 'public'
  LOOP
    sql := format(
      'SELECT COUNT(*) FROM %I c WHERE c.%I IS NOT NULL AND NOT EXISTS (SELECT 1 FROM %I p WHERE p.%I = c.%I)',
      r.child_table, r.child_col, r.parent_table, r.parent_col, r.child_col);
    EXECUTE sql INTO violations;
    IF violations > 0 THEN
      RAISE EXCEPTION 'Dangling FK % on %.% (% rows)', r.cname, r.child_table, r.child_col, violations;
    END IF;
  END LOOP;

  -- 3. Core tables must be queryable.
  PERFORM 1 FROM users LIMIT 1;
  PERFORM 1 FROM reward_tiers LIMIT 1;
  PERFORM 1 FROM customers LIMIT 1;
  PERFORM 1 FROM fulfillments LIMIT 1;
END $$;
`
	cmd := exec.CommandContext(ctx, "psql", "--no-password", "-v", "ON_ERROR_STOP=1", s.databaseURL, "-c", query)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FK integrity check: %w", err)
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func addFileToTar(tw *tar.Writer, srcPath, destName string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	hdr := &tar.Header{
		Name:    destName,
		Mode:    0644,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

func addDirToTar(tw *tar.Writer, srcDir, prefix string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		destName := filepath.Join(prefix, rel)
		if info.IsDir() {
			return tw.WriteHeader(&tar.Header{
				Name:     destName + "/",
				Typeflag: tar.TypeDir,
				Mode:     0755,
				ModTime:  info.ModTime(),
			})
		}
		return addFileToTar(tw, path, destName)
	})
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		dest := filepath.Join(destDir, filepath.Clean("/"+hdr.Name))
		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(dest, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		out, err := os.Create(dest)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		out.Close()
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		dest := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dest, info.Mode())
		}
		return copyFile(path, dest)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func fileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesEnvAndDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("FULFILLOPS_SESSION_SECRET", "12345678901234567890123456789012")
	t.Setenv("FULFILLOPS_EXPORT_DIR", "")
	t.Setenv("FULFILLOPS_BACKUP_DIR", "")
	t.Setenv("FULFILLOPS_PORT", "")
	t.Setenv("GIN_MODE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DatabaseURL != "postgres://example" {
		t.Fatalf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.EncryptionKeyPath != "/app/encryption.key" {
		t.Fatalf("EncryptionKeyPath = %q", cfg.EncryptionKeyPath)
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q", cfg.Port)
	}
	if cfg.GinMode != "debug" {
		t.Fatalf("GinMode = %q", cfg.GinMode)
	}
	if !cfg.SecureCookies {
		t.Fatal("SecureCookies should default to true")
	}
}

func TestValidateCreatesDirsAndRejectsShortSecret(t *testing.T) {
	tmp := t.TempDir()

	cfg := &Config{
		DatabaseURL:   "postgres://example",
		SessionSecret: "short",
		ExportDir:     filepath.Join(tmp, "exports"),
		BackupDir:     filepath.Join(tmp, "backups"),
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected short secret validation error")
	}

	cfg.SessionSecret = "12345678901234567890123456789012"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if _, err := os.Stat(cfg.ExportDir); err != nil {
		t.Fatalf("export dir not created: %v", err)
	}
	if _, err := os.Stat(cfg.BackupDir); err != nil {
		t.Fatalf("backup dir not created: %v", err)
	}
}

func TestLoadEnvParseError(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("FULFILLOPS_SESSION_SECRET", "12345678901234567890123456789012")
	t.Setenv("FULFILLOPS_SECURE_COOKIES", "not-a-boolean")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error from env.Parse")
	}
}

func TestValidateExportDirMkdirError(t *testing.T) {
	tmp := t.TempDir()
	blockPath := filepath.Join(tmp, "block")
	if err := os.WriteFile(blockPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		DatabaseURL:   "postgres://example",
		SessionSecret: "12345678901234567890123456789012",
		ExportDir:     filepath.Join(blockPath, "nested", "exports"),
		BackupDir:     filepath.Join(tmp, "backups"),
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error when export dir cannot be created")
	}
}

func TestValidateBackupDirMkdirError(t *testing.T) {
	tmp := t.TempDir()
	blockPath := filepath.Join(tmp, "block")
	if err := os.WriteFile(blockPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		DatabaseURL:   "postgres://example",
		SessionSecret: "12345678901234567890123456789012",
		ExportDir:     filepath.Join(tmp, "exports"),
		BackupDir:     filepath.Join(blockPath, "nested", "backups"),
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error when backup dir cannot be created")
	}
}

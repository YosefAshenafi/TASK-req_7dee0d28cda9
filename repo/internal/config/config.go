package config

import (
	"fmt"
	"os"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	DatabaseURL        string `env:"DATABASE_URL,required"`
	EncryptionKeyPath  string `env:"FULFILLOPS_ENCRYPTION_KEY_PATH" envDefault:"/app/encryption.key"`
	ExportDir          string `env:"FULFILLOPS_EXPORT_DIR" envDefault:"/app/exports"`
	BackupDir          string `env:"FULFILLOPS_BACKUP_DIR" envDefault:"/app/backups"`
	MigrationsPath     string `env:"FULFILLOPS_MIGRATIONS_PATH" envDefault:"/app/migrations"`
	Port               string `env:"FULFILLOPS_PORT" envDefault:"8080"`
	SessionSecret      string `env:"FULFILLOPS_SESSION_SECRET" envDefault:"change-me-in-production-32bytes!"`
	GinMode            string `env:"GIN_MODE" envDefault:"debug"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if len(c.SessionSecret) < 32 {
		return fmt.Errorf("FULFILLOPS_SESSION_SECRET must be at least 32 characters")
	}
	if err := os.MkdirAll(c.ExportDir, 0755); err != nil {
		return fmt.Errorf("creating export dir %s: %w", c.ExportDir, err)
	}
	if err := os.MkdirAll(c.BackupDir, 0755); err != nil {
		return fmt.Errorf("creating backup dir %s: %w", c.BackupDir, err)
	}
	return nil
}

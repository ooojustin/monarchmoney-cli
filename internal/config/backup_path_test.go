package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupPathPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("backup_path: /tmp/from-file.journal\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.BackupPath != "/tmp/from-file.journal" {
		t.Fatalf("BackupPath = %q, want %q (file over default)", cfg.BackupPath, "/tmp/from-file.journal")
	}

	t.Setenv("MONARCH_BACKUP_PATH", "/tmp/from-env.journal")
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("Load() with env error = %v", err)
	}
	if cfg.BackupPath != "/tmp/from-env.journal" {
		t.Fatalf("BackupPath = %q, want %q (env over file)", cfg.BackupPath, "/tmp/from-env.journal")
	}
}

func TestBackupPathDefaultsToEmpty(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.BackupPath != "" {
		t.Fatalf("BackupPath = %q, want empty default (opt-in)", cfg.BackupPath)
	}
}

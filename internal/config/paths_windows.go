//go:build windows

package config

import (
	"os"
	"path/filepath"
)

func defaultDir() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "monarchmoney-cli")
}

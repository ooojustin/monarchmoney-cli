package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

func DevicePath(sessionPath string) string {
	return filepath.Join(filepath.Dir(sessionPath), "device-id")
}

func LoadDeviceUUID(sessionPath string) (string, error) {
	data, err := os.ReadFile(DevicePath(sessionPath))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	deviceUUID := strings.TrimSpace(string(data))
	if _, err := uuid.Parse(deviceUUID); err != nil {
		return "", fmt.Errorf("invalid device ID: %w", err)
	}
	return deviceUUID, nil
}

func LoadOrCreateDeviceUUID(sessionPath string) (string, error) {
	deviceUUID, err := LoadDeviceUUID(sessionPath)
	if err != nil || deviceUUID != "" {
		return deviceUUID, err
	}
	if err := os.MkdirAll(filepath.Dir(DevicePath(sessionPath)), 0o700); err != nil {
		return "", err
	}

	deviceUUID = uuid.NewString()
	file, err := os.CreateTemp(filepath.Dir(DevicePath(sessionPath)), ".device-id-")
	if err != nil {
		return "", err
	}
	tempPath := file.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return "", err
	}
	if _, err := file.WriteString(deviceUUID + "\n"); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Link(tempPath, DevicePath(sessionPath)); errors.Is(err, os.ErrExist) {
		return LoadDeviceUUID(sessionPath)
	} else if err != nil {
		return "", err
	}
	return deviceUUID, nil
}

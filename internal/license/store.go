package license

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type StoredLicense struct {
	LicenseKey string `json:"license_key"`
	InstanceID string `json:"instance_id"`
}

func getLicensePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(home, ".klip-sync")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(dir, "license.json"), nil
}

func Save(license StoredLicense) error {
	path, err := getLicensePath()
	if err != nil {
		return fmt.Errorf("failed to get license path: %w", err)
	}

	data, err := json.MarshalIndent(license, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal license: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}

func Load() (*StoredLicense, error) {
	path, err := getLicensePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get license path: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no license saved
		}
		return nil, fmt.Errorf("failed to read license: %w", err)
	}

	var stored StoredLicense
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("failed to unmarshal license: %w", err)
	}

	return &stored, nil
}

func Remove() error {
	path, err := getLicensePath()
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

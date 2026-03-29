package app

import (
	"fmt"
	"os"
	"path/filepath"
)

func autostartEnabled() bool {
	_, err := os.Stat(desktopEntryPath())
	return err == nil
}

func enableAutostart() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Klip
Exec=%s
X-GNOME-Autostart-enabled=true
`, execPath)

	path := desktopEntryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create autostart dir: %w", err)
	}

	return os.WriteFile(path, []byte(entry), 0644)
}

func disableAutostart() error {
	err := os.Remove(desktopEntryPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func desktopEntryPath() string {
	configDir, _ := os.UserConfigDir()
	return filepath.Join(configDir, "autostart", "klip.desktop")
}

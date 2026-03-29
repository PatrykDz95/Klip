package app

import (
	"fmt"
	"os"
	"path/filepath"
)

const launchAgentLabel = "app.klip-it.klip"

func autostartEnabled() bool {
	_, err := os.Stat(launchAgentPath())
	return err == nil
}

func enableAutostart() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>
`, launchAgentLabel, execPath)

	path := launchAgentPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	return os.WriteFile(path, []byte(plist), 0644)
}

func disableAutostart() error {
	err := os.Remove(launchAgentPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func launchAgentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

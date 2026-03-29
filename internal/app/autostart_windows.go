package app

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const registryKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const registryValueName = "Klip"

func autostartEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue(registryValueName)
	return err == nil
}

func enableAutostart() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry key: %w", err)
	}
	defer key.Close()

	return key.SetStringValue(registryValueName, execPath)
}

func disableAutostart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry key: %w", err)
	}
	defer key.Close()

	err = key.DeleteValue(registryValueName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}

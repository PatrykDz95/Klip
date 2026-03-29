//go:build windows

package app

import (
	"os/exec"
	"strings"
)

func promptLicenseKey() string {
	script := `Add-Type -AssemblyName Microsoft.VisualBasic; [Microsoft.VisualBasic.Interaction]::InputBox("Enter your license key:", "Klip Pro")`
	out, err := exec.Command("powershell", "-Command", script).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

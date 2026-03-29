//go:build darwin

package app

import (
	"os/exec"
	"strings"
)

func promptLicenseKey() string {
	script := `display dialog "Enter your license key:" default answer "" with title "Klip Pro" buttons {"Cancel", "Activate"} default button "Activate"
set result to text returned of result
return result`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

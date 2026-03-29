//go:build linux

package app

import (
	"os/exec"
	"strings"
)

func promptLicenseKey() string {
	out, err := exec.Command("zenity", "--entry", "--title=Klip Pro", "--text=Enter your license key:").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

//go:build darwin

package clipboard

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

type darwinClipboard struct {
	lastHash [32]byte
	logger   *slog.Logger
}

func newClipboard(logger *slog.Logger) (Clipboard, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if _, err := exec.LookPath("pbpaste"); err != nil {
		return nil, fmt.Errorf("pbpaste not found: %w", err)
	}
	if _, err := exec.LookPath("pbcopy"); err != nil {
		return nil, fmt.Errorf("pbcopy not found: %w", err)
	}

	return &darwinClipboard{logger: logger}, nil
}

func (c *darwinClipboard) Get() (string, error) {
	cmd := exec.Command("pbpaste")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pbpaste failed: %w", err)
	}
	return string(out), nil
}

func (c *darwinClipboard) Set(content string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(content)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pbcopy failed: %w", err)
	}
	c.lastHash = sha256.Sum256([]byte(content)) // prevent feedback loop
	return nil
}

func (c *darwinClipboard) Watch(onChange func(content string)) error {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		content, err := c.Get()
		if err != nil {
			continue
		}

		if content == "" {
			continue
		}

		hash := sha256.Sum256([]byte(content))
		if hash != c.lastHash {
			c.lastHash = hash
			onChange(content)
		}
	}

	return nil
}

func (c *darwinClipboard) GetFiles() ([]string, error) {
	// osascript is always available on macOS — no external tools needed.
	// «class furl» is the AppleScript type for file URLs (POSIX paths via coercion).
	script := `
				try
					set theClip to the clipboard as «class furl»
					set theFiles to {}
					if class of theClip is list then
						repeat with aFile in theClip
							set end of theFiles to POSIX path of aFile
						end repeat
					else
						set theFiles to {POSIX path of theClip}
					end if
					set AppleScript's text item delimiters to linefeed
					theFiles as text
				on error
					""
				end try`

	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return nil, fmt.Errorf("osascript failed: %w", err)
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

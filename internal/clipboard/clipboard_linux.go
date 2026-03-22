//go:build linux

package clipboard

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

type linuxBackend int

const (
	backendWayland linuxBackend = iota
	backendXclip
	backendXsel
)

type linuxClipboard struct {
	lastHash [32]byte
	logger   *slog.Logger
	backend  linuxBackend
}

func newClipboard(logger *slog.Logger) (Clipboard, error) {
	if logger == nil {
		logger = slog.Default()
	}

	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("wl-paste"); err == nil {
			if _, err := exec.LookPath("wl-copy"); err == nil {
				return &linuxClipboard{backend: backendWayland, logger: logger}, nil
			}
		}
		logger.Warn("Wayland detected but wl-clipboard not found, falling back to X11")
	}

	if _, err := exec.LookPath("xclip"); err == nil {
		return &linuxClipboard{backend: backendXclip, logger: logger}, nil
	}
	if _, err := exec.LookPath("xsel"); err == nil {
		return &linuxClipboard{backend: backendXsel, logger: logger}, nil
	}

	return nil, fmt.Errorf("no clipboard tool found; install wl-clipboard (Wayland), xclip or xsel (X11)")
}

func (c *linuxClipboard) Get() (string, error) {
	var cmd *exec.Cmd
	switch c.backend {
	case backendWayland:
		cmd = exec.Command("wl-paste", "--no-newline")
	case backendXclip:
		cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
	case backendXsel:
		cmd = exec.Command("xsel", "--clipboard", "--output")
	}

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("clipboard read failed: %w", err)
	}
	return string(out), nil
}

func (c *linuxClipboard) Set(content string) error {
	var cmd *exec.Cmd
	switch c.backend {
	case backendWayland:
		cmd = exec.Command("wl-copy")
	case backendXclip:
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case backendXsel:
		cmd = exec.Command("xsel", "--clipboard", "--input")
	}

	cmd.Stdin = strings.NewReader(content)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clipboard write failed: %w", err)
	}
	c.lastHash = sha256.Sum256([]byte(content))
	return nil
}

func (c *linuxClipboard) Watch(onChange func(content string)) error {
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

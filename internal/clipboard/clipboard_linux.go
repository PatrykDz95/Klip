//go:build linux

package clipboard

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/url"
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

func (c *linuxClipboard) GetFiles() ([]string, error) {
	switch c.backend {
	case backendWayland:
		return c.getFilesWayland()
	case backendXclip:
		return c.getFilesXclip()
	case backendXsel:
		return nil, nil // xsel doesn't support MIME types
	}
	return nil, nil
}

func (c *linuxClipboard) getFilesWayland() ([]string, error) {
	out, err := exec.Command("wl-paste", "--list-types").Output()
	if err != nil {
		return nil, nil
	}

	types := string(out)
	var mimeType string
	switch {
	case strings.Contains(types, "text/uri-list"):
		mimeType = "text/uri-list"
	case strings.Contains(types, "x-special/gnome-copied-files"):
		mimeType = "x-special/gnome-copied-files"
	default:
		return nil, nil
	}

	out, err = exec.Command("wl-paste", "--type", mimeType).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read file list from clipboard: %w", err)
	}

	return parseURIList(string(out)), nil
}

func (c *linuxClipboard) getFilesXclip() ([]string, error) {
	targetsOut, err := exec.Command("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o").Output()
	if err != nil {
		return nil, nil
	}

	targets := string(targetsOut)
	var mimeType string
	switch {
	case strings.Contains(targets, "text/uri-list"):
		mimeType = "text/uri-list"
	case strings.Contains(targets, "x-special/gnome-copied-files"):
		mimeType = "x-special/gnome-copied-files"
	default:
		return nil, nil
	}

	out, err := exec.Command("xclip", "-selection", "clipboard", "-t", mimeType, "-o").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read file list from clipboard: %w", err)
	}

	return parseURIList(string(out)), nil
}

func parseURIList(raw string) []string {
	var files []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "copy" || line == "cut" || strings.HasPrefix(line, "#") {
			continue
		}
		u, err := url.Parse(line)
		if err != nil || u.Scheme != "file" {
			continue
		}
		files = append(files, u.Path)
	}
	return files
}

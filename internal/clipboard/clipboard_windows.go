//go:build windows

package clipboard

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")

	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	getClipboardData = user32.NewProc("GetClipboardData")
	setClipboardData = user32.NewProc("SetClipboardData")
	emptyClipboard   = user32.NewProc("EmptyClipboard")

	globalAlloc  = kernel32.NewProc("GlobalAlloc")
	globalFree   = kernel32.NewProc("GlobalFree")
	globalLock   = kernel32.NewProc("GlobalLock")
	globalUnlock = kernel32.NewProc("GlobalUnlock")

	dragQueryFileW = shell32.NewProc("DragQueryFileW")
)

const (
	cfUnicodeText = 13
	cfHDrop       = 15
	gmemMoveable  = 0x0002 // value GMEM_MOVEABLE
)

type windowsClipboard struct {
	lastHash [32]byte
	logger   *slog.Logger
}

func newClipboard(logger *slog.Logger) (Clipboard, error) {
	if logger == nil {
		logger = slog.Default()
	}
	return &windowsClipboard{logger: logger}, nil
}

func (c *windowsClipboard) Get() (string, error) {
	r, _, err := openClipboard.Call(0)
	if r == 0 {
		return "", fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer func() {
		if r, _, e := closeClipboard.Call(); r == 0 {
			c.logger.Warn("CloseClipboard failed", "error", e)
		}
	}()

	h, _, _ := getClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", nil
	}

	ptr, _, _ := globalLock.Call(h)
	if ptr == 0 {
		return "", fmt.Errorf("failed to lock clipboard memory")
	}
	defer func() {
		if r, _, e := globalUnlock.Call(h); r == 0 {
			c.logger.Warn("GlobalUnlock failed", "error", e)
		}
	}()

	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(ptr))), nil //nolint:unsafeptr
}

func (c *windowsClipboard) Set(content string) error {
	r, _, err := openClipboard.Call(0)
	if r == 0 {
		return fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer func() {
		if r, _, e := closeClipboard.Call(); r == 0 {
			c.logger.Warn("CloseClipboard failed", "error", e)
		}
	}()

	if r, _, e := emptyClipboard.Call(); r == 0 {
		return fmt.Errorf("failed to empty clipboard: %w", e)
	}

	utf16, err := syscall.UTF16FromString(content)
	if err != nil {
		return fmt.Errorf("failed to convert to UTF16: %w", err)
	}

	h, _, _ := globalAlloc.Call(gmemMoveable, uintptr(len(utf16)*2))
	if h == 0 {
		return fmt.Errorf("failed to allocate global memory")
	}

	ptr, _, _ := globalLock.Call(h)
	if ptr == 0 {
		if r, _, e := globalFree.Call(h); r == 0 {
			c.logger.Warn("GlobalFree failed", "error", e)
		}
		return fmt.Errorf("failed to lock global memory")
	}

	dstSlice := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16)) //nolint:unsafeptr
	copy(dstSlice, utf16)

	if r, _, e := globalUnlock.Call(h); r == 0 {
		c.logger.Warn("GlobalUnlock failed", "error", e)
	}

	r, _, err = setClipboardData.Call(cfUnicodeText, h)
	if r == 0 {
		return fmt.Errorf("failed to set clipboard data: %w", err)
	}
	c.lastHash = sha256.Sum256([]byte(content)) // prevent feedback loop
	return nil
}

func (c *windowsClipboard) Watch(onChange func(content string)) error {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		content, err := c.Get()
		if err != nil {
			c.logger.Warn("Failed to read clipboard", "error", err)
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

func (c *windowsClipboard) GetFiles() ([]string, error) {
	r, _, err := openClipboard.Call(0)
	if r == 0 {
		return nil, fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer func() {
		if r, _, e := closeClipboard.Call(); r == 0 {
			c.logger.Warn("CloseClipboard failed", "error", e)
		}
	}()

	h, _, _ := getClipboardData.Call(cfHDrop)
	if h == 0 {
		return nil, nil
	}

	ptr, _, _ := globalLock.Call(h)
	if ptr == 0 {
		return nil, fmt.Errorf("failed to lock clipboard memory")
	}
	defer func() {
		if r, _, e := globalUnlock.Call(h); r == 0 {
			c.logger.Warn("GlobalUnlock failed", "error", e)
		}
	}()

	count, _, _ := dragQueryFileW.Call(ptr, 0xFFFFFFFF, 0, 0)

	files := make([]string, 0, count)
	for i := range count {
		size, _, _ := dragQueryFileW.Call(ptr, i, 0, 0)
		buf := make([]uint16, size+1)
		_, _, _ = dragQueryFileW.Call(ptr, i, uintptr(unsafe.Pointer(&buf[0])), size+1) //nolint:unsafeptr
		files = append(files, syscall.UTF16ToString(buf))
	}

	return files, nil
}

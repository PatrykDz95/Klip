//go:build windows

package clipboard

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
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
	mu       sync.Mutex // serializes clipboard access (Get vs Set)
}

func newClipboard(logger *slog.Logger) (Clipboard, error) {
	if logger == nil {
		logger = slog.Default()
	}
	return &windowsClipboard{logger: logger}, nil
}

// openClipboardRetry opens the clipboard, retrying briefly because on Windows 11
// the Clipboard History / Cloud Clipboard services hold the clipboard open for
// short windows whenever its contents change (these are off by default on Win10).
// The caller MUST have the OS thread locked before calling, and must call
// CloseClipboard from the SAME thread.
func openClipboardRetry() error {
	var lastErr error
	for range 10 {
		if r, _, err := openClipboard.Call(0); r != 0 {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("failed to open clipboard: %w", lastErr)
}

// unlockGlobal calls GlobalUnlock and only warns on a genuine failure.
// GlobalUnlock returns 0 both on error AND when the lock count simply reached 0
// (the normal case), so it must be distinguished via GetLastError: a zero errno
// means success. Without this check every successful unlock logs a spurious
// "GlobalUnlock failed: The operation completed successfully." warning.
func (c *windowsClipboard) unlockGlobal(h uintptr) {
	if r, _, e := globalUnlock.Call(h); r == 0 {
		if errno, ok := e.(syscall.Errno); ok && errno != 0 {
			c.logger.Warn("GlobalUnlock failed", "error", e)
		}
	}
}

func (c *windowsClipboard) Get() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// The clipboard is bound to the OS thread that opens it: OpenClipboard,
	// GetClipboardData and CloseClipboard must all run on the same thread.
	// Without this the Go scheduler can migrate the goroutine mid-call, causing
	// "Thread does not have a clipboard open." errors.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := openClipboardRetry(); err != nil {
		return "", err
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
	defer c.unlockGlobal(h)

	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(ptr))), nil //nolint:unsafeptr
}

func (c *windowsClipboard) Set(content string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// All clipboard calls below must run on the same OS thread — see Get().
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := openClipboardRetry(); err != nil {
		return err
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

	c.unlockGlobal(h)

	// On success SetClipboardData transfers ownership of h to the system, so we
	// must NOT free it. On failure ownership stays with us and h would leak
	// unless we free it here.
	r, _, err := setClipboardData.Call(cfUnicodeText, h)
	if r == 0 {
		if r, _, e := globalFree.Call(h); r == 0 {
			c.logger.Warn("GlobalFree failed", "error", e)
		}
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

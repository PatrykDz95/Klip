package app

import (
	"context"
	"fmt"
	"klip/internal/p2p"
	"time"
)

// handles incoming clipboard sync messages
func (app *Application) handleClipboardSync(msg *p2p.Message) {
	if msg.Type != p2p.MsgTypeSync || app.isPaused() {
		return
	}

	content := msg.Payload.ClipboardContent
	if err := app.clipboard.Set(content); err != nil {
		app.logger.Error("Failed to set clipboard", "error", err)
		return
	}

	app.logger.Info("Clipboard synced", "size", formatBytes(len(content)))
}

func (app *Application) startClipboardMonitoring() error {
	return app.clipboard.Watch(func(content string) {
		if app.isPaused() {
			return
		}

		app.logger.Info("Local clipboard changed", "size", formatBytes(len(content)))
		app.p2pMgr.BroadcastClipBoard(content)

		app.cancelClipboardClear()

		if content != "" {
			go app.scheduleClearClipboard(5 * time.Minute)
		}
	})

}

// cancels the scheduled clipboard clear
func (app *Application) cancelClipboardClear() {
	app.clipboardClearMu.Lock()
	defer app.clipboardClearMu.Unlock()

	if app.clipboardClearCancel != nil {
		app.clipboardClearCancel()
		app.clipboardClearCancel = nil
	}
}

func formatBytes(b int) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// clears clipboard after specified duration
func (app *Application) scheduleClearClipboard(delay time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())

	// Store cancel function
	app.clipboardClearMu.Lock()
	app.clipboardClearCancel = cancel
	app.clipboardClearMu.Unlock()

	app.logger.Debug("Scheduled clipboard clear", "delay", delay)

	select {
	case <-time.After(delay):
		if err := app.clipboard.Set(""); err != nil {
			app.logger.Error("Failed to clear clipboard", "error", err)
			return
		}

		app.logger.Info("Clipboard cleared automatically")

	case <-ctx.Done():
		// Cancelled (user changed clipboard before timeout)
		app.logger.Debug("Clipboard clear cancelled")
	}
}

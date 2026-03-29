package app

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"sync"
	"time"

	"klip/internal/p2p"

	"github.com/getlantern/systray"
)

const klipUrl = "https://klip-it.app"

type peerEntry struct {
	item   *systray.MenuItem
	cancel context.CancelFunc
}

type UI struct {
	status        *systray.MenuItem
	devices       *systray.MenuItem
	noDevicesItem *systray.MenuItem
	licenseItem   *systray.MenuItem

	mu    sync.Mutex
	peers map[string]peerEntry
}

func (app *Application) buildMenu() {
	app.ui.status = systray.AddMenuItem("Starting...", "Current status")
	app.ui.status.Disable()
	mLicense := systray.AddMenuItem("", "")
	app.ui.licenseItem = mLicense
	systray.AddSeparator()

	app.ui.devices = systray.AddMenuItem("Devices: 0", "Connected devices")
	systray.AddSeparator()
	mPause := systray.AddMenuItem("Pause clipboard syncing", "Don't sync clipboard with other devices")
	systray.AddSeparator()
	mOpenFiles := systray.AddMenuItem("Open received files", "Open the received files folder")
	systray.AddSeparator()
	mAutostart := systray.AddMenuItem(autostartTitle(), "Start Klip when you log in")
	systray.AddSeparator()
	mAbout := systray.AddMenuItem("About Klip", "About this application")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Exit Klip")

	app.updateLicenseMenu()

	go app.handleMenuClicks(mPause, mOpenFiles, mAutostart, mLicense, mAbout, mQuit)
}

func (app *Application) updateLicenseMenu() {
	if app.isPro() {
		app.ui.licenseItem.SetTitle("⭐ Klip Pro")
		app.ui.licenseItem.SetTooltip("Unlimited devices - Thank you for support!")
	} else {
		app.ui.licenseItem.SetTitle("⚡ Upgrade to Pro")
		app.ui.licenseItem.SetTooltip("Unlock unlimited devices")
	}
}

func (app *Application) handleMenuClicks(mPause, mOpenFiles, mAutostart, mLicense, mAbout, mQuit *systray.MenuItem) {
	for {
		select {
		case <-mPause.ClickedCh:
			app.togglePaused(mPause)
		case <-mOpenFiles.ClickedCh:
			app.openReceivedFilesDir()
		case <-mAutostart.ClickedCh:
			app.toggleAutostart(mAutostart)
		case <-mLicense.ClickedCh:
			if !app.isPro() {
				app.activateLicense()
			}
			app.updateLicenseMenu()
		case <-mAbout.ClickedCh:
			app.showAbout()
		case <-mQuit.ClickedCh:
			systray.Quit()
		}
	}
}

func (app *Application) openReceivedFilesDir() {
	dir := getReceivedFilesDir()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		app.logger.Error("Failed to open received files folder", "error", err)
	}
}

func (app *Application) togglePaused(paused *systray.MenuItem) {
	if app.isPaused() {
		paused.SetTitle("Pause clipboard syncing")
		app.setPaused(false)
	} else {
		paused.SetTitle("Resume clipboard syncing")
		app.setPaused(true)
	}
}

func (app *Application) setPaused(v bool) {
	app.pausedMu.Lock()
	app.paused = v
	app.pausedMu.Unlock()
}

func (app *Application) isPaused() bool {
	app.pausedMu.RLock()
	defer app.pausedMu.RUnlock()
	return app.paused
}

func (app *Application) isPro() bool {
	app.licenseMu.RLock()
	defer app.licenseMu.RUnlock()
	return app.license != nil
}

func (app *Application) updateStatus(status string) {
	if app.ui.status != nil {
		app.ui.status.SetTitle(status)
		app.ui.status.Show()
	}
}

func (app *Application) hideStatusAfter(seconds time.Duration) {
	time.Sleep(seconds * time.Second)
	if app.ui.status != nil {
		app.ui.status.Hide()
	}
}

func (app *Application) updatePeerMenu() {
	if app.ui.devices == nil || app.p2pMgr == nil {
		return
	}

	peers := app.p2pMgr.GetPeers()
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].DeviceID < peers[j].DeviceID
	})

	app.ui.mu.Lock()
	defer app.ui.mu.Unlock()

	app.removeDisconnectedPeers(peers)
	app.addNewPeers(peers)
	app.syncNoDevicesPlaceholder(len(peers))
	app.updateDevicesTitle(len(peers))
}

// removeDisconnectedPeers hides menu items for peers that are no longer connected.
func (app *Application) removeDisconnectedPeers(connected []p2p.PeerInfo) {
	connectedSet := make(map[string]struct{}, len(connected))
	for _, p := range connected {
		connectedSet[p.DeviceID] = struct{}{}
	}

	for id, entry := range app.ui.peers {
		if _, ok := connectedSet[id]; !ok {
			entry.cancel()
			entry.item.Hide()
			delete(app.ui.peers, id)
		}
	}
}

// addNewPeers adds menu items for peers not yet in the menu.
func (app *Application) addNewPeers(peers []p2p.PeerInfo) {
	for _, peer := range peers {
		if _, exists := app.ui.peers[peer.DeviceID]; exists {
			continue
		}

		tooltip := fmt.Sprintf("Send file to %s (%s)", peer.DeviceName, peer.Address)
		item := app.ui.devices.AddSubMenuItem(peer.DeviceName, tooltip)

		ctx, cancel := context.WithCancel(context.Background())
		app.ui.peers[peer.DeviceID] = peerEntry{item: item, cancel: cancel}

		go app.handlePeerClick(ctx, peer.DeviceID, item)
	}
}

// syncNoDevicesPlaceholder shows or hides the "No devices found" placeholder.
func (app *Application) syncNoDevicesPlaceholder(peerCount int) {
	if peerCount == 0 && app.ui.noDevicesItem == nil {
		item := app.ui.devices.AddSubMenuItem("No devices found", "")
		item.Disable()
		app.ui.noDevicesItem = item
	} else if peerCount > 0 && app.ui.noDevicesItem != nil {
		app.ui.noDevicesItem.Hide()
		app.ui.noDevicesItem = nil
	}
}

func (app *Application) updateDevicesTitle(count int) {
	var title, tooltip string
	switch count {
	case 0:
		title = "Devices: 0 (searching...)"
		tooltip = "Klip - Searching for devices"
	case 1:
		title = "Devices: 1 connected"
		tooltip = "Klip - 1 device connected"
	default:
		title = fmt.Sprintf("Devices: %d connected", count)
		tooltip = fmt.Sprintf("Klip - %d devices connected", count)
	}
	app.ui.devices.SetTitle(title)
	systray.SetTooltip(tooltip)
}

func (app *Application) handlePeerClick(ctx context.Context, deviceID string, item *systray.MenuItem) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-item.ClickedCh:
			app.logger.Info("Peer clicked", "device_id", deviceID)
			app.sendFileToDevice(deviceID)
		}
	}
}

func (app *Application) toggleAutostart(item *systray.MenuItem) {
	var err error
	if autostartEnabled() {
		err = disableAutostart()
	} else {
		err = enableAutostart()
	}
	if err != nil {
		app.logger.Error("Failed to toggle autostart", "error", err)
		return
	}
	item.SetTitle(autostartTitle())
}

func autostartTitle() string {
	if autostartEnabled() {
		return "✓ Launch at startup"
	}
	return "Launch at startup"
}

func (app *Application) showAbout() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", klipUrl)
	case "darwin":
		cmd = exec.Command("open", klipUrl)
	default:
		cmd = exec.Command("xdg-open", klipUrl)
	}
	if err := cmd.Start(); err != nil {
		app.logger.Error("Failed to open browser", "error", err)
	}
}

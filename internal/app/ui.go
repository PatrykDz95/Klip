package app

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"klip/internal/p2p"

	"github.com/getlantern/systray"
)

type peerEntry struct {
	item   *systray.MenuItem
	cancel context.CancelFunc
}

type UI struct {
	status        *systray.MenuItem
	devices       *systray.MenuItem
	noDevicesItem *systray.MenuItem

	mu    sync.Mutex
	peers map[string]peerEntry
}

func (app *Application) buildMenu() {
	app.ui.status = systray.AddMenuItem("Starting...", "Current status")
	app.ui.status.Disable()

	app.ui.devices = systray.AddMenuItem("Devices: 0", "Connected devices")
	systray.AddSeparator()
	paused := systray.AddMenuItem("Pause syncing", "Pause clipboard syncing")
	systray.AddSeparator()
	mAbout := systray.AddMenuItem("About Klip", "About this application")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Exit Klip")

	go app.handleMenuClicks(paused, mAbout, mQuit)
}

func (app *Application) handleMenuClicks(paused, mAbout, mQuit *systray.MenuItem) {
	for {
		select {
		case <-paused.ClickedCh:
			app.togglePaused(paused)
		case <-mAbout.ClickedCh:
			app.showAbout()
		case <-mQuit.ClickedCh:
			systray.Quit()
		}
	}
}

func (app *Application) togglePaused(paused *systray.MenuItem) {
	if app.isPaused() {
		paused.SetTitle("Pause syncing")
		app.setPaused(false)
	} else {
		paused.SetTitle("Resume syncing")
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

func (app *Application) updateStatus(status string) {
	if app.ui.status != nil {
		app.ui.status.SetTitle(status)
	}
}

func (app *Application) hideStatus() {
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

// TODO: implement showing maybe a window with info about the app instead of just printing to console
func (app *Application) showAbout() {
	app.logger.Info("About clicked")
	fmt.Printf("Klip Secure P2P clipboard sync\n")
}

package app

import "github.com/sqweek/dialog"

func (app *Application) handlePeerDiscovered(peerID, addr string) {
	if app.p2pMgr.HasPeer(peerID) {
		app.logger.Debug("Discovered peer (already connected)", "peer_id", peerID)
		return
	}

	if !app.isPro() && len(app.p2pMgr.GetPeers()) >= maxFreeDevices {
		app.logger.Warn("Free version limited to 2 devices")
		app.setDeviceLimitBlocked(true)
		app.updateStatus("Free limit reached - Upgrade to Pro")
		dialog.Message("You've reached the free limit of 2 devices.\n\nUpgrade to Klip Pro for unlimited devices.").
			Title("Klip - Device Limit Reached").Info()
		return
	}

	app.logger.Info("Discovered new peer", "peer_id", peerID, "addr", addr)

	if err := app.p2pMgr.Connect(peerID, addr); err != nil {
		app.logger.Error("Failed to connect to peer", "peer", peerID, "error", err)
		return
	}

	app.updatePeerMenu()
}

func (app *Application) setDeviceLimitBlocked(v bool) {
	app.deviceLimitMu.Lock()
	app.deviceLimitBlocked = v
	app.deviceLimitMu.Unlock()
}

func (app *Application) isDeviceLimitBlocked() bool {
	app.deviceLimitMu.RLock()
	defer app.deviceLimitMu.RUnlock()
	return app.deviceLimitBlocked
}

func (app *Application) connectToManualPeer(addr string) {
	app.logger.Info("Connecting to manual peer", "addr", addr)

	if err := app.p2pMgr.Connect("manual-peer", addr); err != nil {
		app.logger.Error("Failed to connect to manual peer", "error", err)
	}
}

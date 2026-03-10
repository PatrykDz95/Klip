package app

func (app *Application) handlePeerDiscovered(peerID, addr string) {
	if app.p2pMgr.HasPeer(peerID) {
		app.logger.Debug("Discovered peer (already connected)", "peer_id", peerID)
		return
	}

	app.logger.Info("Discovered new peer", "peer_id", peerID, "addr", addr)

	if err := app.p2pMgr.Connect(peerID, addr); err != nil {
		app.logger.Error("Failed to connect to peer", "peer", peerID, "error", err)
		return
	}

	app.updatePeerMenu()
}

func (app *Application) connectToManualPeer(addr string) {
	app.logger.Info("Connecting to manual peer", "addr", addr)

	if err := app.p2pMgr.Connect("manual-peer", addr); err != nil {
		app.logger.Error("Failed to connect to manual peer", "error", err)
	}
}

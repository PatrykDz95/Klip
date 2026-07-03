package app

import (
	"fmt"
	"klip/internal/p2p"

	"github.com/sqweek/dialog"
)

func (app *Application) handlePeerDiscovered(peerID, addr string) {
	if app.p2pMgr.HasPeer(peerID) {
		app.logger.Debug("Discovered peer (already connected)", "peer_id", peerID)
		return
	}

	if !app.isPro() && len(app.p2pMgr.GetPeers()) >= maxFreeDevices {
		app.logger.Warn("Free version limited to 2 devices")
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

// isDeviceLimitBlocked reports whether the free-tier limit is currently exceeded.
// Computed live from the connected peer count so it self-corrects when a peer
func (app *Application) isDeviceLimitBlocked() bool {
	return !app.isPro() && len(app.p2pMgr.GetPeers()) > maxFreeDevices
}

func (app *Application) connectToManualPeer(addr string) {
	app.logger.Info("Connecting to manual peer", "addr", addr)

	if err := app.p2pMgr.Connect("manual-peer", addr); err != nil {
		app.logger.Error("Failed to connect to manual peer", "error", err)
	}
}

func (app *Application) confirmPeerTrustDecision(decision p2p.PeerTrustDecision) bool {
	deviceName := decision.DeviceName
	if deviceName == "" {
		deviceName = "Unknown device"
	}

	question := fmt.Sprintf(
		"%s (%s) changed its security identity.\n\nTrusted fingerprint:\n%s\n\nNew fingerprint:\n%s\n\nTrust this device again?",
		deviceName,
		decision.DeviceID,
		decision.TrustedFingerprint,
		decision.PeerFingerprint,
	)

	trusted := dialog.Message("%s", question).Title("Klip Security Warning").YesNo()
	if !trusted {
		app.logger.Warn("Rejected peer with changed fingerprint", "device_id", decision.DeviceID)
	}

	return trusted
}

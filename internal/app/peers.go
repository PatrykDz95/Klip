package app

import (
	"fmt"
	"klip/internal/p2p"
	"time"

	"github.com/sqweek/dialog"
)

const (
	dialBackoffInitial = 5 * time.Second
	dialBackoffMax     = 60 * time.Second
)

// dialAttempt tracks retry backoff for a peer we've failed to reach, so a
// powered-off peer isn't dialed on every discovery event.
type dialAttempt struct {
	nextAttempt time.Time
	backoff     time.Duration
	logged      bool // whether the first failure has already been logged at Error
}

func (app *Application) handlePeerDiscovered(peerID, addr string) {
	if app.p2pMgr.HasPeer(peerID) {
		app.logger.Debug("Discovered peer (already connected)", "peer_id", peerID)
		// Clear any stale backoff so a later disconnect reconnects promptly.
		app.recordDialSuccess(peerID)
		return
	}

	if !app.isPro() && len(app.p2pMgr.GetPeers()) >= maxFreeDevices {
		app.logger.Warn("Free version limited to 2 devices")
		app.updateStatus("Free limit reached - Upgrade to Pro")
		dialog.Message("You've reached the free limit of 2 devices.\n\nUpgrade to Klip Pro for unlimited devices.").
			Title("Klip - Device Limit Reached").Info()
		return
	}

	if !app.shouldAttemptDial(peerID) {
		app.logger.Debug("Skipping peer dial (backoff active)", "peer_id", peerID)
		return
	}

	app.logger.Info("Discovered new peer", "peer_id", peerID, "addr", addr)

	if err := app.p2pMgr.Connect(peerID, addr); err != nil {
		app.recordDialFailure(peerID, err)
		return
	}

	app.recordDialSuccess(peerID)
	app.updatePeerMenu()
}

// shouldAttemptDial reports whether enough time has passed since the last failed
// dial to this peer to try again (or if we've never failed).
func (app *Application) shouldAttemptDial(peerID string) bool {
	app.dialBackoffMu.Lock()
	defer app.dialBackoffMu.Unlock()
	att, ok := app.dialBackoff[peerID]
	return !ok || time.Now().After(att.nextAttempt)
}

// recordDialFailure grows the exponential backoff for a peer and logs the first
// failure at Error, subsequent ones at Debug to keep logs clean.
func (app *Application) recordDialFailure(peerID string, err error) {
	app.dialBackoffMu.Lock()
	att, ok := app.dialBackoff[peerID]
	if !ok {
		att = &dialAttempt{backoff: dialBackoffInitial}
		app.dialBackoff[peerID] = att
	} else if att.backoff *= 2; att.backoff > dialBackoffMax {
		att.backoff = dialBackoffMax
	}
	att.nextAttempt = time.Now().Add(att.backoff)
	first, retryIn := !att.logged, att.backoff
	att.logged = true
	app.dialBackoffMu.Unlock()

	if first {
		app.logger.Error("Failed to connect to peer", "peer", peerID, "error", err)
	} else {
		app.logger.Debug("Failed to connect to peer (will retry)",
			"peer", peerID, "error", err, "retry_in", retryIn)
	}
}

// recordDialSuccess clears any backoff state for a peer.
func (app *Application) recordDialSuccess(peerID string) {
	app.dialBackoffMu.Lock()
	defer app.dialBackoffMu.Unlock()

	delete(app.dialBackoff, peerID)
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

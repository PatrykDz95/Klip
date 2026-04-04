package app

import "klip/internal/p2p"

type p2pEventHandler struct {
	app *Application
}

func newP2PEventHandler(app *Application) p2p.EventHandler {
	return &p2pEventHandler{app: app}
}

func (h *p2pEventHandler) OnMessage(msg *p2p.Message) {
	h.app.handleClipboardSync(msg)
}

func (h *p2pEventHandler) OnFileReceive(senderName, fileName string, fileSize int64) (bool, string) {
	return h.app.handleIncomingFile(senderName, fileName, fileSize)
}

func (h *p2pEventHandler) OnPeerTrustDecision(decision p2p.PeerTrustDecision) bool {
	return h.app.confirmPeerTrustDecision(decision)
}

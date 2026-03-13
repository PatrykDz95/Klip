package p2p

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

func (m *Manager) BroadcastClipBoard(content string) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))

	m.mu.Lock()
	if hash == m.lastHash {
		m.mu.Unlock()
		return
	}
	m.lastHash = hash
	m.mu.Unlock()

	msg := &Message{
		Type:      MsgTypeSync,
		DeviceID:  m.deviceID,
		Timestamp: time.Now(),
		Payload: &Payload{
			ClipboardContent: content,
			ContentHash:      hash,
		},
	}

	m.mu.RLock()
	peerCount := len(m.peers)
	for _, peer := range m.peers {
		go func(p *Peer) {
			if err := json.NewEncoder(p.Conn).Encode(msg); err != nil {
				m.logger.Error("Broadcast failed", "peer", p.DeviceName, "error", err)
			}
		}(peer)
	}
	m.mu.RUnlock()

	if peerCount > 0 {
		m.logger.Debug("Clipboard broadcasted",
			"peer_count", peerCount,
			"size", len(content),
		)
	}
}

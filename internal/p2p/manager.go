package p2p

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	deviceID   string
	deviceName string
	port       int
	peers      map[string]*Peer
	mu         sync.RWMutex
	lastHash   string
	serverTLS  *tls.Config
	clientTLS  *tls.Config
	logger     *slog.Logger
	listener   net.Listener
	listenerMu sync.Mutex
	events     EventHandler

	Progress chan FileProgress

	trustStore *peerTrustStore
}

type FileReceiveCallback func(senderName, fileName string, fileSize int64) (bool, string)

type EventHandler interface {
	OnMessage(msg *Message)
	OnFileReceive(senderName, fileName string, fileSize int64) (bool, string)
	OnPeerTrustDecision(decision PeerTrustDecision) bool
}

type PeerTrustDecision struct {
	DeviceID           string
	DeviceName         string
	TrustedFingerprint string
	PeerFingerprint    string
}

func NewManager(deviceID, deviceName string, port int, cert *tls.Certificate, logger *slog.Logger, events EventHandler) *Manager {
	if logger == nil {
		logger = slog.Default()
	}

	trustStore, err := newPeerTrustStore()
	if err != nil {
		logger.Warn("Failed to initialize peer trust store, using in-memory trust only", "error", err)
		trustStore = &peerTrustStore{peers: make(map[string]trustedPeerRecord)}
	}

	return &Manager{
		deviceID:   deviceID,
		deviceName: deviceName,
		port:       port,
		peers:      make(map[string]*Peer),
		serverTLS: &tls.Config{
			Certificates: []tls.Certificate{*cert},
			ClientAuth:   tls.RequireAnyClientCert,
			MinVersion:   tls.VersionTLS13,
		},
		clientTLS: &tls.Config{
			Certificates:       []tls.Certificate{*cert},
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS13,
		},
		logger:     logger,
		events:     events,
		Progress:   make(chan FileProgress, 10),
		trustStore: trustStore,
	}
}

func (m *Manager) HasPeer(deviceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.peers[deviceID]
	return exists
}

func (m *Manager) GetPeers() []PeerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peers := make([]PeerInfo, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, peer.Info())
	}
	return peers
}

// Listen starts the TLS listener for incoming connections.
func (m *Manager) Listen() error {
	listener, err := tls.Listen("tcp", fmt.Sprintf(":%d", m.port), m.serverTLS)
	if err != nil {
		return fmt.Errorf("failed to start TLS listener: %w", err)
	}

	m.listenerMu.Lock()
	m.listener = listener
	m.listenerMu.Unlock()

	m.logger.Info("TLS listener started", "port", m.port)

	go m.acceptLoop(listener)
	return nil
}

func (m *Manager) acceptLoop(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			m.logger.Error("Accept error", "error", err)
			continue
		}

		tlsConn := conn.(*tls.Conn)

		peerFingerprint, err := peerFingerprintSHA256(tlsConn)
		if err != nil {
			m.logger.Warn("Rejecting connection without valid peer certificate", "remote", conn.RemoteAddr(), "error", err)
			if closeErr := conn.Close(); closeErr != nil {
				m.logger.Debug("Failed to close rejected connection", "error", closeErr)
			}
			continue
		}

		m.logger.Debug("TLS connection accepted",
			"remote", conn.RemoteAddr(),
			"fingerprint", shortFingerprint(peerFingerprint),
			"cipher", cipherSuiteName(tlsConn.ConnectionState().CipherSuite),
		)

		go func() {
			if err := m.handleConnection(tlsConn, false, peerFingerprint); err != nil {
				m.logger.Error("Connection handling error", "error", err)
			}
		}()
	}
}

// Connect establishes a TLS connection to a peer.
func (m *Manager) Connect(deviceID, address string) error {
	if m.HasPeer(deviceID) {
		return nil
	}

	dialer := &net.Dialer{Timeout: 3 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, m.clientTLS)
	if err != nil {
		return fmt.Errorf("TLS dial failed: %w", err)
	}
	defer func(conn *tls.Conn) {
		err := conn.Close()
		if err != nil {
			m.logger.Debug("Failed to close connection", "error", err)
		}
	}(conn)

	state := conn.ConnectionState()

	peerFingerprint, err := peerFingerprintSHA256(conn)
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			m.logger.Debug("Failed to close rejected connection", "error", closeErr)
		}
		return fmt.Errorf("failed to get peer certificate fingerprint: %w", err)
	}

	m.logger.Info("TLS connection established",
		"peer", deviceID,
		"tls_version", tlsVersionName(state.Version),
		"fingerprint", shortFingerprint(peerFingerprint),
		"cipher", cipherSuiteName(state.CipherSuite),
	)

	return m.handleConnection(conn, true, peerFingerprint)
}

func (m *Manager) handleConnection(conn net.Conn, initiator bool, peerFingerprint string) error {
	defer func() {
		if err := conn.Close(); err != nil {
			m.logger.Debug("Failed to close connection", "error", err)
		}
	}()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	if initiator {
		if err := encoder.Encode(m.newHelloMessage()); err != nil {
			return fmt.Errorf("failed to send hello: %w", err)
		}
	}

	var msg Message
	if err := decoder.Decode(&msg); err != nil {
		return fmt.Errorf("failed to decode message: %w", err)
	}

	if msg.DeviceID == "" {
		return fmt.Errorf("invalid message: missing device ID")
	}

	deviceName := ""
	if msg.Payload != nil {
		deviceName = msg.Payload.DeviceName
	}

	if err := m.verifyPeerTrust(msg.DeviceID, deviceName, peerFingerprint); err != nil {
		return err
	}

	switch msg.Type {
	case MsgTypeFileOffer:
		return m.handleFileOffer(conn, &msg)
	case MsgTypeHello:
		return m.handlePeerSession(conn, &msg, encoder, decoder, initiator)
	default:
		return fmt.Errorf("unexpected message type: %s", msg.Type)
	}
}

func (m *Manager) verifyPeerTrust(deviceID, deviceName, fingerprint string) error {
	if m.trustStore == nil {
		return nil
	}

	rec, exists := m.trustStore.Get(deviceID)
	if !exists {
		if err := m.trustStore.Set(deviceID, deviceName, fingerprint); err != nil {
			return fmt.Errorf("failed to store trusted fingerprint: %w", err)
		}
		m.logger.Info("Trusted new peer", "device_id", deviceID, "fingerprint", shortFingerprint(fingerprint))
		return nil
	}

	if rec.Fingerprint == fingerprint {
		if err := m.trustStore.Touch(deviceID, deviceName); err != nil {
			m.logger.Warn("Failed to update trust record timestamp", "device_id", deviceID, "error", err)
		}
		return nil
	}

	decision := PeerTrustDecision{
		DeviceID:           deviceID,
		DeviceName:         deviceName,
		TrustedFingerprint: rec.Fingerprint,
		PeerFingerprint:    fingerprint,
	}

	if m.events == nil || !m.events.OnPeerTrustDecision(decision) {
		return fmt.Errorf("peer identity mismatch for %s", deviceID)
	}

	if err := m.trustStore.Set(deviceID, deviceName, fingerprint); err != nil {
		return fmt.Errorf("failed to update trusted fingerprint: %w", err)
	}

	m.logger.Warn("Peer trust updated after fingerprint change", "device_id", deviceID, "fingerprint", shortFingerprint(fingerprint))
	return nil
}

func (m *Manager) handlePeerSession(conn net.Conn, msg *Message, encoder *json.Encoder, decoder *json.Decoder, initiator bool) error {
	if msg.DeviceID == "" || msg.Payload == nil || msg.Payload.DeviceName == "" {
		return fmt.Errorf("invalid hello: missing required fields")
	}

	host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return fmt.Errorf("failed to parse remote address: %w", err)
	}

	listenPort := msg.Payload.ListenPort
	if listenPort == 0 {
		listenPort = m.port
	}

	peer := &Peer{
		DeviceID:   msg.DeviceID,
		DeviceName: msg.Payload.DeviceName,
		OS:         msg.Payload.OS,
		Address:    fmt.Sprintf("%s:%d", host, listenPort),
		Conn:       conn,
		LastSeen:   time.Now(),
	}

	m.mu.Lock()
	m.peers[msg.DeviceID] = peer
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.peers, peer.DeviceID)
		m.mu.Unlock()
	}()

	m.logger.Info("Peer connected",
		"device_name", peer.DeviceName,
		"device_id", peer.DeviceID,
		"os", peer.OS,
	)

	if !initiator {
		if err := encoder.Encode(m.newHelloMessage()); err != nil {
			return fmt.Errorf("failed to send hello response: %w", err)
		}
	}

	for {
		var mMsg Message
		if err := decoder.Decode(&mMsg); err != nil {
			m.logger.Debug("Peer disconnected", "peer", peer.DeviceName, "error", err)
			return nil
		}
		if mMsg.Type == MsgTypeSync && m.events != nil {
			m.events.OnMessage(&mMsg)
		}
	}
}

func (m *Manager) newHelloMessage() *Message {
	return &Message{
		Type:      MsgTypeHello,
		DeviceID:  m.deviceID,
		Timestamp: time.Now(),
		Payload: &Payload{
			ListenPort: m.port,
			DeviceName: m.deviceName,
			OS:         runtime.GOOS,
		},
	}
}

// For incoming connections, ensure the handshake has already occurred.
// For tls.DialWithDialer, the handshake is typically done immediately.
func peerFingerprintSHA256(tlsConn *tls.Conn) (string, error) {
	if err := tlsConn.Handshake(); err != nil {
		return "", fmt.Errorf("TLS handshake failed: %w", err)
	}

	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return "", fmt.Errorf("peer certificate not provided")
	}

	sum := sha256.Sum256(state.PeerCertificates[0].Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:])), nil
}

func shortFingerprint(fp string) string {
	if len(fp) <= 16 {
		return fp
	}
	return fp[:16]
}

func (m *Manager) getSenderName(deviceID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if peer, exists := m.peers[deviceID]; exists {
		return peer.DeviceName
	}
	return "Unknown"
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	default:
		return fmt.Sprintf("TLS 0x%04x", version)
	}
}

func cipherSuiteName(suite uint16) string {
	switch suite {
	case tls.TLS_AES_128_GCM_SHA256:
		return "AES-128-GCM"
	case tls.TLS_AES_256_GCM_SHA384:
		return "AES-256-GCM"
	case tls.TLS_CHACHA20_POLY1305_SHA256:
		return "CHACHA20-POLY1305"
	default:
		return fmt.Sprintf("0x%04x", suite)
	}
}

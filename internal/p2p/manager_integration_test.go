package p2p

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"klip/internal/security"
)

type integrationEventHandler struct {
	mu             sync.Mutex
	decisionCalls  int
	decisionResult bool
}

func (h *integrationEventHandler) OnMessage(_ *Message) {}

func (h *integrationEventHandler) OnFileReceive(_, _ string, _ int64) (bool, string) {
	return false, ""
}

func (h *integrationEventHandler) OnPeerTrustDecision(_ PeerTrustDecision) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.decisionCalls++
	return h.decisionResult
}

func (h *integrationEventHandler) DecisionCalls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.decisionCalls
}

func newIntegrationManager(t *testing.T, certRoot, deviceID, deviceName string, events EventHandler) *Manager {
	t.Helper()

	certDir := filepath.Join(certRoot, deviceID)
	cert, err := security.GenerateSelfSignedCert(certDir, deviceID)
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert failed: %v", err)
	}

	port := freePort(t)
	mgr := NewManager(deviceID, deviceName, port, cert, slog.New(slog.NewTextHandler(io.Discard, nil)), events)
	if err := mgr.Listen(); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	t.Cleanup(func() {
		shutdownManager(mgr)
	})

	return mgr
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate free port: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func managerAddr(m *Manager) string {
	return fmt.Sprintf("127.0.0.1:%d", m.port)
}

func shutdownManager(m *Manager) {
	if m == nil {
		return
	}

	m.listenerMu.Lock()
	listener := m.listener
	m.listenerMu.Unlock()
	if listener != nil {
		_ = listener.Close()
	}

	m.mu.RLock()
	peers := make([]*Peer, 0, len(m.peers))
	for _, p := range m.peers {
		peers = append(peers, p)
	}
	m.mu.RUnlock()
	for _, p := range peers {
		_ = p.Conn.Close()
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout: %s", msg)
}

func connectAsync(m *Manager, peerID, addr string) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- m.Connect(peerID, addr)
	}()
	return ch
}

func TestManagerTOFUPersistsPeerFingerprintOnFirstConnect(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	certRoot := filepath.Join(t.TempDir(), "certs")
	hA := &integrationEventHandler{}
	hB := &integrationEventHandler{}

	mA := newIntegrationManager(t, certRoot, "dev-a", "Device A", hA)
	mB := newIntegrationManager(t, certRoot, "dev-b", "Device B", hB)

	errCh := connectAsync(mA, "dev-b", managerAddr(mB))

	waitFor(t, 2*time.Second, func() bool {
		_, ok := mA.trustStore.Get("dev-b")
		return ok
	}, "manager A should trust dev-b")

	waitFor(t, 2*time.Second, func() bool {
		_, ok := mB.trustStore.Get("dev-a")
		return ok
	}, "manager B should trust dev-a")

	if hA.DecisionCalls() != 0 || hB.DecisionCalls() != 0 {
		t.Fatalf("unexpected trust decision prompts on first connection")
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected connect error: %v", err)
		}
	default:
	}
}

func TestManagerRejectsPeerOnFingerprintMismatch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	certRoot := filepath.Join(t.TempDir(), "certs")
	hA := &integrationEventHandler{decisionResult: false}
	hB := &integrationEventHandler{}

	mA := newIntegrationManager(t, certRoot, "dev-a", "Device A", hA)
	mB := newIntegrationManager(t, certRoot, "dev-b", "Device B", hB)

	connectAsync(mA, "dev-b", managerAddr(mB))

	waitFor(t, 2*time.Second, func() bool {
		rec, ok := mA.trustStore.Get("dev-b")
		return ok && rec.Fingerprint != ""
	}, "manager A should store initial fingerprint")

	oldRec, _ := mA.trustStore.Get("dev-b")

	shutdownManager(mA)
	shutdownManager(mB)

	// Recreate dev-b with a new certificate and same logical device ID.
	mA = newIntegrationManager(t, certRoot, "dev-a", "Device A", hA)
	mB2 := newIntegrationManager(t, filepath.Join(t.TempDir(), "new-certs"), "dev-b", "Device B", hB)

	// Restore manager A trust store state from the same HOME persisted file.
	waitFor(t, 2*time.Second, func() bool {
		rec, ok := mA.trustStore.Get("dev-b")
		return ok && rec.Fingerprint == oldRec.Fingerprint
	}, "manager A should load existing trust entry")

	errCh := connectAsync(mA, "dev-b", managerAddr(mB2))
	var err error
	select {
	case err = <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected mismatch connect to fail quickly")
	}
	if err == nil {
		t.Fatalf("expected connect to fail on fingerprint mismatch")
	}
	if !strings.Contains(err.Error(), "peer identity mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool { return hA.DecisionCalls() > 0 }, "expected trust decision prompt on mismatch")

	rec, _ := mA.trustStore.Get("dev-b")
	if rec.Fingerprint != oldRec.Fingerprint {
		t.Fatalf("fingerprint should not change after rejection")
	}
}

func TestManagerAcceptsPeerOnFingerprintMismatchWhenApproved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	certRoot := filepath.Join(t.TempDir(), "certs")
	hA := &integrationEventHandler{decisionResult: true}
	hB := &integrationEventHandler{}

	mA := newIntegrationManager(t, certRoot, "dev-a", "Device A", hA)
	mB := newIntegrationManager(t, certRoot, "dev-b", "Device B", hB)

	connectAsync(mA, "dev-b", managerAddr(mB))

	waitFor(t, 2*time.Second, func() bool {
		rec, ok := mA.trustStore.Get("dev-b")
		return ok && rec.Fingerprint != ""
	}, "manager A should store initial fingerprint")
	oldRec, _ := mA.trustStore.Get("dev-b")

	shutdownManager(mA)
	shutdownManager(mB)

	mA = newIntegrationManager(t, certRoot, "dev-a", "Device A", hA)
	mB2 := newIntegrationManager(t, filepath.Join(t.TempDir(), "new-certs"), "dev-b", "Device B", hB)

	errCh := connectAsync(mA, "dev-b", managerAddr(mB2))
	waitFor(t, 2*time.Second, func() bool { return hA.DecisionCalls() > 0 }, "expected trust decision prompt on mismatch")

	waitFor(t, 2*time.Second, func() bool {
		rec, ok := mA.trustStore.Get("dev-b")
		return ok && rec.Fingerprint != oldRec.Fingerprint
	}, "fingerprint should update after approval")

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected connect error after approval: %v", err)
		}
	default:
	}
}

package p2p

import (
	"io"
	"log/slog"
	"strings"
	"testing"
)

type testEventHandler struct {
	decisionCalled bool
	decisionResult bool
}

func (h *testEventHandler) OnMessage(_ *Message) {}

func (h *testEventHandler) OnFileReceive(_, _ string, _ int64) (bool, string) {
	return false, ""
}

func (h *testEventHandler) OnPeerTrustDecision(_ PeerTrustDecision) bool {
	h.decisionCalled = true
	return h.decisionResult
}

func newTestManagerForTrust(t *testing.T, events EventHandler) *Manager {
	t.Helper()
	store := newTestTrustStore(t)
	return &Manager{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		trustStore: store,
		events:     events,
	}
}

func TestVerifyPeerTrustFirstContactStoresFingerprint(t *testing.T) {
	m := newTestManagerForTrust(t, nil)

	if err := m.verifyPeerTrust("dev-1", "MacBook", "FP-A"); err != nil {
		t.Fatalf("verifyPeerTrust failed: %v", err)
	}

	rec, ok := m.trustStore.Get("dev-1")
	if !ok {
		t.Fatalf("expected trust record to be created")
	}
	if rec.Fingerprint != "FP-A" {
		t.Fatalf("expected FP-A, got %q", rec.Fingerprint)
	}
}

func TestVerifyPeerTrustKnownFingerprintDoesNotPrompt(t *testing.T) {
	h := &testEventHandler{decisionResult: true}
	m := newTestManagerForTrust(t, h)

	if err := m.trustStore.Set("dev-1", "MacBook", "FP-A"); err != nil {
		t.Fatalf("failed to seed trust store: %v", err)
	}

	if err := m.verifyPeerTrust("dev-1", "MacBook", "FP-A"); err != nil {
		t.Fatalf("verifyPeerTrust failed: %v", err)
	}
	if h.decisionCalled {
		t.Fatalf("did not expect trust decision callback for matching fingerprint")
	}
}

func TestVerifyPeerTrustMismatchWithoutHandlerFails(t *testing.T) {
	m := newTestManagerForTrust(t, nil)
	if err := m.trustStore.Set("dev-1", "MacBook", "FP-A"); err != nil {
		t.Fatalf("failed to seed trust store: %v", err)
	}

	err := m.verifyPeerTrust("dev-1", "MacBook", "FP-B")
	if err == nil {
		t.Fatalf("expected mismatch to fail without decision handler")
	}
	if !strings.Contains(err.Error(), "peer identity mismatch") {
		t.Fatalf("expected mismatch error, got: %v", err)
	}

	rec, _ := m.trustStore.Get("dev-1")
	if rec.Fingerprint != "FP-A" {
		t.Fatalf("fingerprint should stay unchanged on rejection")
	}
}

func TestVerifyPeerTrustMismatchRejectedByUser(t *testing.T) {
	h := &testEventHandler{decisionResult: false}
	m := newTestManagerForTrust(t, h)
	if err := m.trustStore.Set("dev-1", "MacBook", "FP-A"); err != nil {
		t.Fatalf("failed to seed trust store: %v", err)
	}

	err := m.verifyPeerTrust("dev-1", "MacBook", "FP-B")
	if err == nil {
		t.Fatalf("expected mismatch to fail when user rejects")
	}
	if !h.decisionCalled {
		t.Fatalf("expected decision callback to be called")
	}

	rec, _ := m.trustStore.Get("dev-1")
	if rec.Fingerprint != "FP-A" {
		t.Fatalf("fingerprint should stay unchanged after rejection")
	}
}

func TestVerifyPeerTrustMismatchAcceptedUpdatesFingerprint(t *testing.T) {
	h := &testEventHandler{decisionResult: true}
	m := newTestManagerForTrust(t, h)
	if err := m.trustStore.Set("dev-1", "MacBook", "FP-A"); err != nil {
		t.Fatalf("failed to seed trust store: %v", err)
	}

	if err := m.verifyPeerTrust("dev-1", "MacBook", "FP-B"); err != nil {
		t.Fatalf("expected mismatch acceptance to pass, got %v", err)
	}
	if !h.decisionCalled {
		t.Fatalf("expected decision callback to be called")
	}

	rec, _ := m.trustStore.Get("dev-1")
	if rec.Fingerprint != "FP-B" {
		t.Fatalf("expected fingerprint to be updated to FP-B, got %q", rec.Fingerprint)
	}
}

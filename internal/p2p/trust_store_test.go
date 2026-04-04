package p2p

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestTrustStore(t *testing.T) *peerTrustStore {
	t.Helper()
	return &peerTrustStore{
		path:  filepath.Join(t.TempDir(), "trusted_peers.json"),
		peers: make(map[string]trustedPeerRecord),
	}
}

func TestPeerTrustStoreSetGetLoadAndTouch(t *testing.T) {
	store := newTestTrustStore(t)

	if err := store.Set("dev-1", "MacBook", "FP-ONE"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	rec, ok := store.Get("dev-1")
	if !ok {
		t.Fatalf("expected record to exist")
	}
	if rec.Fingerprint != "FP-ONE" {
		t.Fatalf("unexpected fingerprint: got %q", rec.Fingerprint)
	}
	if rec.FirstSeen.IsZero() || rec.LastSeen.IsZero() {
		t.Fatalf("expected first/last seen timestamps to be set")
	}

	loaded := &peerTrustStore{path: store.path, peers: make(map[string]trustedPeerRecord)}
	if err := loaded.load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	loadedRec, ok := loaded.Get("dev-1")
	if !ok {
		t.Fatalf("expected loaded record to exist")
	}
	if loadedRec.Fingerprint != "FP-ONE" {
		t.Fatalf("unexpected loaded fingerprint: got %q", loadedRec.Fingerprint)
	}

	prevLastSeen := loadedRec.LastSeen
	time.Sleep(10 * time.Millisecond)
	if err := loaded.Touch("dev-1", "Office-MacBook"); err != nil {
		t.Fatalf("touch failed: %v", err)
	}

	touched, ok := loaded.Get("dev-1")
	if !ok {
		t.Fatalf("expected touched record to exist")
	}
	if touched.DeviceName != "Office-MacBook" {
		t.Fatalf("expected updated device name, got %q", touched.DeviceName)
	}
	if !touched.LastSeen.After(prevLastSeen) {
		t.Fatalf("expected LastSeen to be newer after Touch")
	}
}

func TestPeerTrustStoreLoadMissingFile(t *testing.T) {
	store := &peerTrustStore{
		path:  filepath.Join(t.TempDir(), "missing.json"),
		peers: make(map[string]trustedPeerRecord),
	}

	if err := store.load(); err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
}

func TestPeerTrustStoreLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trusted_peers.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0600); err != nil {
		t.Fatalf("failed to write invalid json: %v", err)
	}

	store := &peerTrustStore{path: path, peers: make(map[string]trustedPeerRecord)}
	if err := store.load(); err == nil {
		t.Fatalf("expected load to fail for invalid json")
	}
}

func TestPeerTrustStoreConcurrentSetAndTouch(t *testing.T) {
	store := newTestTrustStore(t)

	const workers = 8
	const updatesPerWorker = 30

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < updatesPerWorker; i++ {
				if err := store.Set("shared-device", "worker", "FP-BASE"); err != nil {
					t.Errorf("worker %d Set failed: %v", worker, err)
					return
				}
				if err := store.Touch("shared-device", "worker-touch"); err != nil {
					t.Errorf("worker %d Touch failed: %v", worker, err)
					return
				}
			}
		}(w)
	}
	wg.Wait()

	rec, ok := store.Get("shared-device")
	if !ok {
		t.Fatalf("expected shared-device to exist")
	}
	if rec.Fingerprint != "FP-BASE" {
		t.Fatalf("unexpected fingerprint after concurrent updates: %q", rec.Fingerprint)
	}
}

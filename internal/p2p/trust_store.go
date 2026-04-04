package p2p

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type trustedPeerRecord struct {
	DeviceID    string    `json:"device_id"`
	DeviceName  string    `json:"device_name"`
	Fingerprint string    `json:"fingerprint"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
}

type peerTrustStore struct {
	mu    sync.RWMutex
	path  string
	peers map[string]trustedPeerRecord
}

func newPeerTrustStore() (*peerTrustStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home: %w", err)
	}

	configDir := filepath.Join(home, ".klip-sync")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	store := &peerTrustStore{
		path:  filepath.Join(configDir, "trusted_peers.json"),
		peers: make(map[string]trustedPeerRecord),
	}

	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *peerTrustStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read trust store: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(data, &s.peers); err != nil {
		return fmt.Errorf("failed to decode trust store: %w", err)
	}

	return nil
}

func (s *peerTrustStore) saveLocked() error {
	data, err := json.MarshalIndent(s.peers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode trust store: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp trust store: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("failed to replace trust store: %w", err)
	}

	return nil
}

func (s *peerTrustStore) Get(deviceID string) (trustedPeerRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.peers[deviceID]
	return rec, ok
}

func (s *peerTrustStore) Set(deviceID, deviceName, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	rec, ok := s.peers[deviceID]
	if !ok {
		rec.FirstSeen = now
	}

	rec.DeviceID = deviceID
	rec.DeviceName = deviceName
	rec.Fingerprint = fingerprint
	rec.LastSeen = now
	s.peers[deviceID] = rec

	return s.saveLocked()
}

func (s *peerTrustStore) Touch(deviceID, deviceName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.peers[deviceID]
	if !ok {
		return nil
	}

	if deviceName != "" {
		rec.DeviceName = deviceName
	}
	rec.LastSeen = time.Now()
	s.peers[deviceID] = rec

	return s.saveLocked()
}

package p2p

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return &Manager{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func buildTarGz(t *testing.T, entries map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range entries {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader failed: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar close failed: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close failed: %v", err)
	}

	return buf.Bytes()
}

func TestExtractTarGzBlocksPathTraversal(t *testing.T) {
	m := newTestManager(t)
	destDir := t.TempDir()

	archive := buildTarGz(t, map[string]string{
		"../escape.txt": "blocked",
	})

	err := m.extractTarGz(bytes.NewReader(archive), destDir)
	if err == nil {
		t.Fatalf("expected path traversal archive to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid tar entry path") {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(destDir, "escape.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("unexpected file created in destination")
	}
}

func TestExtractTarGzExtractsRegularFile(t *testing.T) {
	m := newTestManager(t)
	destDir := t.TempDir()

	archive := buildTarGz(t, map[string]string{
		"folder/note.txt": "hello",
	})

	if err := m.extractTarGz(bytes.NewReader(archive), destDir); err != nil {
		t.Fatalf("extractTarGz failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(destDir, "folder", "note.txt"))
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected file content: got %q", string(data))
	}
}

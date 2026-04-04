package p2p

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestFileHeaderRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := int64(123456789)

	if err := WriteFileHeader(&buf, want); err != nil {
		t.Fatalf("WriteFileHeader failed: %v", err)
	}

	got, err := ReadFileHeader(&buf)
	if err != nil {
		t.Fatalf("ReadFileHeader failed: %v", err)
	}
	if got != want {
		t.Fatalf("size mismatch: got %d, want %d", got, want)
	}
}

func TestReadFileHeaderInvalidMagic(t *testing.T) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, uint32(0xDEADBEEF)); err != nil {
		t.Fatalf("failed to write magic: %v", err)
	}
	if err := binary.Write(&buf, binary.BigEndian, int64(10)); err != nil {
		t.Fatalf("failed to write size: %v", err)
	}

	_, err := ReadFileHeader(&buf)
	if err == nil {
		t.Fatalf("expected invalid magic error")
	}
	if !strings.Contains(err.Error(), "invalid file transfer magic") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadFileHeaderNegativeSize(t *testing.T) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, uint32(FileTransferMagic)); err != nil {
		t.Fatalf("failed to write magic: %v", err)
	}
	if err := binary.Write(&buf, binary.BigEndian, int64(-1)); err != nil {
		t.Fatalf("failed to write size: %v", err)
	}

	_, err := ReadFileHeader(&buf)
	if err == nil {
		t.Fatalf("expected negative size error")
	}
	if !strings.Contains(err.Error(), "invalid file size") {
		t.Fatalf("unexpected error: %v", err)
	}
}

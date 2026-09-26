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

func TestMessageValidate(t *testing.T) {
	tests := []struct {
		name    string
		msg     Message
		wantErr string // empty means the message must be accepted
	}{
		{
			name:    "missing device id",
			msg:     Message{Type: MsgTypeSync, Payload: &Payload{}},
			wantErr: "missing device_id",
		},
		{
			name: "valid hello",
			msg:  Message{Type: MsgTypeHello, DeviceID: "d", Payload: &Payload{DeviceName: "Laptop"}},
		},
		{
			name:    "hello without payload",
			msg:     Message{Type: MsgTypeHello, DeviceID: "d"},
			wantErr: "hello: missing device name",
		},
		{
			name:    "hello without device name",
			msg:     Message{Type: MsgTypeHello, DeviceID: "d", Payload: &Payload{}},
			wantErr: "hello: missing device name",
		},
		{
			name: "valid file offer",
			msg:  Message{Type: MsgTypeFileOffer, DeviceID: "d", Payload: &Payload{FileName: "a.txt", Size: 10}},
		},
		{
			name: "empty file is a valid offer",
			msg:  Message{Type: MsgTypeFileOffer, DeviceID: "d", Payload: &Payload{FileName: "a.txt"}},
		},
		{
			name:    "file offer without payload",
			msg:     Message{Type: MsgTypeFileOffer, DeviceID: "d"},
			wantErr: "file_offer: missing file name",
		},
		{
			name:    "file offer without file name",
			msg:     Message{Type: MsgTypeFileOffer, DeviceID: "d", Payload: &Payload{Size: 10}},
			wantErr: "file_offer: missing file name",
		},
		{
			name:    "file offer with negative size",
			msg:     Message{Type: MsgTypeFileOffer, DeviceID: "d", Payload: &Payload{FileName: "a.txt", Size: -1}},
			wantErr: "file_offer: invalid size",
		},
		{
			name: "valid sync",
			msg:  Message{Type: MsgTypeSync, DeviceID: "d", Payload: &Payload{ClipboardContent: "hi"}},
		},
		{
			name:    "sync without payload",
			msg:     Message{Type: MsgTypeSync, DeviceID: "d"},
			wantErr: "sync: missing payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid message, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("unexpected error: got %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

package p2p

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

func (m *Manager) SendFile(peerID string, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			m.logger.Error("Failed to close file", "error", err)
		}
	}()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	m.mu.RLock()
	peer, ok := m.peers[peerID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("peer %s offline", peerID)
	}

	dataConn, err := m.dialPeer(peer.Address)
	if err != nil {
		return err
	}
	defer func() {
		if err := dataConn.Close(); err != nil {
			m.logger.Debug("Failed to close data connection", "error", err)
		}
	}()

	if err := m.sendFileOffer(dataConn, filepath.Base(filePath), info.Size()); err != nil {
		return err
	}

	if err := m.waitForAccept(dataConn); err != nil {
		return err
	}

	if err := WriteFileHeader(dataConn, info.Size()); err != nil {
		return fmt.Errorf("failed to write file header: %w", err)
	}

	dw := &deadlineWriter{conn: dataConn, timeout: 30 * time.Second}
	pr := &progressReader{
		reader:     file,
		total:      info.Size(),
		fileName:   filepath.Base(filePath),
		progressCh: m.Progress,
	}

	buf := make([]byte, 256*1024)
	written, err := io.CopyBuffer(dw, pr, buf)
	if err != nil {
		return fmt.Errorf("failed to send file data: %w", err)
	}

	pr.SendDone()
	m.logger.Info("File sent", "name", filepath.Base(filePath), "size", written)
	return nil
}

func (m *Manager) handleFileOffer(conn net.Conn, msg *Message) error {
	fileName := msg.Payload.FileName
	fileSize := msg.Payload.Size

	m.logger.Info("File offer received", "file", fileName, "size", fileSize)

	senderName := m.getSenderName(msg.DeviceID)
	accepted, savePath := m.resolveFileAcceptance(senderName, fileName, fileSize)

	if !accepted {
		m.logger.Info("File transfer rejected by user", "file", fileName)
		return nil
	}

	m.logger.Info("File transfer accepted", "file", fileName, "save_path", savePath)

	if err := m.sendFileAccept(conn, ""); err != nil {
		return fmt.Errorf("failed to send acceptance: %w", err)
	}

	return m.receiveFile(conn, fileName, fileSize, savePath)
}

func (m *Manager) receiveFile(conn net.Conn, name string, expectedSize int64, savePath string) error {
	m.logger.Info("Starting file download", "file", name, "size", expectedSize, "path", savePath)

	if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	f, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			m.logger.Error("Failed to close file", "error", err)
		}
	}()

	dr := &deadlineReader{conn: conn, timeout: 30 * time.Second}
	pr := &progressReader{
		reader:     dr,
		total:      expectedSize,
		fileName:   name,
		progressCh: m.Progress,
	}

	size, err := ReadFileHeader(dr)
	if err != nil {
		return fmt.Errorf("failed to read file header: %w", err)
	}
	if size != expectedSize {
		return fmt.Errorf("size mismatch in header: got %d, expected %d", size, expectedSize)
	}

	buf := make([]byte, 256*1024)
	written, err := copyBufferN(f, pr, size, buf)
	if err != nil {
		if closeErr := f.Close(); closeErr != nil {
			m.logger.Debug("Failed to close incomplete file", "error", closeErr)
		}
		if removeErr := os.Remove(savePath); removeErr != nil {
			m.logger.Warn("Failed to remove incomplete file", "path", savePath, "error", removeErr)
		}
		return fmt.Errorf("copy failed: %w (got %d/%d)", err, written, size)
	}

	if written != size {
		return fmt.Errorf("size mismatch: wrote %d, expected %d", written, size)
	}

	pr.SendDone()

	if err := f.Sync(); err != nil {
		m.logger.Warn("Failed to sync file", "error", err)
	}

	m.logger.Info("File saved successfully", "bytes", written, "path", savePath)
	return nil
}

// open a TLS connection to the peer's address for file transfer
// uses timeout of 5 seconds to avoid hanging if peer is unresponsive
func (m *Manager) dialPeer(address string) (*tls.Conn, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, m.clientTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to open data channel: %w", err)
	}
	return conn, nil
}

func (m *Manager) sendFileOffer(conn net.Conn, fileName string, size int64) error {
	msg := &Message{
		Type:     MsgTypeFileOffer,
		DeviceID: m.deviceID,
		Payload: &Payload{
			FileName: fileName,
			Size:     size,
		},
	}
	if err := json.NewEncoder(conn).Encode(msg); err != nil {
		return fmt.Errorf("failed to send file offer: %w", err)
	}
	return nil
}

func (m *Manager) waitForAccept(conn net.Conn) error {
	var resp Message
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	if resp.Type != MsgTypeFileAccept {
		return fmt.Errorf("transfer rejected: %s", resp.Type)
	}
	return nil
}

func (m *Manager) sendFileAccept(conn net.Conn, fileID string) error {
	msg := &Message{
		Type:      MsgTypeFileAccept,
		DeviceID:  m.deviceID,
		Timestamp: time.Now(),
		Payload:   &Payload{ContentHash: fileID},
	}
	return json.NewEncoder(conn).Encode(msg)
}

func (m *Manager) resolveFileAcceptance(senderName, fileName string, fileSize int64) (bool, string) {
	if m.onFileReceive != nil {
		return m.onFileReceive(senderName, fileName, fileSize)
	}

	m.logger.Warn("No file receive callback set - auto-accepting")
	savePath, err := getDownloadPath(fileName)
	if err != nil {
		m.logger.Error("Failed to get download path", "error", err)
		return false, ""
	}
	return true, savePath
}

func getDownloadPath(filename string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	downloadDir := filepath.Join(home, "Downloads")

	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return "", err
	}

	savePath := filepath.Join(downloadDir, filename)
	if _, err := os.Stat(savePath); err == nil {
		ext := filepath.Ext(filename)
		base := filename[:len(filename)-len(ext)]
		savePath = filepath.Join(downloadDir, fmt.Sprintf("%s_%d%s", base, time.Now().Unix(), ext))
	}

	return savePath, nil
}

package p2p

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type transferOffer struct {
	name     string
	size     int64
	isFolder bool
	reader   io.Reader
}

func (m *Manager) SendFile(peerID string, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			m.logger.Warn("Failed to close source file", "error", err)
		}
	}()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	return m.send(peerID, transferOffer{
		name:   filepath.Base(filePath),
		size:   info.Size(),
		reader: file,
	})
}

func (m *Manager) SendFolder(peerID string, folderPath string) error {
	tmpFile, err := os.CreateTemp("", "klip-folder-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp archive: %w", err)
	}
	defer func() {
		if err := tmpFile.Close(); err != nil {
			m.logger.Warn("Failed to close temp archive", "error", err)
		}
		if err := os.Remove(tmpFile.Name()); err != nil {
			m.logger.Warn("Failed to remove temp archive", "error", err)
		}
	}()

	if err := m.createTarGz(tmpFile, folderPath); err != nil {
		return fmt.Errorf("failed to archive folder: %w", err)
	}

	info, err := tmpFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat archive: %w", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek archive: %w", err)
	}

	return m.send(peerID, transferOffer{
		name:     filepath.Base(folderPath),
		size:     info.Size(),
		isFolder: true,
		reader:   tmpFile,
	})
}

func (m *Manager) send(peerID string, offer transferOffer) error {
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

	if err := m.sendOffer(dataConn, offer.name, offer.size, offer.isFolder); err != nil {
		return err
	}

	if err := m.waitForAccept(dataConn); err != nil {
		return err
	}

	if err := WriteFileHeader(dataConn, offer.size); err != nil {
		return fmt.Errorf("failed to write file header: %w", err)
	}

	dw := &deadlineWriter{conn: dataConn, timeout: 30 * time.Second}
	pr := &progressReader{
		reader:     offer.reader,
		total:      offer.size,
		fileName:   offer.name,
		progressCh: m.Progress,
	}

	buf := make([]byte, 256*1024)
	written, err := io.CopyBuffer(dw, pr, buf)
	if err != nil {
		return fmt.Errorf("failed to send data: %w", err)
	}

	pr.SendDone()
	m.logger.Info("Transfer sent", "name", offer.name, "size", written, "is_folder", offer.isFolder)
	return nil
}

// handles an incoming file or folder offer from a peer.
func (m *Manager) handleFileOffer(conn net.Conn, msg *Message) error {
	fileName := msg.Payload.FileName
	fileSize := msg.Payload.Size
	isFolder := msg.Payload.IsFolder

	m.logger.Info("Transfer offer received", "name", fileName, "size", fileSize, "is_folder", isFolder)

	senderName := m.getSenderName(msg.DeviceID)
	accepted, savePath := m.resolveFileAcceptance(senderName, fileName, fileSize)
	if !accepted {
		m.logger.Info("Transfer rejected by user", "name", fileName)
		if err := m.sendFileReject(conn); err != nil {
			return fmt.Errorf("failed to send rejection: %w", err)
		}
		return nil
	}

	if err := m.sendFileAccept(conn, ""); err != nil {
		return fmt.Errorf("failed to send acceptance: %w", err)
	}

	if isFolder {
		return m.receiveFolder(conn, fileName, fileSize, savePath)
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

	var receiveErr error
	defer func() {
		if receiveErr != nil {
			f.Close()
			os.Remove(savePath)
			return
		}
		f.Sync()
		if err := f.Close(); err != nil {
			m.logger.Warn("Failed to close file", "error", err)
		}
	}()

	written, receiveErr := m.receiveData(conn, f, name, expectedSize)
	if receiveErr != nil {
		return receiveErr
	}

	m.logger.Info("File saved", "bytes", written, "path", savePath)
	return nil
}

func (m *Manager) receiveFolder(conn net.Conn, name string, expectedSize int64, savePath string) error {
	m.logger.Info("Starting folder download", "folder", name, "archive_size", expectedSize)

	destDir := filepath.Dir(savePath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "klip-recv-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if err := tmpFile.Close(); err != nil {
			m.logger.Warn("Failed to close temp file", "error", err)
		}
		if err := os.Remove(tmpFile.Name()); err != nil {
			m.logger.Warn("Failed to remove temp file", "error", err)
		}
	}()

	if _, err := m.receiveData(conn, tmpFile, name, expectedSize); err != nil {
		return err
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek temp file: %w", err)
	}

	if err := m.extractTarGz(tmpFile, destDir); err != nil {
		return fmt.Errorf("failed to extract folder: %w", err)
	}

	m.logger.Info("Folder received and extracted", "folder", name, "path", filepath.Join(destDir, name))
	return nil
}

// reads the binary header and streams data from conn into dst.
func (m *Manager) receiveData(conn net.Conn, dst io.Writer, name string, expectedSize int64) (int64, error) {
	dr := &deadlineReader{conn: conn, timeout: 30 * time.Second}
	pr := &progressReader{
		reader:     dr,
		total:      expectedSize,
		fileName:   name,
		progressCh: m.Progress,
	}

	size, err := ReadFileHeader(dr)
	if err != nil {
		return 0, fmt.Errorf("failed to read file header: %w", err)
	}
	if size != expectedSize {
		return 0, fmt.Errorf("size mismatch in header: got %d, expected %d", size, expectedSize)
	}

	buf := make([]byte, 256*1024)
	written, err := copyBufferN(dst, pr, size, buf)
	if err != nil {
		return written, fmt.Errorf("copy failed: %w (got %d/%d)", err, written, size)
	}

	if written != size {
		return written, fmt.Errorf("size mismatch: wrote %d, expected %d", written, size)
	}

	pr.SendDone()
	return written, nil
}

func (m *Manager) dialPeer(address string) (*tls.Conn, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, m.clientTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to open data channel: %w", err)
	}
	return conn, nil
}

func (m *Manager) sendOffer(conn net.Conn, name string, size int64, isFolder bool) error {
	msg := &Message{
		Type:     MsgTypeFileOffer,
		DeviceID: m.deviceID,
		Payload: &Payload{
			FileName: name,
			Size:     size,
			IsFolder: isFolder,
		},
	}
	return json.NewEncoder(conn).Encode(msg)
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

func (m *Manager) sendFileReject(conn net.Conn) error {
	msg := &Message{
		Type:      MsgTypeFileReject,
		DeviceID:  m.deviceID,
		Timestamp: time.Now(),
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

func (m *Manager) createTarGz(w io.Writer, folderPath string) error {
	bw := bufio.NewWriterSize(w, 1024*1024)
	gw, _ := gzip.NewWriterLevel(bw, gzip.NoCompression)
	tw := tar.NewWriter(gw)

	walkErr := filepath.Walk(folderPath, m.tarWalkFunc(tw, folderPath))

	if err := tw.Close(); err != nil && walkErr == nil {
		walkErr = err
	}
	if err := gw.Close(); err != nil && walkErr == nil {
		walkErr = err
	}
	if err := bw.Flush(); err != nil && walkErr == nil {
		walkErr = err
	}

	return walkErr
}

func (m *Manager) tarWalkFunc(tw *tar.Writer, folderPath string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			m.logger.Warn("Skipping inaccessible path", "path", path, "error", err)
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			m.logger.Warn("Skipping file with bad header", "path", path, "error", err)
			return nil
		}

		relPath, err := filepath.Rel(filepath.Dir(folderPath), path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if info.IsDir() {
			header.Name += "/"
			return tw.WriteHeader(header)
		}

		return m.addFileToTar(tw, header, path)
	}
}

// opens the file first, then writes header + data to tar.
// Opening before writing the header avoids corrupt entries (header without data).
func (m *Manager) addFileToTar(tw *tar.Writer, header *tar.Header, path string) error {
	f, err := os.Open(path)
	if err != nil {
		m.logger.Warn("Skipping unreadable file", "path", path, "error", err)
		return nil
	}

	writeErr := tw.WriteHeader(header)
	if writeErr == nil {
		_, writeErr = io.Copy(tw, f)
	}
	if closeErr := f.Close(); closeErr != nil && writeErr == nil {
		writeErr = closeErr
	}
	return writeErr
}

func (m *Manager) extractTarGz(r io.Reader, destDir string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		if err := gr.Close(); err != nil {
			m.logger.Warn("Failed to close gzip reader", "error", err)
		}
	}()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		target := filepath.Join(destDir, header.Name)

		// prevent path traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid tar entry path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := m.extractFile(target, tr, header.Size); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manager) extractFile(target string, r io.Reader, size int64) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	f, err := os.Create(target)
	if err != nil {
		return err
	}

	if _, copyErr := io.Copy(f, io.LimitReader(r, size)); copyErr != nil {
		if closeErr := f.Close(); closeErr != nil {
			m.logger.Warn("Failed to close file after copy error", "file", target, "error", closeErr)
		}
		return copyErr
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close extracted file: %w", err)
	}
	return nil
}

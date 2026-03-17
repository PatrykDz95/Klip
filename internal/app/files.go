package app

import (
	"fmt"
	"klip/internal/p2p"
	"os"
	"path/filepath"

	"github.com/gen2brain/beeep"
	"github.com/sqweek/dialog"
)

func (app *Application) handleIncomingFile(senderName, fileName string, fileSize int64) (bool, string) {
	app.playNotificationSound()

	notificationTitle := "Incoming File Transfer"
	notificationMsg := fmt.Sprintf("%s wants to send you:\n%s (%.2f MB)",
		senderName, fileName, float64(fileSize)/(1024*1024))

	if err := beeep.Notify(notificationTitle, notificationMsg, ""); err != nil {
		app.logger.Warn("Failed to show notification", "error", err)
	}

	response := dialog.Message("%s wants to send you a file:\n\n%s\n\nSize: %.2f MB\n\nDo you want to accept?",
		senderName, fileName, float64(fileSize)/(1024*1024)).
		Title("Incoming File Transfer").
		YesNo()

	if !response {
		app.logger.Info("File transfer rejected by user", "file", fileName)
		app.playRejectionSound()
		return false, ""
	}

	app.logger.Info("File transfer accepted", "file", fileName)

	receivedDir := getReceivedFilesDir()
	if err := os.MkdirAll(receivedDir, 0755); err != nil {
		app.logger.Error("Failed to create directory", "error", err)
		return false, ""
	}

	savePath := filepath.Join(receivedDir, fileName)
	app.logger.Info("File will be saved to", "path", savePath)

	return true, savePath
}

func (app *Application) sendFileToDevice(deviceID string) {
	app.logger.Info("Preparing to send file", "device_id", deviceID)

	peers := app.p2pMgr.GetPeers()
	targetPeer := findPeer(peers, deviceID)
	if targetPeer == nil {
		app.logger.Error("Peer not found", "device_id", deviceID)
		return
	}

	filePaths := app.resolveFilesToSend()
	if len(filePaths) == 0 {
		return
	}

	go app.sendFiles(deviceID, targetPeer.DeviceName, filePaths)
}

func (app *Application) resolveFilesToSend() []string {
	files, err := app.clipboard.GetFiles()
	if err != nil {
		app.logger.Debug("Clipboard files check failed", "error", err)
	}

	if len(files) > 0 {
		return files
	}

	filePath, err := dialog.File().Title("Select file to send").Load()
	if err != nil {
		app.logger.Debug("File picker cancelled")
		return nil
	}

	return []string{filePath}
}

func (app *Application) sendFiles(deviceID, deviceName string, filePaths []string) {
	for _, f := range filePaths {
		app.logger.Info("Sending file", "file", filepath.Base(f), "to", deviceName)
		if err := app.p2pMgr.SendFile(deviceID, f); err != nil {
			app.logger.Error("Failed to send file", "file", f, "error", err)
			continue
		}
		app.updateStatus("File sent: " + filepath.Base(f))
	}
	app.hideStatusAfter(10)
}

func findPeer(peers []p2p.PeerInfo, deviceID string) *p2p.PeerInfo {
	for _, peer := range peers {
		if peer.DeviceID == deviceID {
			return &peer
		}
	}
	return nil
}

package app

import (
	"fmt"
	"klip/internal/file_picker"
	"klip/internal/p2p"
	"os"
	"path/filepath"

	"github.com/gen2brain/beeep"
	"github.com/sqweek/dialog"
)

func (app *Application) handleIncomingFile(senderName, fileName string, fileSize int64) (bool, string) {
	if app.isDeviceLimitBlocked() {
		app.logger.Warn("Rejected incoming file: free device limit reached", "file", fileName)
		return false, ""
	}

	app.playNotificationSound()

	notificationTitle := "Incoming Transfer"
	notificationMsg := fmt.Sprintf("%s wants to send you:\n%s (%.2f MB)",
		senderName, fileName, float64(fileSize)/(1024*1024))

	if err := beeep.Notify(notificationTitle, notificationMsg, ""); err != nil {
		app.logger.Warn("Failed to show notification", "error", err)
	}

	response := dialog.Message("%s wants to send you:\n\n%s\n\nSize: %.2f MB\n\nDo you want to accept?",
		senderName, fileName, float64(fileSize)/(1024*1024)).
		Title("Incoming Transfer").
		YesNo()

	if !response {
		app.logger.Info("Transfer rejected by user", "name", fileName)
		app.playRejectionSound()
		return false, ""
	}

	app.logger.Info("Transfer accepted", "name", fileName)

	receivedDir := getReceivedFilesDir()
	if err := os.MkdirAll(receivedDir, 0755); err != nil {
		app.logger.Error("Failed to create directory", "error", err)
		return false, ""
	}

	savePath := filepath.Join(receivedDir, fileName)
	app.logger.Info("Will be saved to", "path", savePath)

	return true, savePath
}

func (app *Application) sendFileToDevice(deviceID string) {
	if app.isDeviceLimitBlocked() {
		dialog.Message("Free device limit reached. Upgrade to Klip Pro to send files.").Title("Klip").Error()
		return
	}

	app.logger.Info("Preparing to send", "device_id", deviceID)

	peers := app.p2pMgr.GetPeers()
	targetPeer := findPeer(peers, deviceID)
	if targetPeer == nil {
		app.logger.Error("Peer not found", "device_id", deviceID)
		return
	}

	result := file_picker.PickFileOrFolder("Select file or folder to send")
	if result == nil {
		app.logger.Debug("Picker cancelled")
		return
	}

	if result.IsDir {
		go app.sendFolder(deviceID, targetPeer.DeviceName, result.Path)
	} else {
		go app.sendFiles(deviceID, targetPeer.DeviceName, []string{result.Path})
	}
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

func (app *Application) sendFolder(deviceID, deviceName, folderPath string) {
	folderName := filepath.Base(folderPath)
	app.logger.Info("Sending folder", "folder", folderName, "to", deviceName)

	app.updateStatus("Sending folder: " + folderName + "...")

	if err := app.p2pMgr.SendFolder(deviceID, folderPath); err != nil {
		app.logger.Error("Failed to send folder", "folder", folderName, "error", err)
		app.updateStatus("Failed to send: " + folderName)
		dialog.Message("Failed to send folder %s:\n%s", folderName, err.Error()).Title("Klip").Error()
		app.hideStatusAfter(3)
		return
	}

	app.updateStatus("Folder sent: " + folderName)
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

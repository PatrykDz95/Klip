package app

import (
	"context"
	"fmt"
	"klip/internal/clipboard"
	"klip/internal/license"
	"klip/internal/p2p"
	"klip/internal/security"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/getlantern/systray"
)

type Application struct {
	deviceID  string
	logger    *slog.Logger
	p2pMgr    *p2p.Manager
	discovery *p2p.Discovery
	clipboard clipboard.Clipboard
	ctx       context.Context
	cancel    context.CancelFunc

	ui       *UI
	iconData []byte

	clipboardClearCancel context.CancelFunc
	clipboardClearMu     sync.Mutex

	paused   bool
	pausedMu sync.RWMutex

	deviceLimitBlocked bool
	deviceLimitMu      sync.RWMutex

	license       *license.StoredLicense
	licenseMu     sync.RWMutex
	licenseClient *license.Client
}

func NewApplication(iconData []byte) *Application {
	return &Application{
		iconData: iconData,
		ui: &UI{
			peers: make(map[string]peerEntry),
		},
	}
}

func (app *Application) OnReady() {
	systray.SetIcon(app.iconData)
	systray.SetTitle("Klip")
	systray.SetTooltip("Klip - Starting...")

	app.buildMenu()
	go app.startBackend()
}

func (app *Application) OnExit() {
	if app.cancel != nil {
		app.cancel()
	}
	if app.logger != nil {
		app.logger.Info("Klip application stopped")
	}
}

func (app *Application) startBackend() {
	cfg := parseFlags()

	if err := app.initLogger(cfg.Verbose); err != nil {
		return
	}

	if cfg.DeviceName == "" {
		cfg.DeviceName = getDefaultDeviceName()
	}

	deviceID := getOrCreateDeviceID()

	app.logger.Info("Starting Klip",
		"device", cfg.DeviceName,
		"device_id", deviceID,
		"os", runtime.GOOS,
	)
	app.deviceID = deviceID
	app.initLicense()
	app.updateLicenseMenu()

	if err := app.initializeComponents(cfg, deviceID); err != nil {
		app.logger.Error("Failed to initialize", "error", err)
		app.updateStatus("Error: " + err.Error())
		return
	}

	if err := app.startServices(cfg, deviceID); err != nil {
		app.logger.Error("Failed to start services", "error", err)
		return
	}

	systray.SetTooltip("Klip - Ready")
	app.updatePeerMenu()

	app.logger.Info("Klip started successfully")
}

func (app *Application) initLogger(verbose bool) error {
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}

	logFile := getLogPath()
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		file = os.Stderr
	}

	app.logger = slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: logLevel,
	}))

	return nil
}

func (app *Application) initializeComponents(cfg *Config, deviceID string) error {
	app.updateStatus("Checking dependencies")
	if !checkDependencies(app.logger) {
		return fmt.Errorf("missing dependencies")
	}

	app.updateStatus("Generating certificates")
	certDir := getCertDir()
	cert, err := security.GenerateSelfSignedCert(certDir, deviceID)
	if err != nil {
		return fmt.Errorf("certificate generation failed in %s: %w", certDir, err)
	}

	// Ensure received files directory exists
	err = os.MkdirAll(getReceivedFilesDir(), 0755)
	if err != nil {
		app.logger.Error("Failed to create received files directory", "error", err)
	}

	app.updateStatus("Initializing clipboard")
	cb, err := clipboard.New(app.logger)
	if err != nil {
		return fmt.Errorf("clipboard initialization failed: %w", err)
	}
	app.clipboard = cb

	app.updateStatus("Starting network")
	app.p2pMgr = p2p.NewManager(deviceID, cfg.DeviceName, cfg.Port, cert, app.logger, newP2PEventHandler(app))
	app.hideStatusAfter(5)

	return nil
}

func (app *Application) startServices(cfg *Config, deviceID string) error {
	if err := app.p2pMgr.Listen(); err != nil {
		return fmt.Errorf("listener start failed: %w", err)
	}

	app.ctx, app.cancel = context.WithCancel(context.Background())
	app.discovery = p2p.NewDiscovery(
		deviceID,
		cfg.DeviceName,
		cfg.Port,
		app.logger,
	)

	go func() {
		if err := app.discovery.Advertise(app.ctx); err != nil {
			app.logger.Error("Advertisement error", "error", err)
		}
	}()

	go app.discovery.Discover(app.ctx)

	go func() {
		for {
			select {
			case <-app.ctx.Done():
				return
			case peer := <-app.discovery.Peers:
				app.handlePeerDiscovered(peer.DeviceID, peer.Address)
			}
		}
	}()

	if cfg.PeerAddr != "" {
		go app.connectToManualPeer(cfg.PeerAddr)
	}

	go func() {
		err := app.startClipboardMonitoring()
		if err != nil {
			app.logger.Error("Clipboard monitoring error", "error", err)
		}
	}()

	go func() {
		for p := range app.p2pMgr.Progress {
			if p.Done {
				app.updateStatus(fmt.Sprintf("%s: 100%%", p.FileName))
				app.hideStatusAfter(5)
			} else {
				pct := float64(p.Transferred) / float64(p.Total) * 100
				app.updateStatus(fmt.Sprintf("%s: %.0f%%", p.FileName, pct))
			}
		}
	}()

	// TODO: look into if there is a better way
	// Periodically refresh peer menu
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-app.ctx.Done():
				return
			case <-ticker.C:
				app.updatePeerMenu()
			}
		}
	}()

	return nil
}

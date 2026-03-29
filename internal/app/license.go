package app

import (
	"klip/internal/license"

	"github.com/sqweek/dialog"
)

const maxFreeDevices = 1

func (app *Application) initLicense() {
	app.licenseClient = license.NewClient()

	stored, err := license.Load()
	if err != nil {
		app.logger.Warn("Failed to load license", "error", err)
		return
	}

	if stored == nil {
		app.logger.Info("Running in Free mode (2 devices)")
		return
	}

	resp, err := app.licenseClient.Validate(stored.LicenseKey, stored.InstanceID)
	if err != nil {
		app.logger.Warn("License validation failed", "error", err)
		return
	}

	if resp.Valid && resp.LicenseKey.Status == "active" {
		app.licenseMu.Lock()
		app.license = stored
		app.licenseMu.Unlock()
		app.logger.Info("Pro license active")
	} else {
		app.logger.Warn("License expired or invalid", "status", resp.LicenseKey.Status)
		if err := license.Remove(); err != nil {
			app.logger.Error("Failed to remove invalid license", "error", err)
		}
	}
}

func (app *Application) activateLicense() {
	key := promptLicenseKey()
	if key == "" {
		return
	}

	app.logger.Info("License activation requested")

	resp, err := app.licenseClient.Activate(key, app.deviceID)
	if err != nil {
		app.logger.Error("Activation failed", "error", err)
		dialog.Message("Activation failed: %s", err).Title("Klip").Error()
		return
	}

	if resp.Instance == nil {
		dialog.Message("Activation failed").Title("Klip").Error()
		return
	}

	stored := license.StoredLicense{
		LicenseKey: key,
		InstanceID: resp.Instance.ID,
	}

	if err := license.Save(stored); err != nil {
		app.logger.Error("Failed to save license", "error", err)
		return
	}

	app.licenseMu.Lock()
	app.license = &stored
	app.licenseMu.Unlock()

	app.setDeviceLimitBlocked(false)
	app.logger.Info("Pro license activated")
	dialog.Message("Pro license activated! Unlimited devices enabled.").Title("Klip").Info()
}

package main

import (
	_ "embed"
	"io"
	"klip/internal/app"
	"log"
	"runtime"

	"github.com/getlantern/systray"
)

//go:embed assets/klip-512.ico
var iconDataIco []byte

//go:embed assets/klip-512.png
var iconDataPng []byte

func main() {
	// Suppress mdns warnings
	log.SetOutput(io.Discard)

	iconData := iconDataIco
	if runtime.GOOS == "linux" {
		iconData = iconDataPng
	}

	application := app.NewApplication(iconData)
	systray.Run(application.OnReady, application.OnExit)
}

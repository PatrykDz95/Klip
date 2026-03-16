package main

import (
	_ "embed"
	"io"
	"klip/internal/app"
	"log"

	"github.com/getlantern/systray"
)

//go:embed assets/klip-fixed-256.png
var iconDataPng []byte

func main() {
	// Suppress mdns warnings
	log.SetOutput(io.Discard)

	application := app.NewApplication(iconDataPng)
	systray.Run(application.OnReady, application.OnExit)
}

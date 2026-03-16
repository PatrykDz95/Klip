package main

import (
	_ "embed"
	"io"
	"klip/internal/app"
	"log"
	"runtime"

	"github.com/getlantern/systray"
)

//go:embed assets/klip-fixed-256.png
var iconDataPng []byte

//go:embed assets/klip-512.ico
var iconIco []byte

func main() {
	// Suppress mdns warnings
	log.SetOutput(io.Discard)

	var icon []byte
	if runtime.GOOS == "windows" {
		icon = iconIco
	} else {
		icon = iconDataPng
	}

	application := app.NewApplication(icon)
	systray.Run(application.OnReady, application.OnExit)
}

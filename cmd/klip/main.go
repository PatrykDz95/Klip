package main

import (
	_ "embed"
	"io"
	"klip/internal/app"
	"log"

	"github.com/getlantern/systray"
)

//go:embed assets/klip-512.ico
var iconData []byte

func main() {
	// Suppress mdns warnings
	log.SetOutput(io.Discard)

	application := app.NewApplication(iconData)
	systray.Run(application.OnReady, application.OnExit)
}

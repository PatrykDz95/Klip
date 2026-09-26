package app

import (
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
)

func newTestApp() *Application {
	app := NewApplication(nil)
	app.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return app
}

func TestBeginDialAllowsOnlyOneConcurrentDialPerPeer(t *testing.T) {
	app := newTestApp()

	const goroutines = 50
	var granted atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})

	for range goroutines {
		wg.Go(func() {
			<-start // release all goroutines at once to maximise contention
			if app.beginDial("peer-1") {
				granted.Add(1)
			}
		})
	}
	close(start)
	wg.Wait()

	if got := granted.Load(); got != 1 {
		t.Fatalf("expected exactly 1 dial to be granted, got %d", got)
	}
}

func TestBeginDialIsPerPeer(t *testing.T) {
	app := newTestApp()

	if !app.beginDial("peer-1") {
		t.Fatalf("expected first dial to peer-1 to be allowed")
	}
	if !app.beginDial("peer-2") {
		t.Fatalf("a dial in progress to peer-1 must not block peer-2")
	}
}

func TestEndDialAllowsNextAttempt(t *testing.T) {
	app := newTestApp()

	if !app.beginDial("peer-1") {
		t.Fatalf("expected first dial to be allowed")
	}
	if app.beginDial("peer-1") {
		t.Fatalf("expected second dial to be refused while the first is in progress")
	}

	app.endDial("peer-1")

	if !app.beginDial("peer-1") {
		t.Fatalf("expected dial to be allowed again after endDial")
	}
}

func TestBeginDialRespectsBackoff(t *testing.T) {
	app := newTestApp()

	app.recordDialFailure("peer-1", errors.New("connection refused"))
	if app.beginDial("peer-1") {
		t.Fatalf("expected dial to be refused while backoff is active")
	}

	app.recordDialSuccess("peer-1")
	if !app.beginDial("peer-1") {
		t.Fatalf("expected dial to be allowed after backoff is cleared")
	}
}

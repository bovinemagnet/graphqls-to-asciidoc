// Package daemon watches GraphQL schema files, republishes the AsciiDoc
// document when they settle, and serves a dashboard reporting what it did.
package daemon

import (
	"context"
	"log"
	"sync"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

// Run starts the watcher, the build coordinator and the dashboard, and blocks
// until the context is cancelled.
func Run(ctx context.Context, cfg *config.Config) error {
	// The background goroutines run under a context of the daemon's own, so a
	// server that never starts — a port already in use, say — still releases
	// them. Waiting on the caller's context alone would hang for ever.
	runCtx, stop := context.WithCancel(ctx)
	defer stop()

	watcher, backend, err := NewWatcher(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = watcher.Close() }()

	clock := SystemClock()
	coord := NewCoordinator(cfg, clock, backend)

	server, err := NewServer(cfg, coord)
	if err != nil {
		return err
	}

	// Build once at startup so the dashboard has something to show and the
	// document exists before anyone opens it.
	coord.Rebuild()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		coord.Run(runCtx, watcher)
	}()

	if cfg.KrokiURL != "" {
		prober, proberErr := NewKrokiProber(cfg.KrokiURL)
		if proberErr != nil {
			return proberErr
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			prober.Run(runCtx, clock, coord)
		}()
	}

	log.Printf("watching with the %s backend, dashboard on http://%s/, document on http://%s%s",
		backend, cfg.DaemonAddr, cfg.DaemonAddr, server.DocumentPath())

	serveErr := server.ListenAndServe(runCtx)

	// Closing the watcher ends the coordinator loop even if the server stopped
	// for its own reasons rather than a cancelled context. Cancelling releases
	// the Kroki prober, whose only exit is a finished context. A build already
	// under way runs to completion: the coordinator only checks the context
	// between builds, so wg.Wait still waits for it.
	_ = watcher.Close()
	stop()
	wg.Wait()

	return serveErr
}

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
		coord.Run(ctx, watcher)
	}()

	if cfg.KrokiURL != "" {
		prober, proberErr := NewKrokiProber(cfg.KrokiURL)
		if proberErr != nil {
			return proberErr
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			prober.Run(ctx, clock, coord)
		}()
	}

	log.Printf("watching with the %s backend, dashboard on http://%s%s",
		backend, cfg.DaemonAddr, server.DocumentPath())

	serveErr := server.ListenAndServe(ctx)

	// Closing the watcher ends the coordinator loop even if the server stopped
	// for its own reasons rather than a cancelled context.
	_ = watcher.Close()
	wg.Wait()

	return serveErr
}

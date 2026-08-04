package daemon

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/build"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

// tickDivisor sets the watch loop's poll frequency relative to the debounce
// period, so a settled burst is noticed promptly without busy-waiting.
const tickDivisor = 10

// minTick is the shortest interval the watch loop ticks at, regardless of how
// small the configured debounce period is.
const minTick = 50 * time.Millisecond

// Coordinator owns the daemon's state and is the only thing that runs builds.
// Everything it exposes is safe for concurrent use, so HTTP handlers can read
// the state and request rebuilds while the watch loop is running.
type Coordinator struct {
	cfg   *config.Config
	clock Clock

	mu    sync.Mutex
	state State

	// buildMu serialises builds so a manual rebuild and a watched change cannot
	// write the output file at the same time.
	buildMu sync.Mutex
}

// NewCoordinator returns a coordinator for the given configuration. backend is
// the watch backend name reported on the dashboard.
func NewCoordinator(cfg *config.Config, clock Clock, backend string) *Coordinator {
	return &Coordinator{
		cfg:   cfg,
		clock: clock,
		state: State{
			Mode:    ModeWatching,
			Backend: backend,
			Kroki:   KrokiStatus{Enabled: cfg.KrokiURL != "", URL: cfg.KrokiURL},
		},
	}
}

// Snapshot returns a copy of the current state.
func (c *Coordinator) Snapshot() State {
	c.mu.Lock()
	defer c.mu.Unlock()

	snapshot := c.state
	snapshot.History = append([]BuildRecord(nil), c.state.History...)
	snapshot.Watched = append([]string(nil), c.state.Watched...)
	return snapshot
}

// SetKroki records the result of a reachability probe.
func (c *Coordinator) SetKroki(status KrokiStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.Kroki = status
}

// Rebuild renders the document and replaces the output file. A failure is
// recorded and reported; the previous good output is left in place.
func (c *Coordinator) Rebuild() {
	c.buildMu.Lock()
	defer c.buildMu.Unlock()

	c.setMode(ModeBuilding)
	defer c.setMode(ModeWatching)

	started := c.clock.Now()
	record := BuildRecord{Started: started}

	result, err := build.Run(c.cfg)
	switch {
	case err != nil:
		record.Err = err.Error()
	default:
		record.Bytes = len(result.Content)
		if writeErr := writeAtomic(c.cfg.OutputFile, result.Content); writeErr != nil {
			record.Err = writeErr.Error()
		}
	}
	record.Duration = c.clock.Now().Sub(started)

	c.record(record, result)
}

// record folds a finished build into the state.
func (c *Coordinator) record(r BuildRecord, result *build.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state.LastBuild = r.Started
	c.state.LastError = r.Err
	if result != nil {
		c.state.Watched = append([]string(nil), result.Files...)
	}

	c.state.History = append([]BuildRecord{r}, c.state.History...)
	if len(c.state.History) > historyLimit {
		c.state.History = c.state.History[:historyLimit]
	}

	if r.Err != "" {
		log.Printf("build failed: %s", r.Err)
	} else if c.cfg.Verbose {
		log.Printf("built %s (%d bytes) in %s", c.cfg.OutputFile, r.Bytes, r.Duration)
	}
}

func (c *Coordinator) setMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.Mode = mode
}

// Run drives the watch loop until the context is cancelled. Changes are fed to
// the debouncer, and a ticker gives it the chance to fire once a burst settles.
func (c *Coordinator) Run(ctx context.Context, w Watcher) {
	debouncer := NewDebouncer(c.cfg.Debounce, c.clock)

	tick := c.cfg.Debounce / tickDivisor
	if tick < minTick {
		tick = minTick
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case _, ok := <-w.Events():
			if !ok {
				return
			}
			debouncer.Changed()

		case <-ticker.C:
			if !debouncer.Ready() {
				continue
			}
			debouncer.BuildStarted()
			c.Rebuild()
			debouncer.BuildCompleted()
		}
	}
}

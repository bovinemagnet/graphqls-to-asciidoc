package daemon

import (
	"fmt"
	"log"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

// Event reports that a watched schema file changed. The backend that produced
// it is deliberately invisible to the rebuild loop.
type Event struct {
	Path string
}

// eventBufferSize is the capacity of a backend's events channel. It absorbs a
// burst of changes (e.g. an editor's save-then-rename sequence) so a slow
// consumer doesn't block the watcher's internal loop.
const eventBufferSize = 16

// Watcher reports changes to the schema files named by the configuration.
type Watcher interface {
	// Events delivers one event per detected change.
	Events() <-chan Event
	// Close stops the watcher and releases its resources.
	Close() error
}

// NewWatcher builds the watcher named by --watch-mode, returning the backend
// actually in use. The backend name is one of config.WatchModeFSNotify or
// config.WatchModePoll — the same vocabulary as --watch-mode, since the
// backend in use is the watch mode that was actually resolved. Under "auto"
// an fsnotify failure falls back to polling; a forced "fsnotify" reports the
// error instead, because forcing a backend should mean it.
func NewWatcher(cfg *config.Config) (Watcher, string, error) {
	switch cfg.WatchMode {
	case config.WatchModePoll:
		w, err := NewPollWatcher(cfg)
		return w, config.WatchModePoll, err

	case config.WatchModeFSNotify:
		w, err := NewFSNotifyWatcher(cfg)
		if err != nil {
			return nil, "", fmt.Errorf("--watch-mode=fsnotify was requested but the watcher could not start: %w", err)
		}
		return w, config.WatchModeFSNotify, nil

	default:
		w, err := NewFSNotifyWatcher(cfg)
		if err == nil {
			return w, config.WatchModeFSNotify, nil
		}
		log.Printf("fsnotify unavailable (%v); falling back to polling every %s", err, cfg.PollInterval)

		w, err = NewPollWatcher(cfg)
		return w, config.WatchModePoll, err
	}
}

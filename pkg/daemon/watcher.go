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

// Watcher reports changes to the schema files named by the configuration.
type Watcher interface {
	// Events delivers one event per detected change.
	Events() <-chan Event
	// Close stops the watcher and releases its resources.
	Close() error
}

// NewWatcher builds the watcher named by --watch-mode, returning the backend
// actually in use. Under "auto" an fsnotify failure falls back to polling; a
// forced "fsnotify" reports the error instead, because forcing a backend should
// mean it.
func NewWatcher(cfg *config.Config) (Watcher, string, error) {
	switch cfg.WatchMode {
	case config.WatchModePoll:
		w, err := NewPollWatcher(cfg)
		return w, "poll", err

	case config.WatchModeFSNotify:
		w, err := NewFSNotifyWatcher(cfg)
		if err != nil {
			return nil, "", fmt.Errorf("--watch-mode=fsnotify was requested but the watcher could not start: %w", err)
		}
		return w, "fsnotify", nil

	default:
		w, err := NewFSNotifyWatcher(cfg)
		if err == nil {
			return w, "fsnotify", nil
		}
		log.Printf("fsnotify unavailable (%v); falling back to polling every %s", err, cfg.PollInterval)

		w, err = NewPollWatcher(cfg)
		return w, "poll", err
	}
}

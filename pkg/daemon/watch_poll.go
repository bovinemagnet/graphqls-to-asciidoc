package daemon

import (
	"os"
	"sort"
	"sync"
	"time"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/build"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

// fileState is the part of a file's metadata that a scan compares.
type fileState struct {
	ModTime time.Time
	Size    int64
}

// snapshotFiles records the state of each readable path. Unreadable paths are
// omitted, which makes them look like a removal on the next diff.
func snapshotFiles(paths []string) map[string]fileState {
	states := make(map[string]fileState, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		states[p] = fileState{ModTime: info.ModTime(), Size: info.Size()}
	}
	return states
}

// diffSnapshots returns the paths that were added, removed or modified between
// two scans, sorted so the result is deterministic.
func diffSnapshots(before, after map[string]fileState) []string {
	changed := make(map[string]struct{})

	for path, now := range after {
		was, existed := before[path]
		if !existed || was != now {
			changed[path] = struct{}{}
		}
	}
	for path := range before {
		if _, stillThere := after[path]; !stillThere {
			changed[path] = struct{}{}
		}
	}

	paths := make([]string, 0, len(changed))
	for path := range changed {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// pollWatcher re-globs the configured pattern on a fixed interval and compares
// file metadata. It works identically everywhere, including on network and
// container filesystems where change notifications are unreliable.
type pollWatcher struct {
	cfg       *config.Config
	events    chan Event
	done      chan struct{}
	closeOnce sync.Once
}

// NewPollWatcher starts a polling watcher for the configured schema files.
func NewPollWatcher(cfg *config.Config) (Watcher, error) {
	files, err := build.ResolveFiles(cfg)
	if err != nil {
		return nil, err
	}

	w := &pollWatcher{
		cfg:    cfg,
		events: make(chan Event, eventBufferSize),
		done:   make(chan struct{}),
	}
	go w.loop(snapshotFiles(files))
	return w, nil
}

func (w *pollWatcher) loop(previous map[string]fileState) {
	defer close(w.events)

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			files, err := build.ResolveFiles(w.cfg)
			if err != nil {
				// A pattern that cannot be expanded right now is reported on the
				// next successful scan; there is nothing useful to emit here.
				continue
			}
			current := snapshotFiles(files)
			for _, path := range diffSnapshots(previous, current) {
				select {
				case w.events <- Event{Path: path}:
				case <-w.done:
					return
				}
			}
			previous = current
		}
	}
}

func (w *pollWatcher) Events() <-chan Event { return w.events }

func (w *pollWatcher) Close() error {
	w.closeOnce.Do(func() { close(w.done) })
	return nil
}

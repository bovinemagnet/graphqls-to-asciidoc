package daemon

import (
	"path/filepath"
	"sort"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/build"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

// fsWatcher reports changes through the operating system's notification
// interface. It watches the directories holding the schema files rather than
// the files themselves, so that newly created files are noticed and editors
// that save by writing a temporary file and renaming it are handled.
type fsWatcher struct {
	cfg       *config.Config
	inner     *fsnotify.Watcher
	events    chan Event
	done      chan struct{}
	closeOnce sync.Once
}

// NewFSNotifyWatcher starts an fsnotify-backed watcher. An error here is the
// caller's cue to fall back to polling.
func NewFSNotifyWatcher(cfg *config.Config) (Watcher, error) {
	inner, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &fsWatcher{
		cfg:    cfg,
		inner:  inner,
		events: make(chan Event, 16),
		done:   make(chan struct{}),
	}

	if err := w.watchDirs(); err != nil {
		_ = inner.Close()
		return nil, err
	}

	go w.loop()
	return w, nil
}

// watchDirs adds the parent directory of every matched schema file.
func (w *fsWatcher) watchDirs() error {
	files, err := build.ResolveFiles(w.cfg)
	if err != nil {
		return err
	}

	dirs := make(map[string]struct{}, len(files))
	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			abs = f
		}
		dirs[filepath.Dir(abs)] = struct{}{}
	}

	names := make([]string, 0, len(dirs))
	for dir := range dirs {
		names = append(names, dir)
	}
	sort.Strings(names)

	for _, dir := range names {
		if err := w.inner.Add(dir); err != nil {
			return err
		}
	}
	return nil
}

// matches reports whether a notified path is one of the schema files we care
// about. Comparison is on the absolute path so that relative configuration and
// absolute notifications agree.
func (w *fsWatcher) matches(path string) (string, bool) {
	files, err := build.ResolveFiles(w.cfg)
	if err != nil {
		return "", false
	}

	notified, err := filepath.Abs(path)
	if err != nil {
		notified = path
	}

	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			abs = f
		}
		if abs == notified {
			return f, true
		}
	}
	return "", false
}

func (w *fsWatcher) loop() {
	defer close(w.events)

	for {
		select {
		case <-w.done:
			return
		case err, ok := <-w.inner.Errors:
			if !ok {
				return
			}
			// A notification error is not fatal; the next event still arrives.
			_ = err
		case ev, ok := <-w.inner.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			// A new directory may now hold matching files. Add is idempotent,
			// so re-running the scan simply keeps the watch set current.
			if ev.Op&fsnotify.Create != 0 {
				_ = w.watchDirs()
			}
			path, interesting := w.matches(ev.Name)
			if !interesting {
				continue
			}
			select {
			case w.events <- Event{Path: path}:
			case <-w.done:
				return
			}
		}
	}
}

func (w *fsWatcher) Events() <-chan Event { return w.events }

// Close stops the watcher and releases the underlying fsnotify resources.
// closeOnce guards against a double close of the done channel when Close is
// called from several goroutines at once.
func (w *fsWatcher) Close() error {
	w.closeOnce.Do(func() { close(w.done) })
	return w.inner.Close()
}

package daemon

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
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

	// known maps the absolute path of every schema file seen by the most recent
	// expansion to the path as the configuration named it. A deleted file has
	// already dropped out of the expansion by the time its event arrives, so the
	// previous expansion is what identifies it. Only the constructor and the
	// loop goroutine touch it, and the constructor finishes first.
	known map[string]string
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
		events: make(chan Event, eventBufferSize),
		done:   make(chan struct{}),
		known:  map[string]string{},
	}

	// A pattern that matches nothing at start-up is an error here, the same as
	// it is for the polling backend; the caller reports it rather than watching
	// an empty set for ever.
	files, err := build.ResolveFiles(cfg)
	if err != nil {
		_ = inner.Close()
		return nil, err
	}
	w.known = knownFiles(files)

	if err := w.refreshWatches(files); err != nil {
		_ = inner.Close()
		return nil, err
	}

	go w.loop()
	return w, nil
}

// absolutePath resolves a path for comparison, falling back to the original
// when the working directory cannot be determined.
func absolutePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// knownFiles indexes an expansion by absolute path, keeping the configured
// spelling as the value so emitted events read the way the user wrote them.
func knownFiles(files []string) map[string]string {
	known := make(map[string]string, len(files))
	for _, f := range files {
		known[absolutePath(f)] = f
	}
	return known
}

// patternRootDir returns the deepest directory of a pattern that holds no
// wildcard, e.g. "schemas" for "schemas/**/*.graphqls". Watching it means a
// file created in a subdirectory that did not exist at start-up still produces
// an event, because its parent is already being watched.
func patternRootDir(pattern string) string {
	normalised := filepath.ToSlash(pattern)

	wildcard := strings.IndexAny(normalised, "*?[{")
	if wildcard < 0 {
		return filepath.Dir(pattern)
	}

	slash := strings.LastIndex(normalised[:wildcard], "/")
	switch {
	case slash < 0:
		return "."
	case slash == 0:
		return string(filepath.Separator)
	default:
		return filepath.FromSlash(normalised[:slash])
	}
}

// watchTargets lists the directories to watch: the parent of every matched
// file, plus the pattern's static root so that an empty or brand new
// subdirectory is covered too. A '**' pattern matches at any depth, so the
// whole tree below the root is included for those.
func (w *fsWatcher) watchTargets(files []string) []string {
	dirs := make(map[string]struct{}, len(files)+1)
	for _, f := range files {
		dirs[filepath.Dir(absolutePath(f))] = struct{}{}
	}

	if w.cfg.SchemaPattern != "" {
		root := absolutePath(patternRootDir(w.cfg.SchemaPattern))
		dirs[root] = struct{}{}
		if strings.Contains(w.cfg.SchemaPattern, "**") {
			for _, sub := range subdirectories(root) {
				dirs[sub] = struct{}{}
			}
		}
	}

	names := make([]string, 0, len(dirs))
	for dir := range dirs {
		names = append(names, dir)
	}
	sort.Strings(names)
	return names
}

// subdirectories lists every directory below root. Unreadable branches are
// skipped rather than failing the scan; they simply go unwatched.
func subdirectories(root string) []string {
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// An unreadable branch is skipped rather than aborting the walk.
			return nil
		}
		if d.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})
	return dirs
}

// refreshWatches brings the watched directory set up to date. Add is
// idempotent, so it can be re-run whenever the tree may have grown.
func (w *fsWatcher) refreshWatches(files []string) error {
	for _, dir := range w.watchTargets(files) {
		if err := w.inner.Add(dir); err != nil {
			return err
		}
	}
	return nil
}

// rescan re-expands the pattern, updates the watched directories and returns
// the files that have appeared since the last expansion. Those appearances
// matter because a file can be created inside a directory before the watch on
// that directory exists, in which case no event for the file itself arrives.
func (w *fsWatcher) rescan() []string {
	files, err := build.ResolveFiles(w.cfg)
	if err != nil {
		// The pattern may match nothing for the moment; the next event tries
		// again, and the known set still identifies what was there before.
		return nil
	}

	// A failure to watch a directory is not fatal: the directory may have gone
	// again already, and the next event retries.
	_ = w.refreshWatches(files)

	current := knownFiles(files)
	var appeared []string
	for abs, configured := range current {
		if _, seen := w.known[abs]; !seen {
			appeared = append(appeared, configured)
		}
	}
	sort.Strings(appeared)

	w.known = current
	return appeared
}

// matches reports whether a notified path is one of the schema files we care
// about. Comparison is on the absolute path so that relative configuration and
// absolute notifications agree. A path that has left the expansion but was in
// the previous one is a removal, and counts.
func (w *fsWatcher) matches(path string) (string, bool) {
	notified := absolutePath(path)

	previous := w.known
	if files, err := build.ResolveFiles(w.cfg); err == nil {
		w.known = knownFiles(files)
	}

	if configured, ok := w.known[notified]; ok {
		return configured, true
	}

	// Absent now but present a moment ago: the file has been removed or renamed
	// away, which changes the document and must be published.
	configured, ok := previous[notified]
	return configured, ok
}

// emit delivers an event unless the watcher is closing.
func (w *fsWatcher) emit(path string) bool {
	select {
	case w.events <- Event{Path: path}:
		return true
	case <-w.done:
		return false
	}
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
			if !w.handle(ev) {
				return
			}
		}
	}
}

// handle processes one notification, reporting whether the loop should carry
// on.
func (w *fsWatcher) handle(ev fsnotify.Event) bool {
	if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return true
	}

	// A new directory may hold matching files, and may have been populated
	// before this event was delivered, so rescanning both widens the watch set
	// and reports anything whose own event was missed.
	if ev.Op&fsnotify.Create != 0 {
		for _, path := range w.rescan() {
			if !w.emit(path) {
				return false
			}
		}
	}

	path, interesting := w.matches(ev.Name)
	if !interesting {
		return true
	}
	return w.emit(path)
}

func (w *fsWatcher) Events() <-chan Event { return w.events }

// Close stops the watcher and releases the underlying fsnotify resources.
// closeOnce guards against a double close of the done channel when Close is
// called from several goroutines at once.
func (w *fsWatcher) Close() error {
	w.closeOnce.Do(func() { close(w.done) })
	return w.inner.Close()
}

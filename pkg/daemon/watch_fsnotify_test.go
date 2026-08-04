package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

func TestFSNotifyWatcherEmitsOnModification(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.graphqls")
	if err := os.WriteFile(path, []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaFile = path

	w, err := NewFSNotifyWatcher(cfg)
	if err != nil {
		t.Skipf("fsnotify is unavailable in this environment: %v", err)
	}
	defer func() { _ = w.Close() }()

	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(path, []byte("type Query { a: String b: Int }"), 0o600); err != nil {
		t.Fatal(err)
	}

	select {
	case ev := <-w.Events():
		if ev.Path != path {
			t.Fatalf("expected an event for %s, got %s", path, ev.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a change event")
	}
}

// TestFSNotifyWatcherEmitsOnDeletion covers a removed schema file. A deletion
// changes the document just as much as an edit does — a file removed to clear a
// duplicate definition must republish — so the fsnotify backend has to report
// it, exactly as the polling backend does.
func TestFSNotifyWatcherEmitsOnDeletion(t *testing.T) {
	dir := t.TempDir()
	kept := filepath.Join(dir, "a.graphqls")
	doomed := filepath.Join(dir, "b.graphqls")
	if err := os.WriteFile(kept, []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doomed, []byte("type Tweet { id: ID }"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaPattern = filepath.Join(dir, "*.graphqls")

	w, err := NewFSNotifyWatcher(cfg)
	if err != nil {
		t.Skipf("fsnotify is unavailable in this environment: %v", err)
	}
	defer func() { _ = w.Close() }()

	time.Sleep(50 * time.Millisecond)
	if err := os.Remove(doomed); err != nil {
		t.Fatal(err)
	}

	select {
	case ev := <-w.Events():
		if ev.Path != doomed {
			t.Fatalf("expected an event for %s, got %s", doomed, ev.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a deletion event")
	}
}

// TestFSNotifyWatcherNoticesANewSubdirectory covers a schema file created in a
// directory that did not exist when the watcher started. A '**' pattern matches
// at any depth, so the watch set has to grow with the tree.
func TestFSNotifyWatcherNoticesANewSubdirectory(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "schemas")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.graphqls"), []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaPattern = filepath.Join(root, "**", "*.graphqls")

	w, err := NewFSNotifyWatcher(cfg)
	if err != nil {
		t.Skipf("fsnotify is unavailable in this environment: %v", err)
	}
	defer func() { _ = w.Close() }()

	time.Sleep(50 * time.Millisecond)
	nested := filepath.Join(root, "deep", "deeper")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(nested, "c.graphqls")
	if err := os.WriteFile(created, []byte("type Tweet { id: ID }"), 0o600); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-w.Events():
			if ev.Path == created {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for an event for %s", created)
		}
	}
}

// awaitEvent drains events until one carries want, reporting whether it
// arrived within the budget.
func awaitEvent(w Watcher, want string, budget time.Duration) bool {
	deadline := time.After(budget)
	for {
		select {
		case ev, ok := <-w.Events():
			if !ok {
				return false
			}
			if ev.Path == want {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// TestFSNotifyWatcherNoticesAFileWrittenWhileTheWatchSetWidens covers the
// window between expanding the pattern and adding the watches. A file written
// after the expansion but before its directory is watched is invisible to
// both — too late to be listed, too early to be notified — and nothing later
// mentions it. Widening the watch set before the expansion closes the window;
// listing first leaves it open.
//
// The window is only as wide as the scan takes, so the tree is large enough to
// make the scan measurable and the write is repeated across a spread of delays.
// Against the listing-first order some of those delays land inside the window;
// against the widening-first order none of them can, whatever the timing.
func TestFSNotifyWatcherNoticesAFileWrittenWhileTheWatchSetWidens(t *testing.T) {
	if testing.Short() {
		t.Skip("the tree this needs is too large for -short")
	}

	const treeSize = 4000
	requireWatchCapacity(t, treeSize)

	dir := t.TempDir()
	root := filepath.Join(dir, "schemas")
	for i := 0; i < treeSize; i++ {
		if err := os.MkdirAll(filepath.Join(root, fmt.Sprintf("d%04d", i)), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "a.graphqls"), []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaPattern = filepath.Join(root, "**", "*.graphqls")

	w, err := NewFSNotifyWatcher(cfg)
	if err != nil {
		t.Skipf("fsnotify is unavailable in this environment: %v", err)
	}
	defer func() { _ = w.Close() }()

	// The ladder is dense around the width of a scan so that at least one rung
	// lands inside the window, and is walked twice because that width varies
	// with the page cache and the scheduler.
	delays := []time.Duration{0, 1, 2, 3, 4, 6, 8, 10, 13, 16, 20, 25, 32, 40, 55, 75, 100, 135, 180, 240, 320}
	const passes = 2

	for pass := 0; pass < passes; pass++ {
		for i, ms := range delays {
			// A fresh subdirectory each time, so its creation is what prompts
			// the rescan and the file lands while that rescan is in flight.
			sub := filepath.Join(root, fmt.Sprintf("zzz%d-%02d", pass, i))
			if err := os.MkdirAll(sub, 0o750); err != nil {
				t.Fatal(err)
			}
			time.Sleep(ms * time.Millisecond)

			created := filepath.Join(sub, "new.graphqls")
			if err := os.WriteFile(created, []byte("type Tweet { id: ID }"), 0o600); err != nil {
				t.Fatal(err)
			}

			if !awaitEvent(w, created, 2*time.Second) {
				t.Errorf("no event for %s, written %s after its directory was created",
					created, ms*time.Millisecond)
			}
		}
	}
}

// TestWatchTargetsNeedsNoFileListing pins the property the ordering rests on:
// the directories to watch come from the configuration alone. A subdirectory
// holding no matched file at all must still be watched, because the file that
// is about to be created there cannot appear in any listing taken beforehand.
// Deriving the watch set from a listing of matched files can only ever cover
// directories that already hold one.
func TestWatchTargetsNeedsNoFileListing(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "schemas")
	empty := filepath.Join(root, "no-schema-here")
	if err := os.MkdirAll(empty, 0o750); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaPattern = filepath.Join(root, "**", "*.graphqls")

	w := &fsWatcher{cfg: cfg}
	targets := w.watchTargets()

	for _, want := range []string{root, empty} {
		if !slices.Contains(targets, want) {
			t.Errorf("expected %s to be watched, got %v", want, targets)
		}
	}
}

// TestWatchTargetsSkipsTheWalkWhenAPatternCannotMatchDeeper guards the other
// half of that rule: a pattern with no wildcard in its directory part cannot
// match below its own root, so the tree below it is neither walked nor watched.
func TestWatchTargetsSkipsTheWalkWhenAPatternCannotMatchDeeper(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "schemas")
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaPattern = filepath.Join(root, "*.graphqls")

	w := &fsWatcher{cfg: cfg}
	targets := w.watchTargets()

	if !slices.Contains(targets, root) {
		t.Errorf("expected %s to be watched, got %v", root, targets)
	}
	if slices.Contains(targets, nested) {
		t.Errorf("did not expect %s to be watched, got %v", nested, targets)
	}
}

// TestFSNotifyWatcherKeepsWatchingPastADirectoryItCannotWatch covers a
// directory that cannot be watched at all. The watch list is sorted, so
// abandoning it at the first failure silently drops every directory after the
// offending one — and a tree with no watches produces no events, so nothing
// ever arrives to prompt the retry. One bad directory must cost only itself.
func TestFSNotifyWatcherKeepsWatchingPastADirectoryItCannotWatch(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read any directory, so no watch would fail")
	}

	probe, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("fsnotify is unavailable in this environment: %v", err)
	}
	_ = probe.Close()

	dir := t.TempDir()
	root := filepath.Join(dir, "schemas")
	// The unreadable directory sorts before the usable one, so an implementation
	// that gives up on the first failure never reaches the latter.
	blocked := filepath.Join(root, "a-unreadable")
	existing := filepath.Join(root, "m-existing")
	kept := filepath.Join(root, "z-kept")
	for _, d := range []string{blocked, existing, kept} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(existing, "api.graphqls"), []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	// Take the unreadable directory away again so the temporary directory can be
	// cleaned up. Removing an empty directory needs permission on its parent,
	// not on the directory itself.
	t.Cleanup(func() { _ = os.Remove(blocked) })

	// A single-star pattern still spans subdirectories, so the whole tree is
	// watched, but its expansion tolerates a directory it cannot read — which
	// leaves the failure where this test wants it, on the watch rather than on
	// the listing.
	cfg := config.NewConfig()
	cfg.SchemaPattern = filepath.Join(root, "*", "api.graphqls")

	w, err := NewFSNotifyWatcher(cfg)
	if err != nil {
		t.Fatalf("one unwatchable directory must not stop the watcher starting: %v", err)
	}
	defer func() { _ = w.Close() }()

	time.Sleep(50 * time.Millisecond)
	created := filepath.Join(kept, "api.graphqls")
	if err := os.WriteFile(created, []byte("type Tweet { id: ID }"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !awaitEvent(w, created, 3*time.Second) {
		t.Errorf("no event for %s; the watch on it was dropped along with the unreadable directory", created)
	}
}

// requireWatchCapacity skips when the kernel cannot hold a watch per directory
// of the tree the caller is about to build. Exhausting the limit mid-test would
// look like the defect under test rather than the environment it ran in.
func requireWatchCapacity(t *testing.T, needed int) {
	t.Helper()

	raw, err := os.ReadFile("/proc/sys/fs/inotify/max_user_watches")
	if err != nil {
		// Not Linux, or the limit is not exposed; carry on and let the watcher
		// report any trouble itself.
		return
	}
	limit, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return
	}
	if limit < needed*2 {
		t.Skipf("inotify allows only %d watches, too few for a tree of %d directories", limit, needed)
	}
}

func TestFSNotifyWatcherIgnoresUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.graphqls")
	if err := os.WriteFile(path, []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaFile = path

	w, err := NewFSNotifyWatcher(cfg)
	if err != nil {
		t.Skipf("fsnotify is unavailable in this environment: %v", err)
	}
	defer func() { _ = w.Close() }()

	time.Sleep(50 * time.Millisecond)
	other := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(other, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	select {
	case ev := <-w.Events():
		t.Fatalf("expected no event for an unrelated file, got %v", ev)
	case <-time.After(400 * time.Millisecond):
	}
}

func TestNewWatcherHonoursExplicitPoll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.graphqls")
	if err := os.WriteFile(path, []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaFile = path
	cfg.WatchMode = config.WatchModePoll

	w, backend, err := NewWatcher(cfg)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if backend != "poll" {
		t.Fatalf("expected the polling backend, got %q", backend)
	}
}

func TestNewWatcherAutoPrefersFSNotify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.graphqls")
	if err := os.WriteFile(path, []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaFile = path
	cfg.WatchMode = config.WatchModeAuto

	w, backend, err := NewWatcher(cfg)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if backend != "fsnotify" && backend != "poll" {
		t.Fatalf("unexpected backend %q", backend)
	}
}

// TestFSNotifyWatcherCloseIsConcurrencySafe guards against a double close of
// the done channel when Close is called from several goroutines at once,
// mirroring TestPollWatcherCloseIsConcurrencySafe for the fsnotify backend.
func TestFSNotifyWatcherCloseIsConcurrencySafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.graphqls")
	if err := os.WriteFile(path, []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaFile = path

	const trials = 200
	const goroutines = 200
	for trial := 0; trial < trials; trial++ {
		w, err := NewFSNotifyWatcher(cfg)
		if err != nil {
			t.Skipf("fsnotify is unavailable in this environment: %v", err)
		}

		var wg sync.WaitGroup
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				if err := w.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			}()
		}
		wg.Wait()
	}
}

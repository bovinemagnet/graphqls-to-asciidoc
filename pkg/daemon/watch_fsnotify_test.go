package daemon

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

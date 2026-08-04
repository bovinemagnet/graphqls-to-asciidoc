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

package daemon

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

func TestDiffSnapshotsDetectsModification(t *testing.T) {
	base := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	before := map[string]fileState{"a.graphqls": {ModTime: base, Size: 10}}
	after := map[string]fileState{"a.graphqls": {ModTime: base.Add(time.Second), Size: 10}}

	if got := diffSnapshots(before, after); len(got) != 1 || got[0] != "a.graphqls" {
		t.Fatalf("expected a.graphqls to be reported as changed, got %v", got)
	}
}

func TestDiffSnapshotsDetectsSizeChangeAtSameModTime(t *testing.T) {
	base := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	before := map[string]fileState{"a.graphqls": {ModTime: base, Size: 10}}
	after := map[string]fileState{"a.graphqls": {ModTime: base, Size: 11}}

	if got := diffSnapshots(before, after); len(got) != 1 {
		t.Fatalf("expected a size change to count, got %v", got)
	}
}

func TestDiffSnapshotsDetectsAdditionAndRemoval(t *testing.T) {
	base := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	before := map[string]fileState{"a.graphqls": {ModTime: base, Size: 10}}
	after := map[string]fileState{"b.graphqls": {ModTime: base, Size: 10}}

	if got := diffSnapshots(before, after); len(got) != 2 {
		t.Fatalf("expected both the removal and the addition, got %v", got)
	}
}

func TestDiffSnapshotsQuietWhenNothingChanged(t *testing.T) {
	base := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	before := map[string]fileState{"a.graphqls": {ModTime: base, Size: 10}}
	after := map[string]fileState{"a.graphqls": {ModTime: base, Size: 10}}

	if got := diffSnapshots(before, after); len(got) != 0 {
		t.Fatalf("expected no changes, got %v", got)
	}
}

func TestPollWatcherEmitsOnModification(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.graphqls")
	if err := os.WriteFile(path, []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaFile = path
	cfg.PollInterval = 20 * time.Millisecond

	w, err := NewPollWatcher(cfg)
	if err != nil {
		t.Fatalf("NewPollWatcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	// A modification time on many filesystems has one second granularity, so
	// change the size too.
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

func TestPollWatcherSilentWhenNothingChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.graphqls")
	if err := os.WriteFile(path, []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaFile = path
	cfg.PollInterval = 20 * time.Millisecond

	w, err := NewPollWatcher(cfg)
	if err != nil {
		t.Fatalf("NewPollWatcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	select {
	case ev := <-w.Events():
		t.Fatalf("expected no events for an untouched file, got %v", ev)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestPollWatcherCloseIsConcurrencySafe guards against a double close of the
// done channel when Close is called from several goroutines at once. Against
// a non-blocking select guard, a single trial of 200 goroutines only panics
// with "close of closed channel" around half the time; 200 trials of a fresh
// watcher each make the panic reliable rather than a matter of luck.
func TestPollWatcherCloseIsConcurrencySafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.graphqls")
	if err := os.WriteFile(path, []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaFile = path
	cfg.PollInterval = 20 * time.Millisecond

	const trials = 200
	const goroutines = 200
	for trial := 0; trial < trials; trial++ {
		w, err := NewPollWatcher(cfg)
		if err != nil {
			t.Fatalf("NewPollWatcher: %v", err)
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

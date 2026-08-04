package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

func daemonTestConfig(t *testing.T, schema string) *config.Config {
	t.Helper()

	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.graphqls")
	if err := os.WriteFile(schemaPath, []byte(schema), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewConfig()
	cfg.SchemaFile = schemaPath
	cfg.OutputFile = filepath.Join(dir, "out.adoc")
	cfg.Daemon = true
	return cfg
}

func TestWriteAtomicReplacesTheTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.adoc")
	if err := writeAtomic(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(path, []byte("second")); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second" {
		t.Fatalf("expected the file to be replaced, got %q", got)
	}
}

func TestRebuildWritesTheOutput(t *testing.T) {
	cfg := daemonTestConfig(t, "type Query { a: String }")
	c := NewCoordinator(cfg, SystemClock(), "poll")

	c.Rebuild()

	got, err := os.ReadFile(cfg.OutputFile)
	if err != nil {
		t.Fatalf("expected the output file to exist: %v", err)
	}
	if !strings.HasPrefix(string(got), "= GraphQL Documentation") {
		t.Fatalf("expected a rendered document, got %.40q", got)
	}

	state := c.Snapshot()
	if state.LastError != "" {
		t.Fatalf("expected a clean build, got error %q", state.LastError)
	}
	if len(state.History) != 1 || state.History[0].Err != "" {
		t.Fatalf("expected one successful history record, got %+v", state.History)
	}
	if state.Mode != ModeWatching {
		t.Fatalf("expected the coordinator to return to watching, got %q", state.Mode)
	}
}

func TestRebuildRecordsAFailureAndKeepsTheOldOutput(t *testing.T) {
	cfg := daemonTestConfig(t, "type Query { a: String }")
	c := NewCoordinator(cfg, SystemClock(), "poll")

	c.Rebuild()

	// Break the schema and rebuild.
	if err := os.WriteFile(cfg.SchemaFile, []byte("type Query { a: }"), 0o600); err != nil {
		t.Fatal(err)
	}
	c.Rebuild()

	state := c.Snapshot()
	if state.LastError == "" {
		t.Fatal("expected the failed build to be recorded")
	}
	if len(state.History) != 2 || state.History[0].Err == "" {
		t.Fatalf("expected the newest record to carry the error, got %+v", state.History)
	}

	got, err := os.ReadFile(cfg.OutputFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "= GraphQL Documentation") {
		t.Fatal("expected the previous good output to survive a failed build")
	}
}

func TestSuccessClearsThePreviousError(t *testing.T) {
	cfg := daemonTestConfig(t, "type Query { a: }")
	c := NewCoordinator(cfg, SystemClock(), "poll")

	c.Rebuild()
	if c.Snapshot().LastError == "" {
		t.Fatal("expected the first build to fail")
	}

	if err := os.WriteFile(cfg.SchemaFile, []byte("type Query { a: String }"), 0o600); err != nil {
		t.Fatal(err)
	}
	c.Rebuild()

	if got := c.Snapshot().LastError; got != "" {
		t.Fatalf("expected the error to be cleared, got %q", got)
	}
}

func TestHistoryIsBounded(t *testing.T) {
	cfg := daemonTestConfig(t, "type Query { a: String }")
	c := NewCoordinator(cfg, SystemClock(), "poll")

	for i := 0; i < historyLimit+5; i++ {
		c.Rebuild()
	}

	if got := len(c.Snapshot().History); got != historyLimit {
		t.Fatalf("expected the history to be capped at %d, got %d", historyLimit, got)
	}
}

func TestSnapshotDoesNotShareTheHistorySlice(t *testing.T) {
	cfg := daemonTestConfig(t, "type Query { a: String }")
	c := NewCoordinator(cfg, SystemClock(), "poll")
	c.Rebuild()

	snapshot := c.Snapshot()
	snapshot.History[0].Err = "tampered"

	if c.Snapshot().History[0].Err == "tampered" {
		t.Fatal("Snapshot must return a copy of the history")
	}
}

# Daemon Mode with HTMX Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `--daemon` mode that watches GraphQL schema files, regenerates the AsciiDoc output after a debounce period, and serves an HTMX dashboard plus the generated document over HTTP, with optional Kroki wiring.

**Architecture:** The read → parse → generate pipeline is first extracted out of `main.go` into `pkg/build`, replacing `log.Fatalf` with returned errors so a daemon can survive a bad schema. `pkg/daemon` then adds a `Watcher` interface (fsnotify and polling backends), a clock-injected debouncer, a coordinator owning mutex-guarded state, and an HTTP server rendering `html/template` fragments driven by HTMX polling.

**Tech Stack:** Go 1.25, `github.com/fsnotify/fsnotify` (new), `net/http`, `html/template`, `go:embed`, htmx 2.0.4 vendored as a static asset.

## Global Constraints

- Go floor is 1.25.0 (`go.mod`). Do not raise it.
- Exactly one new module dependency is authorised: `github.com/fsnotify/fsnotify`. htmx is vendored as a file, not a module.
- One-shot CLI behaviour must not change. `main_test.go` and all existing package tests must keep passing after every task.
- British spelling in all user-facing text, comments and documentation.
- Author is Paul Snow. Commit messages must not mention AI tooling and must not add co-author trailers.
- Run `make test` before every commit.
- Comment density and style must match the surrounding code: short sentence-case comments above exported symbols, `//nolint:lll` on long flag-usage lines as the existing code does.

## Deviation from the spec

The spec placed the HTTP server in a `pkg/daemon/web` sub-package. That creates an
import cycle: `web` needs `daemon.State` to render, and `daemon` needs `web` to
wire the server. Rather than introduce a contrived third package purely to hold
shared structs, the server lives in `pkg/daemon` alongside the coordinator, split
across focused files (`server.go`, `views.go`). Everything else follows the spec.

## File Structure

**Created**

| File | Responsibility |
|---|---|
| `pkg/build/build.go` | `ResolveFiles`, `Run` — schema files to AsciiDoc bytes, errors returned |
| `pkg/build/build_test.go` | Pipeline tests including the "error not exit" regression guard |
| `pkg/daemon/clock.go` | `Clock` interface and the real implementation |
| `pkg/daemon/debounce.go` | `Debouncer` — decides when a change becomes a rebuild |
| `pkg/daemon/debounce_test.go` | Table-driven timing tests against a fake clock |
| `pkg/daemon/watcher.go` | `Event`, `Watcher` interface, `NewWatcher` backend selection |
| `pkg/daemon/watch_poll.go` | Polling backend and its pure snapshot/diff helpers |
| `pkg/daemon/watch_poll_test.go` | Diff unit tests plus a temp-dir integration test |
| `pkg/daemon/watch_fsnotify.go` | fsnotify backend watching parent directories |
| `pkg/daemon/watch_fsnotify_test.go` | Temp-dir integration test, skipped if init fails |
| `pkg/daemon/state.go` | `State`, `BuildRecord`, `KrokiStatus`, atomic file write |
| `pkg/daemon/coordinator.go` | Owns state, runs builds, exposes `Snapshot`/`Rebuild`/`Run` |
| `pkg/daemon/coordinator_test.go` | Build success, build failure, history bounding |
| `pkg/daemon/kroki.go` | Reachability prober and the reverse proxy handler |
| `pkg/daemon/kroki_test.go` | Prober and proxy tests against `httptest` stubs |
| `pkg/daemon/server.go` | `http.ServeMux`, routes, graceful shutdown |
| `pkg/daemon/server_test.go` | Handler tests via `httptest` |
| `pkg/daemon/views.go` | `html/template` parsing and the status view model |
| `pkg/daemon/daemon.go` | `Run(ctx, cfg)` — wires watcher, coordinator and server |
| `pkg/daemon/templates/dashboard.html` | Dashboard shell |
| `pkg/daemon/templates/status.html` | Polled status fragment |
| `pkg/daemon/assets/htmx.min.js` | Vendored htmx 2.0.4 |
| `pkg/daemon/assets/dashboard.css` | Dashboard stylesheet |
| `test/invalid.graphqls` | Deliberately malformed schema fixture |

**Modified**

| File | Change |
|---|---|
| `main.go` | Becomes a thin caller of `build.Run` / `daemon.Run` |
| `pkg/config/config.go` | Six new flags, `SetFlags` tracking, validation, `KrokiDocumentURL`, usage text |
| `pkg/config/config_test.go` | Validation table for the new rules |
| `pkg/generator/generator.go:228` | Emit Kroki attributes in `printHeader` |
| `pkg/generator/types.go:118` | `KrokiServerURL` field on `CatalogueData` |
| `pkg/generator/catalogue.go:81` | Populate `KrokiServerURL` |
| `pkg/templates/templates.go:267` | Kroki attributes in `CatalogueTemplate` |
| `README.md` | Daemon mode section |
| `go.mod` / `go.sum` | fsnotify |

---

### Task 1: Extract the build pipeline into `pkg/build`

`main.go` currently calls `log.Fatalf` at every failure point, which kills the
process. A daemon must get an error back instead. This task moves the pipeline
without changing any CLI behaviour.

**Files:**
- Create: `pkg/build/build.go`
- Create: `pkg/build/build_test.go`
- Create: `test/invalid.graphqls`
- Modify: `main.go`

**Interfaces:**
- Consumes: `config.Config`, `parser.FindSchemaFiles`, `parser.ValidateSchemaFiles`, `parser.CombineSchemaFiles`, `parser.RemoveFragments`, `parser.BuildSchema`, `generator.New`
- Produces:
  - `build.Result{Content []byte, Files []string, Duration time.Duration}`
  - `func build.ResolveFiles(cfg *config.Config) ([]string, error)`
  - `func build.Run(cfg *config.Config) (*build.Result, error)`

- [ ] **Step 1: Create the malformed schema fixture**

Create `test/invalid.graphqls`:

```graphql
type Query {
  brokenField: 
}
```

- [ ] **Step 2: Write the failing tests**

Create `pkg/build/build_test.go`:

```go
package build

import (
	"strings"
	"testing"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

func TestResolveFilesSingleFile(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaFile = "../../test/schema.graphql"

	files, err := ResolveFiles(cfg)
	if err != nil {
		t.Fatalf("ResolveFiles returned an error: %v", err)
	}
	if len(files) != 1 || files[0] != "../../test/schema.graphql" {
		t.Fatalf("expected the single schema file, got %v", files)
	}
}

func TestResolveFilesPattern(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaPattern = "../../test/multi-schema/*.graphqls"

	files, err := ResolveFiles(cfg)
	if err != nil {
		t.Fatalf("ResolveFiles returned an error: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected the pattern to match at least one file")
	}
}

func TestRunProducesDocument(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaFile = "../../test/schema.graphql"

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	if !strings.HasPrefix(string(result.Content), "= GraphQL Documentation") {
		t.Fatalf("expected a document header, got %.40q", result.Content)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected one source file, got %v", result.Files)
	}
}

// TestRunReturnsErrorOnBadSchema is the regression guard for the log.Fatalf
// extraction: a malformed schema must return an error, never exit the process.
func TestRunReturnsErrorOnBadSchema(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaFile = "../../test/invalid.graphqls"

	if _, err := Run(cfg); err == nil {
		t.Fatal("expected a parse error for the malformed schema, got nil")
	}
}

func TestRunReturnsErrorOnMissingFile(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaFile = "../../test/does-not-exist.graphqls"

	if _, err := Run(cfg); err == nil {
		t.Fatal("expected an error for a missing schema file, got nil")
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./pkg/build/ -v`
Expected: FAIL — the `build` package does not exist yet.

- [ ] **Step 4: Write the implementation**

Create `pkg/build/build.go`:

```go
// Package build turns a configuration into a rendered AsciiDoc document. It
// exists so that both the one-shot CLI and the daemon share a single pipeline
// that reports failures as errors rather than exiting the process.
package build

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/vektah/gqlparser/v2/ast"
	gqlparser "github.com/vektah/gqlparser/v2/parser"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/generator"
	schemaParser "github.com/bovinemagnet/graphqls-to-asciidoc/pkg/parser"
)

// schemaSourceName labels the combined schema passed to the GraphQL parser.
const schemaSourceName = "GraphQL schema"

// Result is a rendered document together with the sources it came from.
type Result struct {
	Content  []byte
	Files    []string
	Duration time.Duration
}

// ResolveFiles returns the schema files named by the configuration, expanding
// the glob pattern when one was given.
func ResolveFiles(cfg *config.Config) ([]string, error) {
	if cfg.SchemaPattern == "" {
		return []string{cfg.SchemaFile}, nil
	}

	files, err := schemaParser.FindSchemaFiles(cfg.SchemaPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find schema files with pattern '%s': %w", cfg.SchemaPattern, err)
	}
	if err := schemaParser.ValidateSchemaFiles(files); err != nil {
		return nil, fmt.Errorf("schema file validation failed: %w", err)
	}
	return files, nil
}

// readSchemaContent loads and combines the schema files.
func readSchemaContent(cfg *config.Config, files []string) (string, error) {
	if cfg.SchemaPattern == "" {
		// #nosec G304 -- the path is the user's own -schema argument, already
		// checked by Validate; reading it is what this tool is for.
		schemaBytes, err := os.ReadFile(cfg.SchemaFile)
		if err != nil {
			return "", fmt.Errorf("failed to read schema file %s: %w", cfg.SchemaFile, err)
		}
		return string(schemaBytes), nil
	}

	content, err := schemaParser.CombineSchemaFiles(files)
	if err != nil {
		return "", fmt.Errorf("failed to combine schema files: %w", err)
	}
	if cfg.Verbose {
		log.Printf("Combined %d schema files: %v", len(files), files)
	}
	return content, nil
}

// Run reads, parses and renders the configured schema, returning the document
// bytes. Every failure is returned rather than fatal, so callers that run
// repeatedly can report a bad schema and carry on.
func Run(cfg *config.Config) (*Result, error) {
	started := time.Now()

	files, err := ResolveFiles(cfg)
	if err != nil {
		return nil, err
	}

	schemaContent, err := readSchemaContent(cfg, files)
	if err != nil {
		return nil, err
	}

	// Fragments are client-side constructs and do not belong in schema files.
	cleanedSchema := schemaParser.RemoveFragments(schemaContent)
	if cfg.Verbose && cleanedSchema != schemaContent {
		log.Printf("Removed fragment definitions from schema")
	}

	// Code blocks in descriptions are safe to parse directly because they sit
	// inside triple-quoted strings.
	doc, gqlErr := gqlparser.ParseSchema(&ast.Source{Name: schemaSourceName, Input: cleanedSchema})
	if gqlErr != nil {
		return nil, fmt.Errorf("failed to parse GraphQL schema: %w", gqlErr)
	}

	schema := schemaParser.BuildSchema(doc)

	var out bytes.Buffer
	if err := generator.New(cfg, schema, &out).Generate(); err != nil {
		return nil, fmt.Errorf("failed to generate documentation: %w", err)
	}

	return &Result{Content: out.Bytes(), Files: files, Duration: time.Since(started)}, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./pkg/build/ -v`
Expected: PASS, all five tests.

- [ ] **Step 6: Rewrite `main.go` to use the new package**

Replace the whole of `main.go` with:

```go
package main

import (
	"log"
	"os"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/build"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

var (
	Version   = "development"
	BuildTime = "unknown"
)

func init() {
	// Set version variables in config package
	config.Version = Version
	config.BuildTime = BuildTime
}

func main() {
	cfg := config.ParseFlags()

	if cfg.HandleVersion() {
		os.Exit(0)
	}

	if cfg.HandleHelp() {
		os.Exit(0)
	}

	if err := cfg.Validate(); err != nil {
		config.PrintError(err.Error())
		os.Exit(1)
	}

	result, err := build.Run(cfg)
	if err != nil {
		log.Fatalf("%v", err)
	}

	outputWriter, shouldClose, err := cfg.GetOutputWriter()
	if err != nil {
		log.Fatalf("Failed to setup output: %v", err)
	}

	if _, err := outputWriter.Write(result.Content); err != nil {
		log.Fatalf("Failed to write output: %v", err)
	}

	if shouldClose {
		if closeErr := outputWriter.Close(); closeErr != nil {
			log.Fatalf("Failed to close output file: %v", closeErr)
		}
	}
}
```

- [ ] **Step 7: Run the full test suite**

Run: `make test`
Expected: PASS. `main_test.go` exercises the CLI end to end; it must be
unaffected.

- [ ] **Step 8: Verify the CLI output is byte-identical apart from the timestamp**

```bash
go build -o /tmp/g2a-new . && git stash && go build -o /tmp/g2a-old . && git stash pop
/tmp/g2a-old -s test/schema.graphql | grep -v '^:revdate:' > /tmp/old.adoc
/tmp/g2a-new -s test/schema.graphql | grep -v '^:revdate:' > /tmp/new.adoc
diff /tmp/old.adoc /tmp/new.adoc && echo IDENTICAL
```
Expected: `IDENTICAL`.

- [ ] **Step 9: Commit**

```bash
git add pkg/build main.go test/invalid.graphqls
git commit -m "Extract the document pipeline into pkg/build (#44)

Replaces the log.Fatalf calls in main.go with returned errors so the
pipeline can be re-run without exiting the process."
```

---

### Task 2: Daemon configuration flags and validation

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `pkg/config/config_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks
- Produces:
  - `Config` fields: `Daemon bool`, `DaemonAddr string`, `Debounce time.Duration`, `WatchMode string`, `PollInterval time.Duration`, `KrokiURL string`, `SetFlags map[string]bool`
  - `func (c *Config) KrokiDocumentURL() string`
  - Watch mode constants `WatchModeAuto`, `WatchModeFSNotify`, `WatchModePoll` (values `"auto"`, `"fsnotify"`, `"poll"`)

- [ ] **Step 1: Write the failing tests**

Append to `pkg/config/config_test.go`:

```go
func TestValidateDaemonRequiresOutput(t *testing.T) {
	cfg := NewConfig()
	cfg.SchemaFile = "../../test/schema.graphql"
	cfg.Daemon = true

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "--daemon requires") {
		t.Fatalf("expected a missing-output error, got %v", err)
	}
}

func TestValidateDaemonFlagsWithoutDaemon(t *testing.T) {
	cases := []string{"daemon-addr", "debounce", "watch-mode", "poll-interval"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.SchemaFile = "../../test/schema.graphql"
			cfg.SetFlags = map[string]bool{name: true}

			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("expected --%s to be rejected without --daemon, got %v", name, err)
			}
		})
	}
}

func TestValidateDaemonAccepts(t *testing.T) {
	cfg := NewConfig()
	cfg.SchemaFile = "../../test/schema.graphql"
	cfg.OutputFile = "out.adoc"
	cfg.Daemon = true
	cfg.SetFlags = map[string]bool{"debounce": true, "watch-mode": true}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected a valid daemon configuration, got %v", err)
	}
}

func TestValidateWatchMode(t *testing.T) {
	cfg := NewConfig()
	cfg.SchemaFile = "../../test/schema.graphql"
	cfg.OutputFile = "out.adoc"
	cfg.Daemon = true
	cfg.WatchMode = "magic"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "watch-mode") {
		t.Fatalf("expected an invalid watch-mode error, got %v", err)
	}
}

func TestValidateNonPositiveDurations(t *testing.T) {
	cfg := NewConfig()
	cfg.SchemaFile = "../../test/schema.graphql"
	cfg.OutputFile = "out.adoc"
	cfg.Daemon = true
	cfg.Debounce = 0

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "debounce") {
		t.Fatalf("expected a non-positive debounce error, got %v", err)
	}
}

func TestKrokiDocumentURL(t *testing.T) {
	tests := []struct {
		name   string
		kroki  string
		daemon bool
		addr   string
		want   string
	}{
		{"disabled", "", false, "", ""},
		{"one-shot uses the raw url", "https://kroki.io", false, "", "https://kroki.io"},
		{"daemon uses the local proxy", "https://kroki.io", true, "127.0.0.1:8088", "http://127.0.0.1:8088/kroki"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.KrokiURL = tt.kroki
			cfg.Daemon = tt.daemon
			cfg.DaemonAddr = tt.addr

			if got := cfg.KrokiDocumentURL(); got != tt.want {
				t.Fatalf("KrokiDocumentURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

If `strings` and `testing` are not already imported in `config_test.go`, add
`strings` to the import block.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/config/ -run 'Daemon|WatchMode|Kroki|NonPositive' -v`
Expected: FAIL — compile errors for the unknown `Daemon`, `SetFlags`,
`WatchMode`, `Debounce`, `KrokiURL` fields and `KrokiDocumentURL` method.

- [ ] **Step 3: Add the fields, constants and defaults**

In `pkg/config/config.go`, add `"strings"` and `"time"` to the imports, then add
to the `Config` struct after `IncludeChangelog`:

```go
	Daemon               bool
	DaemonAddr           string
	Debounce             time.Duration
	WatchMode            string
	PollInterval         time.Duration
	KrokiURL             string
	// SetFlags records which flags were explicitly given on the command line,
	// so that daemon-only flags can be rejected outside daemon mode.
	SetFlags map[string]bool
```

Add the watch mode constants above `NewConfig`:

```go
// Watch backends selectable with --watch-mode.
const (
	WatchModeAuto     = "auto"
	WatchModeFSNotify = "fsnotify"
	WatchModePoll     = "poll"
)

// daemonOnlyFlags are rejected unless --daemon is also given. --kroki-url is
// deliberately absent: it also affects one-shot output.
var daemonOnlyFlags = []string{"daemon-addr", "debounce", "watch-mode", "poll-interval"}
```

Extend `NewConfig`'s returned struct literal with:

```go
		DaemonAddr:           "127.0.0.1:8088",
		Debounce:             5 * time.Second,
		WatchMode:            WatchModeAuto,
		PollInterval:         time.Second,
		SetFlags:             map[string]bool{},
```

- [ ] **Step 4: Register the flags**

In `ParseFlags`, after the `--sub-title` registration, add:

```go
	// Daemon flags
	flag.BoolVar(&config.Daemon, "daemon", false, "Watch the schema files and serve a dashboard, rebuilding on change")
	flag.StringVar(&config.DaemonAddr, "daemon-addr", config.DaemonAddr, "Address the daemon dashboard listens on")
	//nolint:lll // flag usage text
	flag.DurationVar(&config.Debounce, "debounce", config.Debounce, "Quiet period that must pass after the last change before a rebuild")
	//nolint:lll // flag usage text
	flag.StringVar(&config.WatchMode, "watch-mode", config.WatchMode, "Watch backend: auto, fsnotify or poll")
	//nolint:lll // flag usage text
	flag.DurationVar(&config.PollInterval, "poll-interval", config.PollInterval, "Filesystem scan interval used by the polling backend")
	//nolint:lll // flag usage text
	flag.StringVar(&config.KrokiURL, "kroki-url", "", "Kroki server used to render diagrams, e.g. https://kroki.io")
```

Replace the trailing `flag.Parse()` / `return config` with:

```go
	flag.Parse()

	flag.Visit(func(f *flag.Flag) {
		config.SetFlags[f.Name] = true
	})

	return config
}
```

- [ ] **Step 5: Add the validation rules**

In `Validate`, immediately before the closing `return nil`, add:

```go
	if err := c.validateDaemon(); err != nil {
		return err
	}

	return nil
}

// validateDaemon checks the daemon flag group. Daemon-only flags are rejected
// outside daemon mode rather than silently ignored.
func (c *Config) validateDaemon() error {
	if !c.Daemon {
		for _, name := range daemonOnlyFlags {
			if c.SetFlags[name] {
				return fmt.Errorf("--%s requires --daemon", name)
			}
		}
		return nil
	}

	if c.OutputFile == "" {
		return fmt.Errorf("--daemon requires -o/--output; there is nothing to serve otherwise")
	}

	switch c.WatchMode {
	case WatchModeAuto, WatchModeFSNotify, WatchModePoll:
	default:
		return fmt.Errorf("--watch-mode must be one of auto, fsnotify or poll, got '%s'", c.WatchMode)
	}

	if c.Debounce <= 0 {
		return fmt.Errorf("--debounce must be greater than zero")
	}

	if c.PollInterval <= 0 {
		return fmt.Errorf("--poll-interval must be greater than zero")
	}

	return nil
}
```

Take care that the original `return nil` at the end of `Validate` is replaced,
not duplicated.

- [ ] **Step 6: Add `KrokiDocumentURL`**

Add below `validateDaemon`:

```go
// KrokiDocumentURL is the value written into the generated document's
// :kroki-server-url: attribute. In daemon mode it points at the daemon's own
// proxy so the browser-side renderer makes same-origin requests.
func (c *Config) KrokiDocumentURL() string {
	if c.KrokiURL == "" {
		return ""
	}
	if c.Daemon {
		return "http://" + c.DaemonAddr + "/kroki"
	}
	return strings.TrimSuffix(c.KrokiURL, "/")
}
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./pkg/config/ -v`
Expected: PASS.

- [ ] **Step 8: Update the usage text**

In `usageText`, add to the `OPTIONS` block after the `--sub-title` line:

```
        --kroki-url URL     Kroki server used to render diagrams, e.g. https://kroki.io

DAEMON MODE:
        --daemon            Watch the schema files and serve a dashboard, rebuilding on change
                            (requires -o/--output)
        --daemon-addr ADDR  Dashboard listen address (default: 127.0.0.1:8088)
        --debounce DUR      Quiet period after the last change before rebuilding (default: 5s)
        --watch-mode MODE   Watch backend: auto, fsnotify or poll (default: auto)
        --poll-interval DUR Filesystem scan interval for the polling backend (default: 1s)
```

And add to `EXAMPLES`:

```
    # Watch a schema and serve the dashboard on http://127.0.0.1:8088
    graphqls-to-asciidoc -s schema.graphql -o docs.adoc --daemon

    # Watch a tree of schemas with a two second debounce and a Kroki server
    graphqls-to-asciidoc -p "schemas/**/*.graphqls" -o docs.adoc --daemon \
        --debounce 2s --kroki-url https://kroki.io
```

- [ ] **Step 9: Run the full suite and commit**

Run: `make test`
Expected: PASS.

```bash
git add pkg/config
git commit -m "Add daemon mode configuration flags and validation (#44)"
```

---

### Task 3: Emit Kroki attributes in generated documents

**Files:**
- Modify: `pkg/generator/generator.go` (in `printHeader`, around line 233)
- Modify: `pkg/generator/types.go` (`CatalogueData`, around line 118)
- Modify: `pkg/generator/catalogue.go` (around line 81)
- Modify: `pkg/templates/templates.go` (`CatalogueTemplate`, around line 267)
- Modify: `pkg/generator/generator_test.go`

**Interfaces:**
- Consumes: `config.Config.KrokiDocumentURL()` from Task 2
- Produces: `CatalogueData.KrokiServerURL string`

- [ ] **Step 1: Write the failing tests**

`pkg/generator/generator_test.go` is an internal test file (`package generator`),
so `printHeader` is callable directly. Append:

```go
func TestPrintHeaderEmitsKrokiAttributes(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaFile = "schema.graphql"
	cfg.KrokiURL = "https://kroki.io"

	var out bytes.Buffer
	New(cfg, &ast.Schema{}, &out).printHeader()

	got := out.String()
	if !strings.Contains(got, ":kroki-server-url: https://kroki.io") {
		t.Errorf("expected the kroki-server-url attribute, got:\n%s", got)
	}
	if !strings.Contains(got, ":kroki-fetch-diagram:") {
		t.Errorf("expected the kroki-fetch-diagram attribute, got:\n%s", got)
	}
}

func TestPrintHeaderOmitsKrokiWhenUnset(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaFile = "schema.graphql"

	var out bytes.Buffer
	New(cfg, &ast.Schema{}, &out).printHeader()

	if strings.Contains(out.String(), "kroki") {
		t.Errorf("expected no kroki attributes, got:\n%s", out.String())
	}
}
```

`bytes`, `strings`, `ast` and `config` are already imported by that file.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/generator/ -run Kroki -v`
Expected: FAIL — no kroki attributes are emitted.

- [ ] **Step 3: Emit the attributes in `printHeader`**

In `pkg/generator/generator.go`, directly after the `:sourceFile:` line
(currently line 233), insert:

```go
	if krokiURL := g.config.KrokiDocumentURL(); krokiURL != "" {
		fmt.Fprintf(g.writer, ":kroki-server-url: %s\n", krokiURL)
		fmt.Fprintln(g.writer, ":kroki-fetch-diagram:")
	}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/generator/ -run Kroki -v`
Expected: PASS.

- [ ] **Step 5: Add the same support to catalogue mode**

In `pkg/generator/types.go`, add to `CatalogueData` after `CommandLine`:

```go
	KrokiServerURL string
```

In `pkg/generator/catalogue.go`, add to the returned `CatalogueData` literal
after `CommandLine`:

```go
		KrokiServerURL: g.config.KrokiDocumentURL(),
```

In `pkg/templates/templates.go`, in `CatalogueTemplate`, replace the line
`:commandline: {{.CommandLine}}` with:

```
:commandline: {{.CommandLine}}
{{- if .KrokiServerURL}}
:kroki-server-url: {{.KrokiServerURL}}
:kroki-fetch-diagram:
{{- end}}
```

- [ ] **Step 6: Write and run the catalogue test**

This test goes in `pkg/build/build_test.go`, not `generator_test.go`: the
generator test file is internal to `package generator`, and `build` imports
`generator`, so importing `build` there would be an import cycle. Append to
`pkg/build/build_test.go`:

```go
func TestCatalogueEmitsKrokiAttributes(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaFile = "../../test/schema.graphql"
	cfg.Catalogue = true
	cfg.KrokiURL = "https://kroki.io"

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if !strings.Contains(string(result.Content), ":kroki-server-url: https://kroki.io") {
		t.Error("expected the catalogue header to carry the kroki-server-url attribute")
	}
}
```

Run: `go test ./pkg/generator/ ./pkg/build/ -v`
Expected: PASS.

- [ ] **Step 7: Run the full suite and commit**

Run: `make test`
Expected: PASS. Existing golden `.adoc` fixtures are unaffected because no
Kroki URL is configured in them.

```bash
git add pkg/generator pkg/templates pkg/build
git commit -m "Emit Kroki attributes when a Kroki server is configured (#44)"
```

---

### Task 4: Clock and debouncer

**Files:**
- Create: `pkg/daemon/clock.go`
- Create: `pkg/daemon/debounce.go`
- Create: `pkg/daemon/debounce_test.go`

**Interfaces:**
- Consumes: nothing
- Produces:
  - `type Clock interface { Now() time.Time }`
  - `func SystemClock() Clock`
  - `func NewDebouncer(quiet time.Duration, clock Clock) *Debouncer`
  - `func (d *Debouncer) Changed()`
  - `func (d *Debouncer) Ready() bool`
  - `func (d *Debouncer) BuildStarted()`
  - `func (d *Debouncer) BuildCompleted()`

- [ ] **Step 1: Write the failing tests**

Create `pkg/daemon/debounce_test.go`:

```go
package daemon

import (
	"testing"
	"time"
)

// fakeClock lets the timing rules be tested without sleeping.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time      { return c.now }
func (c *fakeClock) advance(d time.Duration) { c.now = c.now.Add(d) }

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)}
}

func TestNotReadyWithoutAChange(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	clock.advance(time.Hour)
	if d.Ready() {
		t.Fatal("expected no rebuild without a change")
	}
}

func TestNotReadyInsideTheQuietPeriod(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	d.Changed()
	clock.advance(4 * time.Second)
	if d.Ready() {
		t.Fatal("expected no rebuild four seconds after a change")
	}
}

func TestReadyAfterTheQuietPeriod(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	d.Changed()
	clock.advance(5 * time.Second)
	if !d.Ready() {
		t.Fatal("expected a rebuild five seconds after a change")
	}
}

// A burst of saves keeps resetting the quiet period, so no build happens while
// the user is still typing.
func TestBurstOfChangesResetsTheQuietPeriod(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	for i := 0; i < 10; i++ {
		d.Changed()
		clock.advance(2 * time.Second)
		if d.Ready() {
			t.Fatalf("expected no rebuild during the burst, fired at iteration %d", i)
		}
	}

	clock.advance(3 * time.Second)
	if !d.Ready() {
		t.Fatal("expected a rebuild once the burst settled")
	}
}

// The second condition from the spec: a rebuild may not follow within the
// debounce interval of the previous one.
func TestRateLimitedByTheLastBuild(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	d.Changed()
	clock.advance(5 * time.Second)
	d.BuildStarted()
	d.BuildCompleted()

	d.Changed()
	clock.advance(5 * time.Second)
	if d.Ready() {
		t.Fatal("expected the previous build to rate-limit this one")
	}

	clock.advance(time.Second)
	if !d.Ready() {
		t.Fatal("expected a rebuild once both conditions held")
	}
}

func TestChangeDuringABuildIsNotLost(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	d.Changed()
	clock.advance(5 * time.Second)
	d.BuildStarted()

	// The user saves again while the build is running.
	d.Changed()
	d.BuildCompleted()

	clock.advance(10 * time.Second)
	if !d.Ready() {
		t.Fatal("expected the change made during the build to trigger another one")
	}
}

func TestBuildStartedClearsThePendingChange(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	d.Changed()
	clock.advance(5 * time.Second)
	d.BuildStarted()
	d.BuildCompleted()

	clock.advance(time.Hour)
	if d.Ready() {
		t.Fatal("expected no rebuild once the pending change was consumed")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/daemon/ -v`
Expected: FAIL — the package does not exist.

- [ ] **Step 3: Write the clock**

Create `pkg/daemon/clock.go`:

```go
package daemon

import "time"

// Clock is the daemon's view of time. Injecting it keeps the debounce rules
// testable without sleeping.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// SystemClock returns the wall-clock implementation used in production.
func SystemClock() Clock { return systemClock{} }
```

- [ ] **Step 4: Write the debouncer**

Create `pkg/daemon/debounce.go`:

```go
package daemon

import "time"

// Debouncer decides when a run of filesystem changes has settled enough to be
// worth rebuilding. Two conditions must hold, both taken from the design: the
// quiet period must have elapsed since the last change, and the same interval
// must have elapsed since the last build.
type Debouncer struct {
	quiet      time.Duration
	clock      Clock
	lastChange time.Time
	lastBuild  time.Time
	pending    bool
}

// NewDebouncer returns a debouncer with the given quiet period.
func NewDebouncer(quiet time.Duration, clock Clock) *Debouncer {
	return &Debouncer{quiet: quiet, clock: clock}
}

// Changed records that a watched file was modified.
func (d *Debouncer) Changed() {
	d.lastChange = d.clock.Now()
	d.pending = true
}

// Ready reports whether a pending change has settled and the rate limit allows
// another build.
func (d *Debouncer) Ready() bool {
	if !d.pending {
		return false
	}

	now := d.clock.Now()
	if now.Sub(d.lastChange) < d.quiet {
		return false
	}
	if !d.lastBuild.IsZero() && now.Sub(d.lastBuild) < d.quiet {
		return false
	}
	return true
}

// BuildStarted consumes the pending change. A change arriving during the build
// sets it again, so nothing is lost.
func (d *Debouncer) BuildStarted() {
	d.pending = false
}

// BuildCompleted starts the rate-limit window for the next build.
func (d *Debouncer) BuildCompleted() {
	d.lastBuild = d.clock.Now()
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./pkg/daemon/ -v`
Expected: PASS, all seven tests.

- [ ] **Step 6: Commit**

```bash
git add pkg/daemon
git commit -m "Add the daemon clock and debouncer (#44)"
```

---

### Task 5: Watcher interface and polling backend

**Files:**
- Create: `pkg/daemon/watcher.go`
- Create: `pkg/daemon/watch_poll.go`
- Create: `pkg/daemon/watch_poll_test.go`

**Interfaces:**
- Consumes: `build.ResolveFiles` from Task 1, `config.Config` from Task 2
- Produces:
  - `type Event struct { Path string }`
  - `type Watcher interface { Events() <-chan Event; Close() error }`
  - `func NewPollWatcher(cfg *config.Config) (Watcher, error)`
  - `type fileState struct { ModTime time.Time; Size int64 }`
  - `func snapshotFiles(paths []string) map[string]fileState`
  - `func diffSnapshots(before, after map[string]fileState) []string`

- [ ] **Step 1: Write the failing tests**

Create `pkg/daemon/watch_poll_test.go`:

```go
package daemon

import (
	"os"
	"path/filepath"
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/daemon/ -run Poll -v`
Expected: FAIL — `NewPollWatcher`, `fileState` and `diffSnapshots` are undefined.

- [ ] **Step 3: Write the watcher interface**

Create `pkg/daemon/watcher.go`:

```go
package daemon

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
```

- [ ] **Step 4: Write the polling backend**

Create `pkg/daemon/watch_poll.go`:

```go
package daemon

import (
	"os"
	"sort"
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
	cfg    *config.Config
	events chan Event
	done   chan struct{}
}

// NewPollWatcher starts a polling watcher for the configured schema files.
func NewPollWatcher(cfg *config.Config) (Watcher, error) {
	files, err := build.ResolveFiles(cfg)
	if err != nil {
		return nil, err
	}

	w := &pollWatcher{
		cfg:    cfg,
		events: make(chan Event, 16),
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
	select {
	case <-w.done:
	default:
		close(w.done)
	}
	return nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./pkg/daemon/ -race -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/daemon
git commit -m "Add the Watcher interface and polling backend (#44)"
```

---

### Task 6: fsnotify backend and automatic selection

**Files:**
- Create: `pkg/daemon/watch_fsnotify.go`
- Create: `pkg/daemon/watch_fsnotify_test.go`
- Modify: `pkg/daemon/watcher.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: `Watcher`, `Event`, `NewPollWatcher` from Task 5
- Produces:
  - `func NewFSNotifyWatcher(cfg *config.Config) (Watcher, error)`
  - `func NewWatcher(cfg *config.Config) (Watcher, string, error)` — the second
    return value is the backend name, `"fsnotify"` or `"poll"`

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/fsnotify/fsnotify@latest
go mod tidy
```

- [ ] **Step 2: Write the failing tests**

Create `pkg/daemon/watch_fsnotify_test.go`:

```go
package daemon

import (
	"os"
	"path/filepath"
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
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./pkg/daemon/ -run 'FSNotify|NewWatcher' -v`
Expected: FAIL — `NewFSNotifyWatcher` and `NewWatcher` are undefined.

- [ ] **Step 4: Write the fsnotify backend**

Create `pkg/daemon/watch_fsnotify.go`:

```go
package daemon

import (
	"path/filepath"
	"sort"

	"github.com/fsnotify/fsnotify"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/build"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

// fsWatcher reports changes through the operating system's notification
// interface. It watches the directories holding the schema files rather than
// the files themselves, so that newly created files are noticed and editors
// that save by writing a temporary file and renaming it are handled.
type fsWatcher struct {
	cfg    *config.Config
	inner  *fsnotify.Watcher
	events chan Event
	done   chan struct{}
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

func (w *fsWatcher) Close() error {
	select {
	case <-w.done:
	default:
		close(w.done)
	}
	return w.inner.Close()
}
```

- [ ] **Step 5: Add backend selection**

Append to `pkg/daemon/watcher.go`:

```go
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
```

Add the imports `"fmt"`, `"log"` and
`"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"` to
`pkg/daemon/watcher.go`.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./pkg/daemon/ -race -v`
Expected: PASS. The two fsnotify tests may report SKIP in a restricted
environment; that is acceptable.

- [ ] **Step 7: Commit**

```bash
git add pkg/daemon go.mod go.sum
git commit -m "Add the fsnotify watch backend with a polling fallback (#44)"
```

---

### Task 7: State, history and the build coordinator

**Files:**
- Create: `pkg/daemon/state.go`
- Create: `pkg/daemon/coordinator.go`
- Create: `pkg/daemon/coordinator_test.go`

**Interfaces:**
- Consumes: `build.Run` (Task 1), `Debouncer` and `Clock` (Task 4), `Watcher` (Tasks 5–6)
- Produces:
  - `type BuildRecord struct { Started time.Time; Duration time.Duration; Bytes int; Err string }`
  - `type KrokiStatus struct { Enabled bool; Reachable bool; LastCheck time.Time; URL string }`
  - `type State struct { Mode, Backend string; LastBuild time.Time; LastError string; Watched []string; History []BuildRecord; Kroki KrokiStatus }`
  - `func writeAtomic(path string, data []byte) error`
  - `func NewCoordinator(cfg *config.Config, clock Clock, backend string) *Coordinator`
  - `func (c *Coordinator) Snapshot() State`
  - `func (c *Coordinator) Rebuild()`
  - `func (c *Coordinator) SetKroki(status KrokiStatus)`
  - `func (c *Coordinator) Run(ctx context.Context, w Watcher)`
  - Constants `ModeWatching = "watching"`, `ModeBuilding = "building"`, `historyLimit = 20`

- [ ] **Step 1: Write the failing tests**

Create `pkg/daemon/coordinator_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/daemon/ -run 'Rebuild|History|Atomic|Snapshot|Success' -v`
Expected: FAIL — `NewCoordinator`, `writeAtomic`, `ModeWatching` and
`historyLimit` are undefined.

- [ ] **Step 3: Write the state types**

Create `pkg/daemon/state.go`:

```go
package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Daemon modes reported on the dashboard.
const (
	ModeWatching = "watching"
	ModeBuilding = "building"
)

// historyLimit caps the number of build records kept in memory.
const historyLimit = 20

// BuildRecord is one completed build, successful or not.
type BuildRecord struct {
	Started  time.Time
	Duration time.Duration
	Bytes    int
	Err      string
}

// Succeeded reports whether the build produced a document.
func (r BuildRecord) Succeeded() bool { return r.Err == "" }

// KrokiStatus is the result of the most recent reachability probe.
type KrokiStatus struct {
	Enabled   bool
	Reachable bool
	LastCheck time.Time
	URL       string
}

// State is everything the dashboard renders. It is always handed out as a copy.
type State struct {
	Mode      string
	Backend   string
	LastBuild time.Time
	LastError string
	Watched   []string
	History   []BuildRecord // newest first
	Kroki     KrokiStatus
}

// writeAtomic writes data to a temporary file in the destination directory and
// renames it over the target, so a reader never sees a half-written document.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("failed to create a temporary file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to close %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to set permissions on %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to replace %s: %w", path, err)
	}
	return nil
}
```

- [ ] **Step 4: Write the coordinator**

Create `pkg/daemon/coordinator.go`:

```go
package daemon

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/build"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

// Coordinator owns the daemon's state and is the only thing that runs builds.
// Everything it exposes is safe for concurrent use, so HTTP handlers can read
// the state and request rebuilds while the watch loop is running.
type Coordinator struct {
	cfg   *config.Config
	clock Clock

	mu    sync.Mutex
	state State

	// buildMu serialises builds so a manual rebuild and a watched change cannot
	// write the output file at the same time.
	buildMu sync.Mutex
}

// NewCoordinator returns a coordinator for the given configuration. backend is
// the watch backend name reported on the dashboard.
func NewCoordinator(cfg *config.Config, clock Clock, backend string) *Coordinator {
	return &Coordinator{
		cfg:   cfg,
		clock: clock,
		state: State{
			Mode:    ModeWatching,
			Backend: backend,
			Kroki:   KrokiStatus{Enabled: cfg.KrokiURL != "", URL: cfg.KrokiURL},
		},
	}
}

// Snapshot returns a copy of the current state.
func (c *Coordinator) Snapshot() State {
	c.mu.Lock()
	defer c.mu.Unlock()

	snapshot := c.state
	snapshot.History = append([]BuildRecord(nil), c.state.History...)
	snapshot.Watched = append([]string(nil), c.state.Watched...)
	return snapshot
}

// SetKroki records the result of a reachability probe.
func (c *Coordinator) SetKroki(status KrokiStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.Kroki = status
}

// Rebuild renders the document and replaces the output file. A failure is
// recorded and reported; the previous good output is left in place.
func (c *Coordinator) Rebuild() {
	c.buildMu.Lock()
	defer c.buildMu.Unlock()

	c.setMode(ModeBuilding)
	defer c.setMode(ModeWatching)

	started := c.clock.Now()
	record := BuildRecord{Started: started}

	result, err := build.Run(c.cfg)
	switch {
	case err != nil:
		record.Err = err.Error()
	default:
		record.Bytes = len(result.Content)
		if writeErr := writeAtomic(c.cfg.OutputFile, result.Content); writeErr != nil {
			record.Err = writeErr.Error()
		}
	}
	record.Duration = c.clock.Now().Sub(started)

	c.record(record, result)
}

// record folds a finished build into the state.
func (c *Coordinator) record(r BuildRecord, result *build.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state.LastBuild = r.Started
	c.state.LastError = r.Err
	if result != nil {
		c.state.Watched = append([]string(nil), result.Files...)
	}

	c.state.History = append([]BuildRecord{r}, c.state.History...)
	if len(c.state.History) > historyLimit {
		c.state.History = c.state.History[:historyLimit]
	}

	if r.Err != "" {
		log.Printf("build failed: %s", r.Err)
	} else if c.cfg.Verbose {
		log.Printf("built %s (%d bytes) in %s", c.cfg.OutputFile, r.Bytes, r.Duration)
	}
}

func (c *Coordinator) setMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.Mode = mode
}

// Run drives the watch loop until the context is cancelled. Changes are fed to
// the debouncer, and a ticker gives it the chance to fire once a burst settles.
func (c *Coordinator) Run(ctx context.Context, w Watcher) {
	debouncer := NewDebouncer(c.cfg.Debounce, c.clock)

	tick := c.cfg.Debounce / 10
	if tick < 50*time.Millisecond {
		tick = 50 * time.Millisecond
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case _, ok := <-w.Events():
			if !ok {
				return
			}
			debouncer.Changed()

		case <-ticker.C:
			if !debouncer.Ready() {
				continue
			}
			debouncer.BuildStarted()
			c.Rebuild()
			debouncer.BuildCompleted()
		}
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./pkg/daemon/ -race -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/daemon
git commit -m "Add the daemon state model and build coordinator (#44)"
```

---

### Task 8: Kroki prober and reverse proxy

**Files:**
- Create: `pkg/daemon/kroki.go`
- Create: `pkg/daemon/kroki_test.go`

**Interfaces:**
- Consumes: `KrokiStatus`, `Coordinator.SetKroki` from Task 7
- Produces:
  - `func NewKrokiProber(rawURL string) (*KrokiProber, error)`
  - `func (p *KrokiProber) Probe(ctx context.Context, now time.Time) KrokiStatus`
  - `func (p *KrokiProber) Run(ctx context.Context, clock Clock, c *Coordinator)`
  - `func NewKrokiProxy(rawURL string) (http.Handler, error)`
  - `const krokiProbeInterval = 30 * time.Second`

- [ ] **Step 1: Write the failing tests**

Create `pkg/daemon/kroki_test.go`:

```go
package daemon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestKrokiProbeReportsReachable(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer stub.Close()

	p, err := NewKrokiProber(stub.URL)
	if err != nil {
		t.Fatal(err)
	}

	status := p.Probe(context.Background(), time.Now())
	if !status.Enabled || !status.Reachable {
		t.Fatalf("expected a reachable server, got %+v", status)
	}
}

func TestKrokiProbeReportsUnreachable(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	stub.Close() // nothing is listening now

	p, err := NewKrokiProber(stub.URL)
	if err != nil {
		t.Fatal(err)
	}

	if status := p.Probe(context.Background(), time.Now()); status.Reachable {
		t.Fatal("expected an unreachable server")
	}
}

func TestKrokiProbeTreatsServerErrorsAsUnreachable(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer stub.Close()

	p, err := NewKrokiProber(stub.URL)
	if err != nil {
		t.Fatal(err)
	}

	if status := p.Probe(context.Background(), time.Now()); status.Reachable {
		t.Fatal("expected a 502 to count as unreachable")
	}
}

func TestKrokiProxyForwardsTheStrippedPath(t *testing.T) {
	var seen string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Path
		_, _ = io.WriteString(w, "<svg/>")
	}))
	defer stub.Close()

	proxy, err := NewKrokiProxy(stub.URL)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/kroki/", http.StripPrefix("/kroki", proxy))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/kroki/mermaid/svg/abc123", nil))

	if seen != "/mermaid/svg/abc123" {
		t.Fatalf("expected the /kroki prefix to be stripped, upstream saw %q", seen)
	}
	if !strings.Contains(rec.Body.String(), "<svg/>") {
		t.Fatalf("expected the upstream body to be relayed, got %q", rec.Body.String())
	}
}

func TestNewKrokiProxyRejectsABadURL(t *testing.T) {
	if _, err := NewKrokiProxy("://not a url"); err == nil {
		t.Fatal("expected an error for a malformed Kroki URL")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/daemon/ -run Kroki -v`
Expected: FAIL — `NewKrokiProber` and `NewKrokiProxy` are undefined.

- [ ] **Step 3: Write the implementation**

Create `pkg/daemon/kroki.go`:

```go
package daemon

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// krokiProbeInterval is how often the daemon re-checks the Kroki server.
const krokiProbeInterval = 30 * time.Second

// KrokiProber checks that the configured Kroki server answers. A failing probe
// never fails a build; it only changes the dashboard indicator.
type KrokiProber struct {
	url    string
	client *http.Client
}

// NewKrokiProber returns a prober for the given server.
func NewKrokiProber(rawURL string) (*KrokiProber, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Kroki URL '%s'", rawURL)
	}
	return &KrokiProber{url: rawURL, client: &http.Client{Timeout: 5 * time.Second}}, nil
}

// Probe performs one reachability check.
func (p *KrokiProber) Probe(ctx context.Context, now time.Time) KrokiStatus {
	status := KrokiStatus{Enabled: true, URL: p.url, LastCheck: now}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return status
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return status
	}
	defer func() { _ = resp.Body.Close() }()

	// Any answer short of a server error means Kroki is up; the root path is
	// not guaranteed to be a 200.
	status.Reachable = resp.StatusCode < http.StatusInternalServerError
	return status
}

// Run probes immediately and then on a fixed interval until the context ends.
func (p *KrokiProber) Run(ctx context.Context, clock Clock, c *Coordinator) {
	c.SetKroki(p.Probe(ctx, clock.Now()))

	ticker := time.NewTicker(krokiProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.SetKroki(p.Probe(ctx, clock.Now()))
		}
	}
}

// NewKrokiProxy relays requests to the Kroki server so the browser-side
// renderer makes same-origin requests and is not blocked by CORS.
func NewKrokiProxy(rawURL string) (http.Handler, error) {
	target, err := url.Parse(rawURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("invalid Kroki URL '%s'", rawURL)
	}
	return httputil.NewSingleHostReverseProxy(target), nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/daemon/ -race -run Kroki -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/daemon
git commit -m "Add the Kroki reachability probe and reverse proxy (#44)"
```

---

### Task 9: Dashboard templates, assets and the status view

**Files:**
- Create: `pkg/daemon/templates/dashboard.html`
- Create: `pkg/daemon/templates/status.html`
- Create: `pkg/daemon/assets/dashboard.css`
- Create: `pkg/daemon/assets/htmx.min.js` (vendored)
- Create: `pkg/daemon/views.go`
- Create: `pkg/daemon/views_test.go`

**Interfaces:**
- Consumes: `State`, `BuildRecord`, `KrokiStatus` from Task 7
- Produces:
  - `type statusView struct { State; DocumentPath string; Watching int; LastBuildText string }`
  - `func newStatusView(s State, documentPath string) statusView`
  - `func renderStatus(w io.Writer, v statusView) error`
  - `func renderDashboard(w io.Writer, v statusView) error`
  - `var assetsFS embed.FS` serving `assets/`

- [ ] **Step 1: Vendor htmx**

```bash
mkdir -p pkg/daemon/assets pkg/daemon/templates
curl -fsSL https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js -o pkg/daemon/assets/htmx.min.js
test -s pkg/daemon/assets/htmx.min.js && head -c 60 pkg/daemon/assets/htmx.min.js
```

Expected: a non-empty minified JavaScript file. If the machine is offline, stop
and ask how to obtain htmx rather than inventing a substitute.

- [ ] **Step 2: Write the failing tests**

Create `pkg/daemon/views_test.go`:

```go
package daemon

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func sampleState() State {
	started := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	return State{
		Mode:      ModeWatching,
		Backend:   "fsnotify",
		LastBuild: started,
		Watched:   []string{"a.graphqls", "b.graphqls"},
		History: []BuildRecord{
			{Started: started, Duration: 12 * time.Millisecond, Bytes: 4096},
		},
		Kroki: KrokiStatus{Enabled: true, Reachable: true, URL: "https://kroki.io", LastCheck: started},
	}
}

func TestRenderStatusShowsTheEssentials(t *testing.T) {
	var out bytes.Buffer
	if err := renderStatus(&out, newStatusView(sampleState(), "/docs.adoc")); err != nil {
		t.Fatalf("renderStatus: %v", err)
	}

	got := out.String()
	for _, want := range []string{"watching", "fsnotify", "2", "4096", "kroki.io"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected the fragment to mention %q, got:\n%s", want, got)
		}
	}
}

func TestRenderStatusShowsTheBuildError(t *testing.T) {
	state := sampleState()
	state.LastError = "failed to parse GraphQL schema: syntax error"
	state.History[0].Err = state.LastError

	var out bytes.Buffer
	if err := renderStatus(&out, newStatusView(state, "/docs.adoc")); err != nil {
		t.Fatalf("renderStatus: %v", err)
	}

	if !strings.Contains(out.String(), "syntax error") {
		t.Errorf("expected the error text in the fragment, got:\n%s", out.String())
	}
}

func TestRenderStatusEscapesTheErrorText(t *testing.T) {
	state := sampleState()
	state.LastError = `<script>alert("x")</script>`

	var out bytes.Buffer
	if err := renderStatus(&out, newStatusView(state, "/docs.adoc")); err != nil {
		t.Fatalf("renderStatus: %v", err)
	}

	if strings.Contains(out.String(), "<script>") {
		t.Error("expected the error text to be HTML-escaped")
	}
}

func TestRenderDashboardIncludesThePollingTrigger(t *testing.T) {
	var out bytes.Buffer
	if err := renderDashboard(&out, newStatusView(sampleState(), "/docs.adoc")); err != nil {
		t.Fatalf("renderDashboard: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, `hx-get="/fragments/status"`) {
		t.Error("expected the status fragment to be polled")
	}
	if !strings.Contains(got, `every 2s`) {
		t.Error("expected a two second polling trigger")
	}
	if !strings.Contains(got, "/static/htmx.min.js") {
		t.Error("expected the vendored htmx script tag")
	}
	if !strings.Contains(got, "/docs.adoc") {
		t.Error("expected a link to the generated document")
	}
}

func TestStatusViewCountsWatchedFiles(t *testing.T) {
	v := newStatusView(sampleState(), "/docs.adoc")
	if v.Watching != 2 {
		t.Fatalf("expected two watched files, got %d", v.Watching)
	}
}

func TestStatusViewHandlesAnAbsentBuild(t *testing.T) {
	v := newStatusView(State{Mode: ModeWatching, Backend: "poll"}, "/docs.adoc")
	if v.LastBuildText != "never" {
		t.Fatalf(`expected "never" before the first build, got %q`, v.LastBuildText)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./pkg/daemon/ -run 'Render|StatusView' -v`
Expected: FAIL — `renderStatus`, `renderDashboard` and `newStatusView` are
undefined.

- [ ] **Step 4: Write the status fragment template**

Create `pkg/daemon/templates/status.html`:

```html
{{define "status"}}
<div class="status-grid">
  <div class="tile">
    <span class="label">State</span>
    <span class="value mode-{{.Mode}}">{{.Mode}}</span>
  </div>
  <div class="tile">
    <span class="label">Watch backend</span>
    <span class="value">{{.Backend}}</span>
  </div>
  <div class="tile">
    <span class="label">Files watched</span>
    <span class="value">{{.Watching}}</span>
  </div>
  <div class="tile">
    <span class="label">Last build</span>
    <span class="value">{{.LastBuildText}}</span>
  </div>
  {{if .Kroki.Enabled}}
  <div class="tile">
    <span class="label">Kroki</span>
    <span class="value {{if .Kroki.Reachable}}ok{{else}}bad{{end}}">
      {{if .Kroki.Reachable}}reachable{{else}}unreachable{{end}} &middot; {{.Kroki.URL}}
    </span>
  </div>
  {{end}}
</div>

{{if .LastError}}
<div class="error">
  <h2>Last build failed</h2>
  <pre>{{.LastError}}</pre>
</div>
{{end}}

<h2>Recent builds</h2>
{{if .History}}
<table class="history">
  <thead>
    <tr><th>Started</th><th>Duration</th><th>Size</th><th>Result</th></tr>
  </thead>
  <tbody>
    {{range .History}}
    <tr class="{{if .Succeeded}}ok{{else}}bad{{end}}">
      <td>{{.Started.Format "15:04:05"}}</td>
      <td>{{.Duration}}</td>
      <td>{{if .Succeeded}}{{.Bytes}} bytes{{else}}&mdash;{{end}}</td>
      <td>{{if .Succeeded}}built{{else}}failed{{end}}</td>
    </tr>
    {{end}}
  </tbody>
</table>
{{else}}
<p class="muted">No builds yet.</p>
{{end}}

<h2>Watched files</h2>
{{if .Watched}}
<ul class="files">
  {{range .Watched}}<li>{{.}}</li>{{end}}
</ul>
{{else}}
<p class="muted">No files matched yet.</p>
{{end}}
{{end}}
```

- [ ] **Step 5: Write the dashboard shell template**

Create `pkg/daemon/templates/dashboard.html`:

```html
{{define "dashboard"}}<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>graphqls-to-asciidoc daemon</title>
  <link rel="stylesheet" href="/static/dashboard.css">
  <script src="/static/htmx.min.js" defer></script>
</head>
<body>
  <header>
    <h1>graphqls-to-asciidoc</h1>
    <nav>
      <a href="{{.DocumentPath}}">{{.DocumentPath}}</a>
      <button hx-post="/rebuild" hx-target="#status" hx-swap="innerHTML">Rebuild now</button>
    </nav>
  </header>

  <main id="status" hx-get="/fragments/status" hx-trigger="every 2s" hx-swap="innerHTML">
    {{template "status" .}}
  </main>
</body>
</html>
{{end}}
```

- [ ] **Step 6: Write the stylesheet**

Create `pkg/daemon/assets/dashboard.css`:

```css
:root {
  color-scheme: light dark;
  --bg: #ffffff;
  --fg: #1b1b1b;
  --muted: #6a6a6a;
  --line: #d8d8d8;
  --ok: #1a7f37;
  --bad: #c0392b;
  --panel: #f6f6f6;
}

@media (prefers-color-scheme: dark) {
  :root {
    --bg: #16181d;
    --fg: #e6e6e6;
    --muted: #9a9a9a;
    --line: #333842;
    --ok: #4ac26b;
    --bad: #ff7b72;
    --panel: #1e2128;
  }
}

* { box-sizing: border-box; }

body {
  margin: 0;
  padding: 0 1.5rem 3rem;
  background: var(--bg);
  color: var(--fg);
  font: 15px/1.5 ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
}

header {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: center;
  justify-content: space-between;
  padding: 1.25rem 0;
  border-bottom: 1px solid var(--line);
}

header h1 { margin: 0; font-size: 1.1rem; font-weight: 600; }
nav { display: flex; gap: 1rem; align-items: center; }
nav a { color: inherit; }

button {
  font: inherit;
  padding: 0.4rem 0.9rem;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--panel);
  color: inherit;
  cursor: pointer;
}

button:hover { border-color: var(--fg); }

.status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 0.75rem;
  margin: 1.5rem 0;
}

.tile {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.85rem 1rem;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--panel);
}

.label { font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--muted); }
.value { font-size: 1.05rem; font-weight: 600; overflow-wrap: anywhere; }
.value.ok, tr.ok td:last-child { color: var(--ok); }
.value.bad, tr.bad td:last-child { color: var(--bad); }
.mode-building { color: var(--ok); }

h2 { font-size: 0.95rem; margin: 1.75rem 0 0.6rem; }

.error pre {
  margin: 0;
  padding: 0.9rem 1rem;
  border: 1px solid var(--bad);
  border-radius: 8px;
  background: var(--panel);
  color: var(--bad);
  overflow-x: auto;
  white-space: pre-wrap;
}

table.history { border-collapse: collapse; width: 100%; }
table.history th, table.history td {
  text-align: left;
  padding: 0.4rem 0.6rem;
  border-bottom: 1px solid var(--line);
  font-variant-numeric: tabular-nums;
}
table.history th { font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--muted); }

ul.files { margin: 0; padding-left: 1.2rem; }
ul.files li { overflow-wrap: anywhere; }
.muted { color: var(--muted); }
```

- [ ] **Step 7: Write the view code**

Create `pkg/daemon/views.go`:

```go
package daemon

import (
	"embed"
	"html/template"
	"io"
)

//go:embed templates/*.html
var templateFS embed.FS

// assetsFS holds the vendored htmx build and the stylesheet, so the daemon
// needs no network access to serve its dashboard.
//
//go:embed assets
var assetsFS embed.FS

var dashboardTemplates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

// statusView is the state plus the few presentation values the templates need.
type statusView struct {
	State
	DocumentPath  string
	Watching      int
	LastBuildText string
}

// newStatusView adapts the coordinator's state for rendering.
func newStatusView(s State, documentPath string) statusView {
	text := "never"
	if !s.LastBuild.IsZero() {
		text = s.LastBuild.Format("15:04:05")
	}

	return statusView{
		State:         s,
		DocumentPath:  documentPath,
		Watching:      len(s.Watched),
		LastBuildText: text,
	}
}

// renderStatus writes the polled fragment.
func renderStatus(w io.Writer, v statusView) error {
	return dashboardTemplates.ExecuteTemplate(w, "status", v)
}

// renderDashboard writes the full page.
func renderDashboard(w io.Writer, v statusView) error {
	return dashboardTemplates.ExecuteTemplate(w, "dashboard", v)
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test ./pkg/daemon/ -run 'Render|StatusView' -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add pkg/daemon
git commit -m "Add the daemon dashboard templates and vendored htmx (#44)"
```

---

### Task 10: HTTP server

**Files:**
- Create: `pkg/daemon/server.go`
- Create: `pkg/daemon/server_test.go`

**Interfaces:**
- Consumes: `Coordinator` (Task 7), `NewKrokiProxy` (Task 8), `renderStatus`/`renderDashboard`/`assetsFS` (Task 9)
- Produces:
  - `func NewServer(cfg *config.Config, c *Coordinator) (*Server, error)`
  - `func (s *Server) Handler() http.Handler`
  - `func (s *Server) DocumentPath() string`
  - `func (s *Server) ListenAndServe(ctx context.Context) error`

- [ ] **Step 1: Write the failing tests**

Create `pkg/daemon/server_test.go`:

```go
package daemon

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T, schema string) (*Server, *Coordinator) {
	t.Helper()

	cfg := daemonTestConfig(t, schema)
	c := NewCoordinator(cfg, SystemClock(), "poll")
	s, err := NewServer(cfg, c)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return s, c
}

func TestDocumentPathUsesTheOutputBaseName(t *testing.T) {
	s, _ := newTestServer(t, "type Query { a: String }")
	if s.DocumentPath() != "/out.adoc" {
		t.Fatalf("expected /out.adoc, got %q", s.DocumentPath())
	}
}

func TestDashboardRoute(t *testing.T) {
	s, _ := newTestServer(t, "type Query { a: String }")

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `hx-trigger="every 2s"`) {
		t.Error("expected the dashboard to poll the status fragment")
	}
}

func TestStatusFragmentRoute(t *testing.T) {
	s, c := newTestServer(t, "type Query { a: String }")
	c.Rebuild()

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/fragments/status", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<html") {
		t.Error("expected a fragment, not a full page")
	}
	if !strings.Contains(body, "Recent builds") {
		t.Errorf("expected the history section, got:\n%s", body)
	}
}

func TestRebuildRoute(t *testing.T) {
	s, c := newTestServer(t, "type Query { a: String }")

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/rebuild", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(c.Snapshot().History) != 1 {
		t.Fatal("expected the request to trigger a build")
	}
	if !strings.Contains(rec.Body.String(), "Recent builds") {
		t.Error("expected the status fragment in the response")
	}
}

func TestRebuildRouteRejectsGet(t *testing.T) {
	s, _ := newTestServer(t, "type Query { a: String }")

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/rebuild", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestDocumentRouteServesTheOutput(t *testing.T) {
	s, c := newTestServer(t, "type Query { a: String }")
	c.Rebuild()

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/out.adoc", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("expected a text/plain content type, got %q", ct)
	}
	if !strings.HasPrefix(rec.Body.String(), "= GraphQL Documentation") {
		t.Fatalf("expected the document, got %.40q", rec.Body.String())
	}
}

func TestDocumentRouteBeforeTheFirstBuild(t *testing.T) {
	s, _ := newTestServer(t, "type Query { a: String }")

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/out.adoc", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 before the first build, got %d", rec.Code)
	}
}

func TestStaticAssetRoute(t *testing.T) {
	s, _ := newTestServer(t, "type Query { a: String }")

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/htmx.min.js", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for the vendored htmx, got %d", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("expected a non-empty asset")
	}
}

func TestKrokiProxyRoute(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mermaid/svg/abc" {
			t.Errorf("upstream saw %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, "<svg/>")
	}))
	defer upstream.Close()

	cfg := daemonTestConfig(t, "type Query { a: String }")
	cfg.KrokiURL = upstream.URL
	c := NewCoordinator(cfg, SystemClock(), "poll")
	s, err := NewServer(cfg, c)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/kroki/mermaid/svg/abc", nil))

	if !strings.Contains(rec.Body.String(), "<svg/>") {
		t.Fatalf("expected the proxied body, got %q", rec.Body.String())
	}
}

func TestKrokiRouteAbsentWhenNotConfigured(t *testing.T) {
	s, _ := newTestServer(t, "type Query { a: String }")

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/kroki/mermaid/svg/abc", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 with no Kroki server configured, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/daemon/ -run 'Server|Route|DocumentPath|Static|Dashboard|StatusFragment' -v`
Expected: FAIL — `NewServer` is undefined.

- [ ] **Step 3: Write the server**

Create `pkg/daemon/server.go`:

```go
package daemon

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

// Server exposes the dashboard, the generated document and the Kroki proxy.
type Server struct {
	cfg     *config.Config
	coord   *Coordinator
	mux     *http.ServeMux
	docPath string
}

// NewServer builds the HTTP surface for the daemon.
func NewServer(cfg *config.Config, c *Coordinator) (*Server, error) {
	s := &Server{
		cfg:     cfg,
		coord:   c,
		mux:     http.NewServeMux(),
		docPath: "/" + filepath.Base(cfg.OutputFile),
	}

	s.mux.HandleFunc("/", s.handleDashboard)
	s.mux.HandleFunc("/fragments/status", s.handleStatus)
	s.mux.HandleFunc("/rebuild", s.handleRebuild)
	s.mux.HandleFunc(s.docPath, s.handleDocument)

	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return nil, err
	}
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(assets))))

	if cfg.KrokiURL != "" {
		proxy, err := NewKrokiProxy(cfg.KrokiURL)
		if err != nil {
			return nil, err
		}
		s.mux.Handle("/kroki/", http.StripPrefix("/kroki", proxy))
	}

	return s, nil
}

// Handler returns the routed handler, exported so tests can drive it directly.
func (s *Server) Handler() http.Handler { return s.mux }

// DocumentPath is the URL the generated document is served from. It keeps the
// .adoc extension so browser extensions recognise it.
func (s *Server) DocumentPath() string { return s.docPath }

func (s *Server) view() statusView {
	return newStatusView(s.coord.Snapshot(), s.docPath)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// ServeMux routes every unmatched path to "/", so reject the rest here.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderDashboard(w, s.view()); err != nil {
		log.Printf("failed to render the dashboard: %v", err)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderStatus(w, s.view()); err != nil {
		log.Printf("failed to render the status fragment: %v", err)
	}
}

func (s *Server) handleRebuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "only POST is allowed", http.StatusMethodNotAllowed)
		return
	}

	// The build is synchronous so the response already reflects the result.
	s.coord.Rebuild()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderStatus(w, s.view()); err != nil {
		log.Printf("failed to render the status fragment: %v", err)
	}
}

func (s *Server) handleDocument(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, s.cfg.OutputFile)
}

// ListenAndServe runs the server until the context is cancelled, then shuts it
// down gracefully.
func (s *Server) ListenAndServe(ctx context.Context) error {
	server := &http.Server{
		Addr:              s.cfg.DaemonAddr,
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/daemon/ -race -v`
Expected: PASS.

`TestDocumentRouteBeforeTheFirstBuild` relies on `http.ServeFile` returning 404
for a missing file, which it does.

- [ ] **Step 5: Commit**

```bash
git add pkg/daemon
git commit -m "Add the daemon HTTP server and dashboard routes (#44)"
```

---

### Task 11: Wire the daemon together

**Files:**
- Create: `pkg/daemon/daemon.go`
- Modify: `main.go`
- Create: `pkg/daemon/daemon_test.go`

**Interfaces:**
- Consumes: everything from Tasks 5–10
- Produces: `func Run(ctx context.Context, cfg *config.Config) error`

- [ ] **Step 1: Write the failing test**

Create `pkg/daemon/daemon_test.go`:

```go
package daemon

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

// freeAddr reserves a loopback port and releases it, so the daemon can bind it.
func freeAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func TestRunBuildsOnceAndServesTheDashboard(t *testing.T) {
	cfg := daemonTestConfig(t, "type Query { a: String }")
	cfg.DaemonAddr = freeAddr(t)
	cfg.WatchMode = config.WatchModePoll
	cfg.PollInterval = 20 * time.Millisecond
	cfg.Debounce = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	// The daemon builds once at startup so there is something to serve.
	deadline := time.Now().Add(5 * time.Second)
	var resp *http.Response
	var err error
	for time.Now().Before(deadline) {
		resp, err = http.Get("http://" + cfg.DaemonAddr + "/")
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("dashboard never came up: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if !strings.Contains(string(body), "graphqls-to-asciidoc") {
		t.Fatalf("expected the dashboard, got:\n%s", body)
	}
	if _, err := os.Stat(cfg.OutputFile); err != nil {
		t.Fatalf("expected an initial build to have written the output: %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned an error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not shut down when the context was cancelled")
	}
}

func TestRunRebuildsAfterAChange(t *testing.T) {
	cfg := daemonTestConfig(t, "type Query { a: String }")
	cfg.DaemonAddr = freeAddr(t)
	cfg.WatchMode = config.WatchModePoll
	cfg.PollInterval = 20 * time.Millisecond
	cfg.Debounce = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = Run(ctx, cfg) }()

	// Wait for the initial build.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(cfg.OutputFile); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := os.WriteFile(cfg.SchemaFile, []byte("type Query { a: String bee: Int }"), 0o600); err != nil {
		t.Fatal(err)
	}

	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		out, err := os.ReadFile(cfg.OutputFile)
		if err == nil && strings.Contains(string(out), "bee") {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the change to be published")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/daemon/ -run TestRun -v`
Expected: FAIL — `Run` is undefined.

- [ ] **Step 3: Write the wiring**

Create `pkg/daemon/daemon.go`:

```go
// Package daemon watches GraphQL schema files, republishes the AsciiDoc
// document when they settle, and serves a dashboard reporting what it did.
package daemon

import (
	"context"
	"log"
	"sync"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

// Run starts the watcher, the build coordinator and the dashboard, and blocks
// until the context is cancelled.
func Run(ctx context.Context, cfg *config.Config) error {
	watcher, backend, err := NewWatcher(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = watcher.Close() }()

	clock := SystemClock()
	coord := NewCoordinator(cfg, clock, backend)

	server, err := NewServer(cfg, coord)
	if err != nil {
		return err
	}

	// Build once at startup so the dashboard has something to show and the
	// document exists before anyone opens it.
	coord.Rebuild()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		coord.Run(ctx, watcher)
	}()

	if cfg.KrokiURL != "" {
		prober, proberErr := NewKrokiProber(cfg.KrokiURL)
		if proberErr != nil {
			return proberErr
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			prober.Run(ctx, clock, coord)
		}()
	}

	log.Printf("watching with the %s backend, dashboard on http://%s%s",
		backend, cfg.DaemonAddr, server.DocumentPath())

	serveErr := server.ListenAndServe(ctx)

	// Closing the watcher ends the coordinator loop even if the server stopped
	// for its own reasons rather than a cancelled context.
	_ = watcher.Close()
	wg.Wait()

	return serveErr
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/daemon/ -race -run TestRun -v`
Expected: PASS.

- [ ] **Step 5: Add the daemon branch to `main.go`**

In `main.go`, add `"context"` and the daemon import, then insert immediately
after the `cfg.Validate()` block:

```go
	if cfg.Daemon {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		if err := daemon.Run(ctx, cfg); err != nil {
			log.Fatalf("daemon failed: %v", err)
		}
		return
	}
```

Add `"os/signal"` and `"syscall"` to the imports alongside
`"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/daemon"`.

- [ ] **Step 6: Verify by hand**

```bash
go build -o bin/graphqls-to-asciidoc .
./bin/graphqls-to-asciidoc -s test/schema.graphql -o /tmp/daemon-demo.adoc --daemon --debounce 2s &
sleep 3
curl -s http://127.0.0.1:8088/ | head -20
curl -s http://127.0.0.1:8088/fragments/status | head -20
curl -s -X POST http://127.0.0.1:8088/rebuild | head -5
curl -s -o /dev/null -w '%{http_code} %{content_type}\n' http://127.0.0.1:8088/daemon-demo.adoc
kill %1
```

Expected: the dashboard HTML, a status fragment, a fragment from the rebuild
POST, and `200 text/plain; charset=utf-8` for the document. The daemon should
exit cleanly on the `kill`.

- [ ] **Step 7: Verify the daemon survives a broken schema**

```bash
cp test/schema.graphql /tmp/watch.graphqls
./bin/graphqls-to-asciidoc -s /tmp/watch.graphqls -o /tmp/daemon-demo.adoc --daemon --debounce 1s &
sleep 2
echo 'type Broken { x: }' >> /tmp/watch.graphqls
sleep 3
curl -s http://127.0.0.1:8088/fragments/status | grep -i 'failed' && echo "ERROR REPORTED, DAEMON ALIVE"
kill %1
```

Expected: `ERROR REPORTED, DAEMON ALIVE` — the daemon must still be answering.

- [ ] **Step 8: Run the full suite and commit**

Run: `make test`
Expected: PASS.

```bash
git add pkg/daemon main.go
git commit -m "Wire up daemon mode behind the --daemon flag (#44)"
```

---

### Task 12: Documentation

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: the flags from Task 2
- Produces: nothing consumed by later tasks

- [ ] **Step 1: Add the daemon section**

Add a `## Daemon mode` section to `README.md`, placed after the existing usage
examples and before the section describing the output format:

````markdown
## Daemon mode

`--daemon` watches the schema files, republishes the document when editing
settles, and serves a small dashboard.

```bash
graphqls-to-asciidoc -s schema.graphqls -o docs.adoc --daemon
```

Open http://127.0.0.1:8088 for the dashboard, or
http://127.0.0.1:8088/docs.adoc for the generated document. The document keeps
its `.adoc` extension so a browser preview extension recognises it.

### When a rebuild happens

Two conditions must both hold before the daemon republishes:

- the debounce interval has elapsed since the last change, and
- the same interval has elapsed since the last build.

Saving repeatedly keeps resetting the first condition, so nothing is published
while you are still typing. The **Rebuild now** button on the dashboard ignores
both conditions.

A build that fails leaves the previous document in place and shows the parse
error on the dashboard. The daemon keeps running.

### Daemon flags

| Flag | Default | Purpose |
|---|---|---|
| `--daemon` | `false` | Watch and serve. Requires `-o`/`--output`. |
| `--daemon-addr` | `127.0.0.1:8088` | Dashboard listen address. |
| `--debounce` | `5s` | Quiet period before a rebuild. |
| `--watch-mode` | `auto` | `auto`, `fsnotify` or `poll`. |
| `--poll-interval` | `1s` | Scan interval for the polling backend. |

`auto` uses filesystem notifications and falls back to polling if they are
unavailable, which is the usual outcome on some network and container mounts.
The backend in use is logged at startup and shown on the dashboard.

### Diagrams with Kroki

`--kroki-url` writes `:kroki-server-url:` and `:kroki-fetch-diagram:` into the
generated document, so an Asciidoctor toolchain with the Kroki extension can
render diagrams:

```bash
graphqls-to-asciidoc -s schema.graphqls -o docs.adoc --daemon --kroki-url https://kroki.io
```

In daemon mode the attribute points at the daemon's own `/kroki` path, which
proxies to the configured server so the browser makes same-origin requests. The
dashboard shows whether the server is reachable. The tool does not generate
diagrams itself; it only tells the renderer where Kroki lives.
````

- [ ] **Step 2: Add the flags to the README's flag list**

Find the existing options table or list in `README.md` and add the six new
flags (`--daemon`, `--daemon-addr`, `--debounce`, `--watch-mode`,
`--poll-interval`, `--kroki-url`) in the same format as the entries around them.

- [ ] **Step 3: Verify the documented behaviour**

Run: `./bin/graphqls-to-asciidoc --help | grep -A 8 'DAEMON MODE'`
Expected: the flag list matches the README table.

- [ ] **Step 4: Run the full suite and commit**

Run: `make test`
Expected: PASS.

```bash
git add README.md
git commit -m "Document daemon mode and the Kroki integration (#44)"
```

---

## Final verification

- [ ] `make test` passes.
- [ ] `go test ./... -race` passes.
- [ ] `golangci-lint run` is clean, or reports only pre-existing findings.
- [ ] `go build ./...` succeeds.
- [ ] The one-shot CLI output is unchanged (Task 1, Step 8).
- [ ] The daemon serves the dashboard, the status fragment, the document and a
      Kroki proxy, and survives a malformed schema (Task 11, Steps 6–7).

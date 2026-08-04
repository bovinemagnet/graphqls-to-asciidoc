# Daemon Mode with HTMX Dashboard — Design

- **Issue:** [#44](https://github.com/bovinemagnet/graphqls-to-asciidoc/issues/44)
- **Date:** 2026-08-04
- **Author:** Paul Snow
- **Status:** Approved

## Summary

Add a `--daemon` mode to `graphqls-to-asciidoc`. The daemon watches the schema
files named by `-schema` or `-pattern`, regenerates the AsciiDoc output after a
configurable quiet period, and serves a small HTMX dashboard reporting build
status, errors and history. The generated `.adoc` is served over HTTP so a
browser extension can render it. A Kroki server URL can be configured; the
daemon proxies to it and advertises it to the renderer through AsciiDoc
attributes.

The existing one-shot CLI behaviour is unchanged.

## Scope

**In scope**

- Watch, debounce and rebuild loop.
- HTMX dashboard: status, build history, error text, manual rebuild.
- Serving the generated `.adoc` over HTTP.
- Kroki wiring: attribute emission, reverse proxy, reachability indicator.

**Out of scope** (deferred to their own specs)

- Phase 2 of the issue: embedding asciidoctor.js to render HTML in the daemon.
- Server-Sent Events for dashboard updates.
- Generating diagrams in Go (posting diagram source to Kroki and writing SVGs).

## Motivation

Editing a GraphQL schema currently means re-running the CLI by hand after every
save. Daemon mode closes that loop: save the schema, and a second or two later
the browser tab holding the generated document is up to date. The dashboard
surfaces parse and generation failures directly, which is where most of the
friction sits while a schema is being written.

## Architecture

### Package layout

`main.go` currently calls `log.Fatalf` at every failure point in the
read → clean → parse → generate pipeline. A daemon must survive a bad schema
and report it rather than exit, so that pipeline is extracted first.

- **`pkg/build`** — `build.Run(cfg *config.Config) (*Result, error)`. Reads or
  globs the schema files, strips fragments, parses, and generates into a
  `bytes.Buffer`. Returns errors instead of exiting the process. `main.go`
  becomes a thin one-shot caller; the daemon calls the same function for every
  rebuild. No behavioural change to the CLI.
- **`pkg/daemon`** — the `Watcher` interface with `fsnotify` and polling
  backends, the debouncer, the build coordinator, and a bounded build-history
  ring buffer.
- **`pkg/daemon/web`** — `http.ServeMux`, the `html/template` dashboard,
  static assets embedded with `go:embed`, and the Kroki reverse proxy.
- **`pkg/config`** — new flags and their validation.

htmx is vendored into the repository and embedded with `go:embed`, not loaded
from a CDN. The binary stays self-contained and the dashboard works offline.

### Components and their boundaries

| Unit | Does | Depends on |
|---|---|---|
| `build.Run` | schema bytes → AsciiDoc bytes, or an error | `config`, `parser`, `generator` |
| `Watcher` | emits "something changed" events | filesystem |
| `Debouncer` | decides *when* a change becomes a rebuild | a `Clock` |
| `Coordinator` | owns state, runs builds, writes output | `build.Run`, `Watcher`, `Debouncer` |
| `web.Server` | renders state as HTML, accepts rebuild requests | `Coordinator` (read + trigger only) |

The coordinator exposes a read-only `Snapshot()` and a `TriggerRebuild()` to
the web layer. The web layer never touches the watcher or the filesystem.

## Flags

| Flag | Default | Purpose |
|---|---|---|
| `--daemon` | `false` | Enable watch mode and the dashboard |
| `--daemon-addr` | `127.0.0.1:8088` | Dashboard listen address (loopback by default) |
| `--debounce` | `5s` | Quiet period before a rebuild |
| `--watch-mode` | `auto` | `auto` \| `fsnotify` \| `poll` |
| `--poll-interval` | `1s` | Filesystem scan interval, polling backend only |
| `--kroki-url` | *(empty)* | Kroki server, e.g. `https://kroki.io` |

### Validation

- `--daemon` requires `-o`/`--output`. There is nothing to serve otherwise, and
  writing the document to stdout on every rebuild is meaningless.
- The daemon-only flags (`--daemon-addr`, `--debounce`, `--watch-mode`,
  `--poll-interval`) are an error when given without `--daemon`. A flag that
  silently does nothing is worse than a rejected command line.
- `--kroki-url` is accepted in both modes; it is the one daemon-adjacent flag
  that also affects one-shot output.
- `--debounce` and `--poll-interval` must be greater than zero.
- `--watch-mode` must be one of the three listed values.

### Watch mode selection

`auto` attempts fsnotify and falls back to polling if the watcher cannot be
initialised. The chosen backend is logged once at startup, so the active mode
is never ambiguous. `fsnotify` and `poll` force a backend; forcing `fsnotify`
on a system where it fails to initialise is a fatal startup error rather than a
silent fallback.

Both backends emit the same event type on the same channel. The debounce and
rebuild logic is shared and has no knowledge of which backend produced an
event.

**fsnotify backend.** Watches the *directories* containing the matched schema
files, not the files themselves, so that newly created files matching the
pattern are picked up and editors that save by rename-over are handled. Events
for paths that do not match the configured pattern are discarded. The set of
watched directories is recomputed after each rebuild.

**Polling backend.** Re-globs the pattern every `--poll-interval` and compares
each file's modification time and size against the previous scan. Added and
removed files count as changes.

## Debounce semantics

Taken directly from the issue. Both conditions must hold before a rebuild
starts:

```
rebuild when  (now - lastChange) >= debounce
        and   (now - lastBuild)  >= debounce
```

A save resets `lastChange`, so continuous editing never triggers a build. The
second condition rate-limits rebuilds so a burst of saves separated by more
than the debounce interval cannot produce back-to-back builds.

`POST /rebuild` bypasses both conditions and builds immediately.

Changes that arrive while a build is running set a dirty flag; the coordinator
re-evaluates the conditions once the build finishes rather than discarding the
event.

## Build coordination and output

A single goroutine owns the daemon state, guarded by a mutex:

```go
type State struct {
    Mode        string      // watching | building
    Backend     string      // fsnotify | poll
    LastBuild   time.Time
    LastError   string      // empty when the last build succeeded
    Watched     []string    // schema files currently matched
    History     []BuildRecord // bounded ring buffer, newest first
    Kroki       KrokiStatus
}

type BuildRecord struct {
    Started  time.Time
    Duration time.Duration
    Bytes    int
    Err      string
}
```

History is capped at 20 records.

Output is written to a temporary file in the destination directory and then
`os.Rename`d over the target, so a browser polling the document never reads a
half-written file. A failed build leaves the previous good output untouched and
records the error in the state.

## HTTP surface

| Route | Purpose |
|---|---|
| `GET /` | Dashboard shell |
| `GET /fragments/status` | HTMX partial, polled by `hx-trigger="every 2s"` |
| `POST /rebuild` | Force a rebuild; responds with the same partial |
| `GET /<basename>.adoc` | The generated document, `text/plain; charset=utf-8` |
| `GET /static/*` | Embedded htmx and stylesheet |
| `/kroki/*` | Reverse proxy to `--kroki-url` |

`<basename>` is the base name of the `-o` path, so the served URL keeps the
`.adoc` extension that browser extensions key off. The document is read from
disk on each request.

The status fragment is the single source of truth for the dashboard's dynamic
content: state, backend in use, last build time and duration, number of files
watched, the error text of a failed build, the build history table, and the
Kroki indicator. Keeping it a single fragment means SSE can replace the polling
trigger later without changing the markup.

The listener binds to loopback by default. Binding to a non-loopback address is
possible via `--daemon-addr` but is the operator's explicit choice; there is no
authentication, and this is a local development tool.

## Kroki

When `--kroki-url` is set, the generated document header gains:

```
:kroki-server-url: <url>
:kroki-fetch-diagram:
```

In daemon mode the emitted URL points at the daemon's own `/kroki` path, so the
browser-side renderer makes same-origin requests and sidesteps CORS. In
one-shot CLI mode the raw `--kroki-url` value is emitted.

The daemon probes the Kroki server every 30 seconds and caches the result; the
dashboard shows reachable or unreachable with the time of the last check. A
failing probe never fails a build.

No diagram generation happens in Go. The daemon does not inspect, extract or
post diagram source.

## Error handling

| Failure | Behaviour |
|---|---|
| Schema file unreadable | Build fails, error recorded and shown, previous output kept |
| GraphQL parse error | Same; the parser's message is shown verbatim in the dashboard |
| Output write fails | Same; the rename is not performed |
| fsnotify init fails under `auto` | Log once, fall back to polling |
| fsnotify init fails under `--watch-mode=fsnotify` | Fatal at startup |
| Kroki unreachable | Indicator shows unreachable; builds unaffected |
| Port already in use | Fatal at startup with the address in the message |

One-shot CLI failures continue to exit non-zero as they do today.

## Lifecycle

`SIGINT` and `SIGTERM` cancel the root context, which triggers a graceful
`http.Server` shutdown and closes the watcher. An in-flight build is allowed to
finish so the output file is never left mid-rename.

## Testing

Test-driven, following the project's existing per-package `*_test.go`
convention.

- **Debouncer** — table-driven against an injected `Clock`: a single change,
  a burst within the window, a change during a build, and the
  `lastBuild` rate-limit condition.
- **Polling watcher** — temporary directory; assert events for modified,
  created and deleted files, and no event for an untouched tree.
- **fsnotify watcher** — temporary directory integration test, skipped when the
  backend cannot initialise.
- **`build.Run`** — the existing `test/schema.graphql` produces non-empty
  output; a deliberately malformed schema returns an error rather than exiting
  the process. This is the regression guard for the `log.Fatalf` extraction.
- **Handlers** — `httptest`: the status fragment reflects a seeded state,
  `POST /rebuild` triggers the coordinator, the document route serves the file
  with the right content type, and the Kroki proxy forwards to a stub backend.
- **Config** — validation table covering each new flag rule.

## Documentation

The README gains a daemon mode section covering the new flags, the debounce
rule, and the browser-extension workflow. This repository has no Antora
`src/docs` tree, so no Antora page is added.

## Dependencies

One new direct dependency: `github.com/fsnotify/fsnotify`. htmx is vendored as
a static asset rather than added as a module dependency.

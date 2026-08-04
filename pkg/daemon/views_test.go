package daemon

import (
	"bytes"
	"io/fs"
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
	if v.LastBuildText != noBuildYetText {
		t.Fatalf(`expected %q before the first build, got %q`, noBuildYetText, v.LastBuildText)
	}
}

// TestStatusViewCarriesTheDocumentPath checks that newStatusView passes an
// arbitrary document path straight through, rather than assuming a fixed one.
func TestStatusViewCarriesTheDocumentPath(t *testing.T) {
	v := newStatusView(sampleState(), "/out/api.adoc")
	if v.DocumentPath != "/out/api.adoc" {
		t.Fatalf("expected DocumentPath to be carried through, got %q", v.DocumentPath)
	}
}

// TestAssetsFSServesTheVendoredFiles checks that assetsFS is rooted so that
// fs.Sub(assetsFS, "assets") — as the HTTP server will call it — exposes the
// stylesheet and vendored htmx script directly, rather than nested under an
// extra "assets" segment.
func TestAssetsFSServesTheVendoredFiles(t *testing.T) {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}

	for _, name := range []string{"htmx.min.js", "dashboard.css"} {
		if _, err := fs.Stat(sub, name); err != nil {
			t.Errorf("expected %s to be served from assetsFS: %v", name, err)
		}
	}
}

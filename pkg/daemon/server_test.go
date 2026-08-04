package daemon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) (*Server, *Coordinator) {
	t.Helper()

	cfg := daemonTestConfig(t, "type Query { a: String }")
	c := NewCoordinator(cfg, SystemClock(), "poll")
	s, err := NewServer(cfg, c)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return s, c
}

func newTestRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(context.Background(), method, target, http.NoBody)
}

func TestDocumentPathUsesTheOutputBaseName(t *testing.T) {
	s, _ := newTestServer(t)
	if s.DocumentPath() != "/out.adoc" {
		t.Fatalf("expected /out.adoc, got %q", s.DocumentPath())
	}
}

func TestDashboardRoute(t *testing.T) {
	s, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, newTestRequest(t, http.MethodGet, "/"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `hx-trigger="every 2s"`) {
		t.Error("expected the dashboard to poll the status fragment")
	}
}

func TestStatusFragmentRoute(t *testing.T) {
	s, c := newTestServer(t)
	c.Rebuild()

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, newTestRequest(t, http.MethodGet, "/fragments/status"))

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
	s, c := newTestServer(t)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, newTestRequest(t, http.MethodPost, "/rebuild"))

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
	s, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, newTestRequest(t, http.MethodGet, "/rebuild"))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestDocumentRouteServesTheOutput(t *testing.T) {
	s, c := newTestServer(t)
	c.Rebuild()

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, newTestRequest(t, http.MethodGet, "/out.adoc"))

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
	s, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, newTestRequest(t, http.MethodGet, "/out.adoc"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 before the first build, got %d", rec.Code)
	}
}

func TestStaticAssetRoute(t *testing.T) {
	s, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, newTestRequest(t, http.MethodGet, "/static/htmx.min.js"))

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
	s.Handler().ServeHTTP(rec, newTestRequest(t, http.MethodGet, "/kroki/mermaid/svg/abc"))

	if !strings.Contains(rec.Body.String(), "<svg/>") {
		t.Fatalf("expected the proxied body, got %q", rec.Body.String())
	}
}

func TestKrokiRouteAbsentWhenNotConfigured(t *testing.T) {
	s, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, newTestRequest(t, http.MethodGet, "/kroki/mermaid/svg/abc"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 with no Kroki server configured, got %d", rec.Code)
	}
}

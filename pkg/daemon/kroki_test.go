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
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/kroki/mermaid/svg/abc123", http.NoBody)
	mux.ServeHTTP(rec, req)

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

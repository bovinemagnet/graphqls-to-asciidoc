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

	var lc net.ListenConfig
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
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
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, "http://"+cfg.DaemonAddr+"/", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err = http.DefaultClient.Do(req)
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

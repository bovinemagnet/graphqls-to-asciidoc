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

// krokiProbeTimeout bounds how long a single reachability check may take.
const krokiProbeTimeout = 5 * time.Second

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
	return &KrokiProber{url: rawURL, client: &http.Client{Timeout: krokiProbeTimeout}}, nil
}

// Probe performs one reachability check.
func (p *KrokiProber) Probe(ctx context.Context, now time.Time) KrokiStatus {
	status := KrokiStatus{Enabled: true, URL: p.url, LastCheck: now}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, http.NoBody)
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

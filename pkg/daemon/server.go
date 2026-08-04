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

// htmlContentType is the header value for the dashboard page and its
// htmx-polled fragments.
const htmlContentType = "text/html; charset=utf-8"

// shutdownTimeout bounds how long ListenAndServe waits for in-flight
// requests to finish once the context is cancelled.
const shutdownTimeout = 5 * time.Second

// readHeaderTimeout bounds how long the server waits to read request headers,
// guarding against slow-loris style connections.
const readHeaderTimeout = 10 * time.Second

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

// view builds the presentation state for the current snapshot.
func (s *Server) view() *statusView {
	snapshot := s.coord.Snapshot()
	return newStatusView(&snapshot, s.docPath)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// ServeMux routes every unmatched path to "/", so reject the rest here.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", htmlContentType)
	if err := renderDashboard(w, s.view()); err != nil {
		log.Printf("failed to render the dashboard: %v", err)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", htmlContentType)
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

	w.Header().Set("Content-Type", htmlContentType)
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
		ReadHeaderTimeout: readHeaderTimeout,
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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

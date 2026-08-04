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

// noBuildYetText is shown in place of a timestamp before the first build.
const noBuildYetText = "never"

// statusView is the state plus the few presentation values the templates need.
type statusView struct {
	State
	DocumentPath  string
	Watching      int
	LastBuildText string
}

// newStatusView adapts the coordinator's state for rendering. It returns a
// pointer so callers can chain it straight into renderStatus/renderDashboard.
func newStatusView(s *State, documentPath string) *statusView {
	text := noBuildYetText
	if !s.LastBuild.IsZero() {
		text = s.LastBuild.Format("15:04:05")
	}

	return &statusView{
		State:         *s,
		DocumentPath:  documentPath,
		Watching:      len(s.Watched),
		LastBuildText: text,
	}
}

// renderStatus writes the polled fragment.
func renderStatus(w io.Writer, v *statusView) error {
	return dashboardTemplates.ExecuteTemplate(w, "status", v)
}

// renderDashboard writes the full page.
func renderDashboard(w io.Writer, v *statusView) error {
	return dashboardTemplates.ExecuteTemplate(w, "dashboard", v)
}

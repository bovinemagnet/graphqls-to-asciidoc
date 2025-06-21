package metrics

import (
	"fmt"
	"os"
	"time"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
	"github.com/jedib0t/go-pretty/v6/table"
)

// SectionMetrics holds timing and count data for a processing section
type SectionMetrics struct {
	Name      string
	Count     int
	Duration  time.Duration
	Processed bool
}

// Metrics tracks processing statistics and timing information
type Metrics struct {
	config    *config.Config
	startTime time.Time
	sections  map[string]*SectionMetrics
	enabled   bool
}

// New creates a new Metrics instance
func New(cfg *config.Config) *Metrics {
	return &Metrics{
		config:    cfg,
		startTime: time.Now(),
		sections:  make(map[string]*SectionMetrics),
		enabled:   cfg.Verbose,
	}
}

// StartSection begins timing for a processing section
func (m *Metrics) StartSection(name string) *SectionTimer {
	if !m.enabled {
		return &SectionTimer{enabled: false}
	}

	section := &SectionMetrics{
		Name:      name,
		Count:     0,
		Processed: false,
	}
	m.sections[name] = section

	return &SectionTimer{
		metrics:   m,
		section:   section,
		startTime: time.Now(),
		enabled:   true,
	}
}

// SectionTimer handles timing for individual sections
type SectionTimer struct {
	metrics   *Metrics
	section   *SectionMetrics
	startTime time.Time
	enabled   bool
}

// AddCount increments the count for the current section
func (st *SectionTimer) AddCount(count int) {
	if !st.enabled {
		return
	}
	st.section.Count += count
}

// Finish completes timing for the section
func (st *SectionTimer) Finish() {
	if !st.enabled {
		return
	}
	st.section.Duration = time.Since(st.startTime)
	st.section.Processed = true
}

// LogInputParameters logs the input configuration parameters
func (m *Metrics) LogInputParameters() {
	if !m.enabled {
		return
	}

	// Create input parameters table
	t := table.NewWriter()
	t.SetOutputMirror(os.Stderr)
	t.SetStyle(table.StyleRounded)
	t.SetTitle("GraphQLS-to-AsciiDoc - Processing Started")
	t.AppendHeader(table.Row{"Parameter", "Value"})

	// Input parameters
	t.AppendRow(table.Row{"Schema File", m.config.SchemaFile})
	if m.config.OutputFile != "" {
		t.AppendRow(table.Row{"Output File", m.config.OutputFile})
	} else {
		t.AppendRow(table.Row{"Output", "stdout"})
	}
	t.AppendRow(table.Row{"Exclude Internal", m.config.ExcludeInternal})

	// Add separator before sections
	t.AppendSeparator()

	// Sections to process
	t.AppendRow(table.Row{"Queries", formatEnabled(m.config.IncludeQueries)})
	t.AppendRow(table.Row{"Mutations", formatEnabled(m.config.IncludeMutations)})
	t.AppendRow(table.Row{"Subscriptions", formatEnabled(m.config.IncludeSubscriptions)})
	t.AppendRow(table.Row{"Types", formatEnabled(m.config.IncludeTypes)})
	t.AppendRow(table.Row{"Enums", formatEnabled(m.config.IncludeEnums)})
	t.AppendRow(table.Row{"Inputs", formatEnabled(m.config.IncludeInputs)})
	t.AppendRow(table.Row{"Directives", formatEnabled(m.config.IncludeDirectives)})
	t.AppendRow(table.Row{"Scalars", formatEnabled(m.config.IncludeScalars)})

	// Render the table
	t.Render()
	fmt.Fprintf(os.Stderr, "\n")
}

// LogMetricsTable prints a comprehensive metrics table
func (m *Metrics) LogMetricsTable() {
	if !m.enabled {
		return
	}

	totalDuration := time.Since(m.startTime)

	t := table.NewWriter()
	t.SetOutputMirror(os.Stderr)
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Section", "Count", "Duration", "Status"})

	// Define the order of sections for consistent display
	sectionOrder := []string{
		"Queries", "Mutations", "Subscriptions",
		"Types", "Enums", "Inputs", "Directives", "Scalars",
	}

	var totalProcessed int
	var totalSectionTime time.Duration

	for _, sectionName := range sectionOrder {
		if section, exists := m.sections[sectionName]; exists && section.Processed {
			status := "✓"
			if section.Count == 0 {
				status = "○" // processed but no items
			}
			t.AppendRow(table.Row{
				section.Name,
				section.Count,
				formatDuration(section.Duration),
				status,
			})
			totalProcessed += section.Count
			totalSectionTime += section.Duration
		} else {
			t.AppendRow(table.Row{
				sectionName, "-", "-", "✗",
			})
		}
	}

	t.AppendSeparator()
	t.AppendRow(table.Row{
		"TOTAL", totalProcessed, formatDuration(totalDuration), "✓",
	})

	// Print the table
	t.Render()

	// Calculate processing efficiency
	processingRatio := float64(totalSectionTime) / float64(totalDuration) * 100
	fmt.Fprintf(os.Stderr, "\nProcessing Efficiency: %.1f%% (%.2fms overhead)\n",
		processingRatio,
		float64(totalDuration-totalSectionTime)/1000000)

	fmt.Fprintf(os.Stderr, "Items per Second:      %.1f\n",
		float64(totalProcessed)/totalDuration.Seconds())

	fmt.Fprintf(os.Stderr, "\n")
}

// formatEnabled returns a colored status string
func formatEnabled(enabled bool) string {
	if enabled {
		return "✓ enabled"
	}
	return "✗ disabled"
}

// formatDuration formats duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	} else if d < time.Millisecond {
		return fmt.Sprintf("%.1fμs", float64(d.Nanoseconds())/1000)
	} else if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Nanoseconds())/1000000)
	} else {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// LogProgress logs processing progress for a section
func (m *Metrics) LogProgress(section string, message string) {
	if !m.enabled {
		return
	}

	timestamp := time.Since(m.startTime)
	fmt.Fprintf(os.Stderr, "[%8s] %s: %s\n",
		formatDuration(timestamp), section, message)
}

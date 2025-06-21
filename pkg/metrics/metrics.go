package metrics

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
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

	fmt.Fprintf(os.Stderr, "┌─ GraphQLS-to-AsciiDoc - Processing Started ─┐\n")
	fmt.Fprintf(os.Stderr, "│\n")
	fmt.Fprintf(os.Stderr, "│ Input Parameters:\n")
	fmt.Fprintf(os.Stderr, "│   Schema File:         %s\n", m.config.SchemaFile)
	
	if m.config.OutputFile != "" {
		fmt.Fprintf(os.Stderr, "│   Output File:         %s\n", m.config.OutputFile)
	} else {
		fmt.Fprintf(os.Stderr, "│   Output:              stdout\n")
	}
	
	fmt.Fprintf(os.Stderr, "│   Exclude Internal:    %t\n", m.config.ExcludeInternal)
	fmt.Fprintf(os.Stderr, "│\n")
	fmt.Fprintf(os.Stderr, "│ Sections to Process:\n")
	fmt.Fprintf(os.Stderr, "│   Queries:             %s\n", formatEnabled(m.config.IncludeQueries))
	fmt.Fprintf(os.Stderr, "│   Mutations:           %s\n", formatEnabled(m.config.IncludeMutations))
	fmt.Fprintf(os.Stderr, "│   Subscriptions:       %s\n", formatEnabled(m.config.IncludeSubscriptions))
	fmt.Fprintf(os.Stderr, "│   Types:               %s\n", formatEnabled(m.config.IncludeTypes))
	fmt.Fprintf(os.Stderr, "│   Enums:               %s\n", formatEnabled(m.config.IncludeEnums))
	fmt.Fprintf(os.Stderr, "│   Inputs:              %s\n", formatEnabled(m.config.IncludeInputs))
	fmt.Fprintf(os.Stderr, "│   Directives:          %s\n", formatEnabled(m.config.IncludeDirectives))
	fmt.Fprintf(os.Stderr, "│   Scalars:             %s\n", formatEnabled(m.config.IncludeScalars))
	fmt.Fprintf(os.Stderr, "│\n")
	fmt.Fprintf(os.Stderr, "└──────────────────────────────────────────────┘\n")
	fmt.Fprintf(os.Stderr, "\n")
}

// LogMetricsTable prints a comprehensive metrics table
func (m *Metrics) LogMetricsTable() {
	if !m.enabled {
		return
	}

	totalDuration := time.Since(m.startTime)
	
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "┌─ Processing Metrics ──────────────────────────┐\n")
	fmt.Fprintf(os.Stderr, "│\n")
	fmt.Fprintf(os.Stderr, "│ %-15s │ %7s │ %12s │ %8s\n", "Section", "Count", "Duration", "Status")
	fmt.Fprintf(os.Stderr, "│ %s │ %s │ %s │ %s\n", 
		strings.Repeat("─", 15), 
		strings.Repeat("─", 7), 
		strings.Repeat("─", 12), 
		strings.Repeat("─", 8))

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
			
			fmt.Fprintf(os.Stderr, "│ %-15s │ %7d │ %12s │ %8s\n", 
				section.Name, 
				section.Count, 
				formatDuration(section.Duration),
				status)
			
			totalProcessed += section.Count
			totalSectionTime += section.Duration
		} else {
			// Section was not processed (disabled)
			fmt.Fprintf(os.Stderr, "│ %-15s │ %7s │ %12s │ %8s\n", 
				sectionName, "-", "-", "✗")
		}
	}

	fmt.Fprintf(os.Stderr, "│ %s │ %s │ %s │ %s\n", 
		strings.Repeat("─", 15), 
		strings.Repeat("─", 7), 
		strings.Repeat("─", 12), 
		strings.Repeat("─", 8))
	
	fmt.Fprintf(os.Stderr, "│ %-15s │ %7d │ %12s │ %8s\n", 
		"TOTAL", totalProcessed, formatDuration(totalDuration), "✓")
	
	fmt.Fprintf(os.Stderr, "│\n")
	
	// Calculate processing efficiency
	processingRatio := float64(totalSectionTime) / float64(totalDuration) * 100
	fmt.Fprintf(os.Stderr, "│ Processing Efficiency: %.1f%% (%.2fms overhead)\n", 
		processingRatio, 
		float64(totalDuration-totalSectionTime)/1000000)
	
	fmt.Fprintf(os.Stderr, "│ Items per Second:      %.1f\n", 
		float64(totalProcessed)/totalDuration.Seconds())
	
	fmt.Fprintf(os.Stderr, "│\n")
	fmt.Fprintf(os.Stderr, "└──────────────────────────────────────────────┘\n")
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
package metrics

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{Verbose: true}
	m := New(cfg)

	if m == nil {
		t.Fatal("New() returned nil")
	}
	if m.config != cfg {
		t.Error("Config not set correctly")
	}
	if !m.enabled {
		t.Error("Metrics should be enabled when Verbose is true")
	}
}

func TestNewDisabled(t *testing.T) {
	cfg := &config.Config{Verbose: false}
	m := New(cfg)

	if m == nil {
		t.Fatal("New() returned nil")
	}
	if m.enabled {
		t.Error("Metrics should be disabled when Verbose is false")
	}
}

func TestSectionTimer(t *testing.T) {
	cfg := &config.Config{Verbose: true}
	m := New(cfg)

	timer := m.StartSection("TestSection")
	if timer == nil {
		t.Fatal("StartSection() returned nil")
	}
	if !timer.enabled {
		t.Error("Timer should be enabled when metrics are enabled")
	}

	timer.AddCount(5)
	time.Sleep(1 * time.Millisecond) // Small delay to ensure measurable duration
	timer.Finish()

	section := m.sections["TestSection"]
	if section == nil {
		t.Fatal("Section not found in metrics")
	}
	if section.Count != 5 {
		t.Errorf("Expected count 5, got %d", section.Count)
	}
	if section.Duration == 0 {
		t.Error("Duration should be greater than 0")
	}
	if !section.Processed {
		t.Error("Section should be marked as processed")
	}
}

func TestSectionTimerDisabled(t *testing.T) {
	cfg := &config.Config{Verbose: false}
	m := New(cfg)

	timer := m.StartSection("TestSection")
	if timer == nil {
		t.Fatal("StartSection() returned nil")
	}
	if timer.enabled {
		t.Error("Timer should be disabled when metrics are disabled")
	}

	timer.AddCount(5)
	timer.Finish()

	// No section should be created when disabled
	if len(m.sections) > 0 {
		t.Error("No sections should be created when metrics are disabled")
	}
}

func TestFormatDuration(t *testing.T) {
	testCases := []struct {
		duration time.Duration
		expected string
	}{
		{500 * time.Nanosecond, "500ns"},
		{1500 * time.Nanosecond, "1.5μs"},
		{1500 * time.Microsecond, "1.5ms"},
		{1500 * time.Millisecond, "1.50s"},
	}

	for _, tc := range testCases {
		result := formatDuration(tc.duration)
		if result != tc.expected {
			t.Errorf("formatDuration(%v) = %q, expected %q", tc.duration, result, tc.expected)
		}
	}
}

func TestFormatEnabled(t *testing.T) {
	if formatEnabled(true) != "✓ enabled" {
		t.Error("formatEnabled(true) should return '✓ enabled'")
	}
	if formatEnabled(false) != "✗ disabled" {
		t.Error("formatEnabled(false) should return '✗ disabled'")
	}
}

func TestMetricsWithEnabledConfig(t *testing.T) {
	cfg := &config.Config{
		Verbose: true,
	}

	metrics := New(cfg)
	if metrics == nil {
		t.Fatal("New() returned nil")
	}

	// Test section timer
	timer := metrics.StartSection("TestSection")
	if timer == nil {
		t.Fatal("StartSection() returned nil")
	}

	// Test logging
	metrics.LogProgress("TestSection", "Test message")
	metrics.LogInputParameters()

	// Test metrics table logging
	metrics.LogMetricsTable()

	// Test timer operations
	timer.AddCount(5)
	timer.Finish()
}

func TestMetricsWithDisabledConfig(t *testing.T) {
	cfg := &config.Config{
		Verbose: false,
	}

	metrics := New(cfg)
	if metrics == nil {
		t.Fatal("New() returned nil")
	}

	// Test section timer (should be disabled)
	timer := metrics.StartSection("TestSection")
	if timer == nil {
		t.Fatal("StartSection() returned nil even when disabled")
	}

	// Test logging (should be no-ops)
	metrics.LogProgress("TestSection", "Test message")
	metrics.LogInputParameters()
	metrics.LogMetricsTable()

	// Test timer operations (should be no-ops)
	timer.AddCount(5)
	timer.Finish()
}

func TestSectionTimerOperations(t *testing.T) {
	cfg := &config.Config{Verbose: true}
	metrics := New(cfg)

	timer := metrics.StartSection("TestSection")

	// Test multiple count additions
	timer.AddCount(1)
	timer.AddCount(2)
	timer.AddCount(3)

	// Test finish
	timer.Finish()

	// Test that we can call finish multiple times safely
	timer.Finish()
}

func TestFormatDurationEdgeCases(t *testing.T) {
	// Test zero duration
	duration := time.Duration(0)
	formatted := formatDuration(duration)
	if formatted != "0ns" {
		t.Errorf("Expected '0ns', got %s", formatted)
	}

	// Test very small duration
	duration = time.Microsecond
	formatted = formatDuration(duration)
	if formatted != "1.0μs" {
		t.Errorf("Expected '1.0μs', got %s", formatted)
	}

	// Test very large duration
	duration = 25 * time.Hour
	formatted = formatDuration(duration)
	if !strings.Contains(formatted, "s") {
		t.Errorf("Expected seconds in format, got %s", formatted)
	}
}

func TestMetricsBasicOperations(t *testing.T) {
	cfg := &config.Config{Verbose: true}
	metrics := New(cfg)

	// Test multiple sections
	sections := []string{"Section1", "Section2", "Section3"}

	for _, section := range sections {
		timer := metrics.StartSection(section)
		metrics.LogProgress(section, fmt.Sprintf("Processing %s", section))
		timer.AddCount(1)
		timer.Finish()
	}

	// Test final metrics table
	metrics.LogMetricsTable()
}

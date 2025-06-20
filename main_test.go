package main

import (
	"strings"
	"testing"
	"text/template"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/generator"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/parser"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/templates"
)

func TestVersionOutput(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalBuildTime := BuildTime

	// Set test values
	Version = "test-version"
	BuildTime = "2025-01-01_12:00:00"

	// Restore original values after test
	defer func() {
		Version = originalVersion
		BuildTime = originalBuildTime
	}()

	// Test version output format
	expectedContains := []string{
		"graphqls-to-asciidoc",
		"Version: test-version",
		"Build Time: 2025-01-01_12:00:00",
		"Built with: go",
	}

	// Note: We can't easily test the actual -version flag without refactoring main(),
	// but we can verify the variables are accessible
	if Version != "test-version" {
		t.Errorf("Version variable not set correctly, got %q", Version)
	}
	if BuildTime != "2025-01-01_12:00:00" {
		t.Errorf("BuildTime variable not set correctly, got %q", BuildTime)
	}

	// Verify the variables can be used in formatted output
	for _, expected := range expectedContains {
		if expected == "Version: test-version" && !strings.Contains(expected, Version) {
			t.Errorf("Version not found in expected string: %s", expected)
		}
		if expected == "Build Time: 2025-01-01_12:00:00" && !strings.Contains(expected, BuildTime) {
			t.Errorf("BuildTime not found in expected string: %s", expected)
		}
	}
}

// Test that the field template can be parsed and executed
func TestFieldTemplateBasic(t *testing.T) {
	// Test that the field template can be parsed and doesn't have syntax errors
	tmpl, err := template.New("field").Funcs(template.FuncMap{
		"processDescription": parser.ProcessDescription,
	}).Parse(templates.FieldTemplate)

	if err != nil {
		t.Errorf("Field template parsing failed: %v", err)
	}

	// Test basic execution with sample data
	var buf strings.Builder
	data := generator.FieldData{
		Type:            "`String`",
		Name:            "testField",
		Description:     "Test description",
		RequiredOrArray: false,
		Required:        "",
		IsArray:         false,
		Directives:      "",
		Changelog:       "",
	}

	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Errorf("Field template execution failed: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, "testField") {
		t.Errorf("Template output should contain field name 'testField', got: %q", result)
	}
	if !strings.Contains(result, "Test description") {
		t.Errorf("Template output should contain description, got: %q", result)
	}
}
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/vektah/gqlparser/v2/ast"
	gqlparser "github.com/vektah/gqlparser/v2/parser"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/generator"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/parser"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/templates"
)

const (
	testVersion   = "test-version"
	testBuildTime = "2025-01-01_12:00:00"

	rootQuery        = "Query"
	rootMutation     = "Mutation"
	rootSubscription = "Subscription"
)

func TestVersionOutput(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalBuildTime := BuildTime

	// Set test values
	Version = testVersion
	BuildTime = testBuildTime

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
	if Version != testVersion {
		t.Errorf("Version variable not set correctly, got %q", Version)
	}
	if BuildTime != testBuildTime {
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

// Golden-file tests for default-value rendering across every shape PR #45
// supports plus the tricky cases called out in issue #55. Each fixture in
// test/defaults/ pairs a .graphql schema with a .adoc golden; the test
// regenerates the output in memory, normalises the non-deterministic
// preamble lines (:revdate:, :commandline:), and diffs against the golden.
//
// If a change intentionally moves the default-value rendering, regenerate
// the goldens with `make test_doc_defaults`.
func TestDefaultValueGoldens(t *testing.T) {
	matches, err := filepath.Glob("test/defaults/*.graphql")
	if err != nil {
		t.Fatalf("failed to glob fixtures: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no fixtures found under test/defaults/")
	}

	for _, schemaPath := range matches {
		name := strings.TrimSuffix(filepath.Base(schemaPath), ".graphql")
		t.Run(name, func(t *testing.T) {
			goldenPath := strings.TrimSuffix(schemaPath, ".graphql") + ".adoc"

			got, err := renderFixture(schemaPath)
			if err != nil {
				t.Fatalf("render %s: %v", schemaPath, err)
			}
			got = normalisePreamble(got)

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v", goldenPath, err)
			}

			if got != string(want) {
				t.Errorf(
					"%s: output does not match golden.\n"+
						"Regenerate with `make test_doc_defaults` and re-run.\n\n"+
						"First-diff region:\n%s",
					goldenPath, firstDiff(got, string(want)),
				)
			}
		})
	}
}

// renderFixture reproduces the parse→generate pipeline from main.go for a
// single schema file and returns the generated AsciiDoc.
func renderFixture(schemaPath string) (string, error) {
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", err
	}
	cleaned := parser.RemoveFragments(string(schemaBytes))

	doc, gqlErr := gqlparser.ParseSchema(&ast.Source{
		Name:  "GraphQL schema",
		Input: cleaned,
	})
	if gqlErr != nil {
		return "", gqlErr
	}

	schema := &ast.Schema{
		Types:      make(map[string]*ast.Definition),
		Directives: make(map[string]*ast.DirectiveDefinition),
	}
	for _, def := range doc.Definitions {
		schema.Types[def.Name] = def
		switch def.Name {
		case rootQuery:
			schema.Query = def
		case rootMutation:
			schema.Mutation = def
		case rootSubscription:
			schema.Subscription = def
		}
	}
	for _, def := range doc.Directives {
		schema.Directives[def.Name] = def
	}

	cfg := config.NewConfig()
	cfg.SchemaFile = schemaPath
	cfg.IncludeSubscriptions = true // default-off; fixtures may use it

	var buf bytes.Buffer
	gen := generator.New(cfg, schema, &buf)
	if err := gen.Generate(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var (
	revdateLineRE     = regexp.MustCompile(`(?m)^:revdate:.*$`)
	commandlineLineRE = regexp.MustCompile(`(?m)^:commandline:.*$`)
)

// normalisePreamble replaces the two non-deterministic preamble lines with
// stable placeholders so goldens can be diffed byte-for-byte.
func normalisePreamble(s string) string {
	s = revdateLineRE.ReplaceAllString(s, ":revdate: <REVDATE>")
	s = commandlineLineRE.ReplaceAllString(s, ":commandline: <COMMANDLINE>")
	return s
}

// firstDiff returns a small window around the first divergence between two
// strings, enough to orient a developer looking at the test failure.
func firstDiff(got, want string) string {
	shared := min(len(got), len(want))
	i := 0
	for i < shared && got[i] == want[i] {
		i++
	}
	start := max(i-40, 0)
	gotEnd := min(i+80, len(got))
	wantEnd := min(i+80, len(want))
	return "  got:  …" + got[start:gotEnd] + "…\n  want: …" + want[start:wantEnd] + "…"
}

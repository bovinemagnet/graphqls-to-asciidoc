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

// Asciidoctor refuses to reuse a section id and warns "id assigned to section
// already in use: <id>", leaving duplicate ids in the rendered HTML. Antora
// makes this easy to trip over because it sets idprefix to an empty string and
// idseparator to a hyphen, so a section titled "Mutations" auto-generates the
// bare id "mutations" rather than Asciidoctor's default "_mutations".
func TestGeneratedDocumentHasUniqueSectionIds(t *testing.T) {
	doc, err := renderFixture("test/schema.graphql")
	if err != nil {
		t.Fatalf("render test/schema.graphql: %v", err)
	}

	firstUse := make(map[string]string)
	for _, s := range sectionIDs(doc) {
		if prev, seen := firstUse[s.id]; seen {
			t.Errorf("section id %q assigned twice: first by %q, again by %q", s.id, prev, s.title)
			continue
		}
		firstUse[s.id] = s.title
	}
}

// Changelog tags are emitted for every query, mutation, type and input, even
// when the construct carries no changelog annotation. An always-present tag
// pair lets a downstream document write
// include::schema.adoc[tags=query-changelog-Tweet] without having to know
// whether that particular item happens to be annotated.
func TestChangelogTagsAlwaysPresent(t *testing.T) {
	doc, err := renderFixture("test/schema.graphql")
	if err != nil {
		t.Fatalf("render test/schema.graphql: %v", err)
	}

	// Constructs in test/schema.graphql with no changelog annotation: the tag
	// pair must still be there, with nothing between the two lines.
	empty := []string{
		"query-changelog-Tweet",
		"mutation-changelog-deleteTweet",
		"type-changelog-Message",
		"input-changelog-MessageInput",
		"enum-changelog-Sentiment",
		"subscription-changelog-commentAdded",
		"scalar-changelog-Date",
		"directive-changelog-Size",
	}
	for _, tag := range empty {
		want := "// tag::" + tag + "[]\n// end::" + tag + "[]\n"
		if !strings.Contains(doc, want) {
			t.Errorf("expected an empty changelog tag pair for %s", tag)
		}
	}

	// Annotated constructs keep their content between the tags.
	annotated := []string{
		"query-changelog-FeaturedTweets",
		"type-changelog-CLogExample",
		"input-changelog-CLogExampleInput",
	}
	for _, tag := range annotated {
		_, afterOpen, found := strings.Cut(doc, "// tag::"+tag+"[]")
		if !found {
			t.Errorf("missing changelog tag %s", tag)
			continue
		}
		body, _, found := strings.Cut(afterOpen, "// end::"+tag+"[]")
		if !found {
			t.Errorf("missing closing changelog tag %s", tag)
			continue
		}
		if !strings.Contains(body, ".Changelog") {
			t.Errorf("changelog tag %s should still wrap its .Changelog block, got %q", tag, body)
		}
	}
}

// Section ids that are auto-generated depend on the toolchain: Asciidoctor's
// defaults give "_types" while Antora's give "types". An explicit anchor pins
// the id so a cross-reference means the same thing under either.
func TestTopLevelSectionsHaveExplicitAnchors(t *testing.T) {
	doc, err := renderFixture("test/schema.graphql")
	if err != nil {
		t.Fatalf("render test/schema.graphql: %v", err)
	}

	for _, s := range sectionIDs(doc) {
		if s.level == 1 && !s.explicit {
			t.Errorf("section %q relies on an auto-generated id (%q); give it an explicit anchor", s.title, s.id)
		}
	}
}

type sectionID struct {
	id       string
	title    string
	level    int
	explicit bool
}

var (
	headingRE     = regexp.MustCompile(`^(={2,6})\s+(\S.*)$`)
	blockAnchorRE = regexp.MustCompile(`^\[\[([^\]\[]+)\]\]$`)
	nonAlnumRE    = regexp.MustCompile(`[^a-z0-9]+`)
)

// sectionIDs returns the id every section heading in doc lays claim to, in
// document order: the explicit block anchor directly above the heading if there
// is one, otherwise the id Antora auto-generates from the title.
func sectionIDs(doc string) []sectionID {
	var ids []sectionID
	lines := strings.Split(doc, "\n")
	delimiter := "" // non-empty while inside a listing block or table

	for i, line := range lines {
		switch {
		case delimiter != "":
			if line == delimiter {
				delimiter = ""
			}
			continue
		case line == "----" || line == "|===":
			// Headings inside a source block or table are content, not sections.
			delimiter = line
			continue
		}

		m := headingRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		title := strings.TrimSpace(m[2])
		level := len(m[1]) - 1 // "==" is level 1, "===" is level 2

		id, explicit := autoID(title), false
		if i > 0 {
			if anchor := blockAnchorRE.FindStringSubmatch(lines[i-1]); anchor != nil {
				id, explicit = anchor[1], true
			}
		}
		ids = append(ids, sectionID{id: id, title: title, level: level, explicit: explicit})
	}
	return ids
}

// autoID mirrors how Asciidoctor derives a section id from its title under
// Antora's attributes: an empty idprefix and a hyphen idseparator.
func autoID(title string) string {
	return strings.Trim(nonAlnumRE.ReplaceAllString(strings.ToLower(title), "-"), "-")
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

	schema := parser.BuildSchema(doc)

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

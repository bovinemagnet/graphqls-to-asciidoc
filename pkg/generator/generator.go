package generator

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/metrics"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/parser"
)

// Generator handles AsciiDoc generation from GraphQL schemas
type Generator struct {
	config  *config.Config
	schema  *ast.Schema
	writer  io.Writer
	metrics *metrics.Metrics
}

// New creates a new Generator instance
func New(cfg *config.Config, schema *ast.Schema, writer io.Writer) *Generator {
	return &Generator{
		config:  cfg,
		schema:  schema,
		writer:  writer,
		metrics: metrics.New(cfg),
	}
}

// formatDefaultValue returns " = <value>" if a default is set, otherwise empty string.
func formatDefaultValue(defaultValue *ast.Value) string {
	if defaultValue == nil {
		return ""
	}
	return " = " + defaultValue.String()
}

// formatArgumentListItem returns a formatted argument bullet point with optional default value.
func formatArgumentListItem(name, typeName string, defaultValue *ast.Value) string {
	if defaultValue != nil {
		return fmt.Sprintf("* `%s : %s = %s`\n", name, typeName, defaultValue.String())
	}
	return fmt.Sprintf("* `%s : %s`\n", name, typeName)
}

// extractLists separates list items (lines starting with "- " or "* ") from non-list lines.
func extractLists(text string) (nonList, list string) {
	lines := strings.Split(text, "\n")
	var nonListLines, listLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if (strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "-- ")) || strings.HasPrefix(trimmed, "* ") {
			listLines = append(listLines, line)
		} else {
			nonListLines = append(nonListLines, line)
		}
	}
	return strings.Join(nonListLines, "\n"), strings.Join(listLines, "\n")
}

// splitOnArgumentsMarker splits a processed description at any Arguments marker,
// returning the main description and numbered argument references.
// It strips .Parameters: sections and checks for **Arguments:**, .Arguments:, and .Arguments markers.
func splitOnArgumentsMarker(processedDesc string) (mainDesc, numberedRefs string) {
	// First, remove .Parameters: section and anything after it
	paramSplit := strings.Split(processedDesc, ".Parameters:")
	if len(paramSplit) > 1 {
		processedDesc = paramSplit[0]
	}

	parts := strings.Split(processedDesc, "**Arguments:**")
	if len(parts) > 1 {
		return parser.ConvertDashToAsterisk(parts[0]),
			parser.ConvertDescriptionToRefNumbers(parts[1], true)
	}

	parts = strings.Split(processedDesc, ".Arguments:")
	if len(parts) > 1 {
		return parser.ConvertDashToAsterisk(parts[0]),
			parser.ConvertDescriptionToRefNumbers(parts[1], true)
	}

	parts = strings.Split(processedDesc, ".Arguments")
	if len(parts) > 1 {
		return parser.ConvertDashToAsterisk(parts[0]),
			parser.ConvertDescriptionToRefNumbers(parts[1], true)
	}

	// No Arguments marker at all — extract list items as arguments
	mainDesc, numberedRefs = extractLists(processedDesc)
	mainDesc = parser.ConvertDashToAsterisk(mainDesc)
	if strings.TrimSpace(numberedRefs) != "" {
		numberedRefs = parser.ConvertDescriptionToRefNumbers(numberedRefs, true)
	}
	return mainDesc, numberedRefs
}

// defaultFuncMap returns the standard template function map used by most templates.
func defaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"printAsciiDocTagsTmpl":          func(s string) string { return s },
		"convertDescriptionToRefNumbers": func(desc string, _ bool) string { return desc },
	}
}

// executeTemplate parses and executes a named template with the default function map,
// writing to the generator's writer. Errors are logged to stderr.
func (g *Generator) executeTemplate(name, tmplStr string, data interface{}) error {
	tmpl, err := template.New(name).Funcs(defaultFuncMap()).Parse(tmplStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing %s template: %v\n", name, err)
		return err
	}

	if err := tmpl.Execute(g.writer, data); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing %s template: %v\n", name, err)
		return err
	}
	return nil
}

// Generate generates the complete AsciiDoc documentation
func (g *Generator) Generate() error {
	// Check if catalogue mode is enabled
	if g.config.Catalogue {
		return g.generateCatalogue()
	}

	// Log input parameters
	g.metrics.LogInputParameters()

	// Print header
	headerTimer := g.metrics.StartSection("Header")
	g.printHeader()
	headerTimer.AddCount(1)
	headerTimer.Finish()

	// Write catalogue section (summary tables)
	catalogueTimer := g.metrics.StartSection("Catalogue")
	if err := g.writeCatalogueSection(); err != nil {
		return fmt.Errorf("error generating catalogue section: %w", err)
	}
	catalogueTimer.AddCount(1)
	catalogueTimer.Finish()

	// Create definitions map for type processing
	g.metrics.LogProgress("Setup", "Creating definitions map")
	definitionsMap := make(map[string]*ast.Definition)
	for _, def := range g.schema.Types {
		definitionsMap[def.Name] = def
	}

	// Sort definitions
	sortedDefs := make([]*ast.Definition, 0, len(g.schema.Types))
	for _, def := range g.schema.Types {
		sortedDefs = append(sortedDefs, def)
	}
	sort.Slice(sortedDefs, func(i, j int) bool {
		return sortedDefs[i].Name < sortedDefs[j].Name
	})

	g.metrics.LogProgress("Setup", fmt.Sprintf("Found %d total definitions", len(g.schema.Types)))

	// Generate sections based on configuration
	if g.config.IncludeQueries && g.schema.Query != nil {
		timer := g.metrics.StartSection("Queries")
		count := g.generateQueries(definitionsMap)
		timer.AddCount(count)
		timer.Finish()
	}

	if g.config.IncludeMutations && g.schema.Mutation != nil {
		timer := g.metrics.StartSection("Mutations")
		count := g.generateMutations(definitionsMap)
		timer.AddCount(count)
		timer.Finish()
	}

	if g.config.IncludeSubscriptions && g.schema.Subscription != nil {
		timer := g.metrics.StartSection("Subscriptions")
		count := g.generateSubscriptions(definitionsMap)
		timer.AddCount(count)
		timer.Finish()
	}

	if g.config.IncludeTypes {
		timer := g.metrics.StartSection("Types")
		count := g.generateTypes(sortedDefs, definitionsMap)
		timer.AddCount(count)
		timer.Finish()
	}

	if g.config.IncludeEnums {
		timer := g.metrics.StartSection("Enums")
		count := g.generateEnums(sortedDefs)
		timer.AddCount(count)
		timer.Finish()
	}

	if g.config.IncludeInputs {
		timer := g.metrics.StartSection("Inputs")
		count := g.generateInputs(sortedDefs, definitionsMap)
		timer.AddCount(count)
		timer.Finish()
	}

	if g.config.IncludeDirectives {
		timer := g.metrics.StartSection("Directives")
		count := g.generateDirectives()
		timer.AddCount(count)
		timer.Finish()
	}

	if g.config.IncludeScalars {
		timer := g.metrics.StartSection("Scalars")
		count := g.generateScalars(sortedDefs)
		timer.AddCount(count)
		timer.Finish()
	}

	// Log final metrics table
	g.metrics.LogMetricsTable()

	return nil
}

// printHeader prints the AsciiDoc document header
func (g *Generator) printHeader() {
	fmt.Fprintln(g.writer, "= GraphQL Documentation")
	fmt.Fprintln(g.writer, ":toc: left")
	fmt.Fprintf(g.writer, ":revdate: %s\n", time.Now().Format("Mon, 02 Jan 2006 15:04:05 MST"))
	fmt.Fprintf(g.writer, ":commandline: %s\n", strings.Join(os.Args, " "))
	fmt.Fprintf(g.writer, ":sourceFile: %s\n", g.config.SchemaFile)
	fmt.Fprintln(g.writer, ":reproducible:")
	fmt.Fprintln(g.writer, ":page-partial:")
	fmt.Fprintln(g.writer, ":sect-anchors:")
	fmt.Fprintln(g.writer, ":table-caption!:")
	fmt.Fprintln(g.writer, ":table-stripes: even")
	fmt.Fprintln(g.writer, ":pdf-page-size: A4")
	fmt.Fprintln(g.writer, ":tags: api, GraphQL, nodes, types, query")
	fmt.Fprintln(g.writer)
	fmt.Fprintln(g.writer)
	fmt.Fprintln(g.writer, "[IMPORTANT]")
	fmt.Fprintln(g.writer, "====")
	fmt.Fprintf(g.writer, "This is automatically generated from the schema file `%s`. +\n", g.config.SchemaFile)
	fmt.Fprintln(g.writer, "Do not edit this file directly. +")
	fmt.Fprintln(g.writer, "Last generated _{revdate}_")
	fmt.Fprintln(g.writer, "====")
	fmt.Fprintln(g.writer)
}

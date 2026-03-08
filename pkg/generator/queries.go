package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/changelog"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/parser"
)

// generateQueries generates the queries section
func (g *Generator) generateQueries(definitionsMap map[string]*ast.Definition) int {
	if g.schema.Query == nil {
		return 0
	}

	g.metrics.LogProgress("Queries", "Starting query generation")

	fmt.Fprintln(g.writer, "== Query")
	fmt.Fprintln(g.writer)
	fmt.Fprintln(g.writer)
	if g.schema.Query.Description != "" {
		fmt.Fprintln(g.writer, parser.ProcessDescription(g.schema.Query.Description))
	}

	// Collect and filter queries
	var queryFields []*ast.FieldDefinition
	for _, f := range g.schema.Query.Fields {
		if !g.shouldIncludeField(f.Name, f.Description, f.Directives) {
			continue
		}
		queryFields = append(queryFields, f)
	}

	// Sort queries alphabetically by name
	sort.Slice(queryFields, func(i, j int) bool {
		return queryFields[i].Name < queryFields[j].Name
	})

	// Generate documentation for each query
	for _, f := range queryFields {
		g.generateQueryField(f, definitionsMap)
	}

	count := len(queryFields)
	g.metrics.LogProgress("Queries", fmt.Sprintf("Generated %d queries", count))
	return count
}

// generateQueryField generates documentation for a single query field
func (g *Generator) generateQueryField(field *ast.FieldDefinition, definitionsMap map[string]*ast.Definition) {
	fmt.Fprintf(g.writer, "// tag::query-%s[]\n", field.Name)
	fmt.Fprintln(g.writer)
	fmt.Fprintf(g.writer, "[[query_%s]]\n", strings.ToLower(field.Name))
	fmt.Fprintf(g.writer, "=== %s\n", field.Name)
	fmt.Fprintln(g.writer)
	fmt.Fprintln(g.writer)

	// Process description and extract changelog
	processedDesc, changelog := changelog.ProcessWithChangelog(field.Description, parser.ProcessDescription)

	mainDesc, numberedRefs := splitOnArgumentsMarker(processedDesc)

	fmt.Fprintf(g.writer, "// tag::method-description-%s[]\n", field.Name)
	if strings.TrimSpace(mainDesc) != "" {
		fmt.Fprint(g.writer, strings.TrimSpace(mainDesc))
		fmt.Fprintln(g.writer)
	}
	fmt.Fprintf(g.writer, "// end::method-description-%s[]\n", field.Name)
	fmt.Fprintln(g.writer)

	// Generate method signature
	fmt.Fprintf(g.writer, "// tag::method-signature-%s[]\n", field.Name)
	fmt.Fprintf(g.writer, ".query: %s\n", field.Name)
	fmt.Fprintln(g.writer, "[source, kotlin]")
	fmt.Fprintln(g.writer, "----")
	fmt.Fprintf(g.writer, "%s(\n", field.Name)

	// Generate arguments
	for i, arg := range field.Arguments {
		argType := parser.ProcessTypeNameForSignature(arg.Type.String(), definitionsMap)
		fmt.Fprintf(g.writer, "  %s: %s", arg.Name, argType)
		if i < len(field.Arguments)-1 {
			fmt.Fprint(g.writer, " ,")
		}
		fmt.Fprintf(g.writer, " <%d> \n", i+1)
	}

	fmt.Fprintf(g.writer, "): %s <%d>\n", parser.ProcessTypeNameForSignature(field.Type.String(), definitionsMap), len(field.Arguments)+1)
	fmt.Fprintln(g.writer, "----")
	fmt.Fprintf(g.writer, "// end::method-signature-%s[]\n", field.Name)
	fmt.Fprintln(g.writer)

	// Add numbered references from description
	fmt.Fprintf(g.writer, "// tag::method-args-%s[]\n", field.Name)
	if strings.TrimSpace(numberedRefs) != "" {
		fmt.Fprint(g.writer, strings.TrimSpace(numberedRefs))
		fmt.Fprintln(g.writer)
	}
	fmt.Fprintf(g.writer, "// end::method-args-%s[]\n", field.Name)
	fmt.Fprintln(g.writer)

	fmt.Fprintf(g.writer, "// tag::query-name-%s[]\n", field.Name)
	fmt.Fprintf(g.writer, "*Query Name:* _%s_\n", field.Name)
	fmt.Fprintf(g.writer, "// end::query-name-%s[]\n", field.Name)
	fmt.Fprintln(g.writer)

	fmt.Fprintf(g.writer, "// tag::query-return-%s[]\n", field.Name)
	fmt.Fprintf(g.writer, "*Return:* %s\n", parser.ProcessTypeName(field.Type.String(), definitionsMap))
	fmt.Fprintf(g.writer, "// end::query-return-%s[]\n", field.Name)
	fmt.Fprintln(g.writer)

	// Add changelog section right after return
	if changelog != "" {
		fmt.Fprintf(g.writer, "// tag::query-changelog-%s[]\n", field.Name)
		fmt.Fprint(g.writer, changelog)
		fmt.Fprintln(g.writer)
		fmt.Fprintf(g.writer, "// end::query-changelog-%s[]\n", field.Name)
		fmt.Fprintln(g.writer)
	}

	if len(field.Arguments) > 0 {
		fmt.Fprintf(g.writer, "// tag::arguments-%s[]\n", field.Name)
		fmt.Fprintln(g.writer, ".Arguments")
		for _, arg := range field.Arguments {
			fmt.Fprintf(g.writer, "* `%s : %s`\n", arg.Name, arg.Type.String())
		}
		fmt.Fprintf(g.writer, "// end::arguments-%s[]\n", field.Name)
		fmt.Fprintln(g.writer)
	}

	fmt.Fprintf(g.writer, "// end::query-%s[]\n", field.Name)
	fmt.Fprintln(g.writer)
}

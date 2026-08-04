package generator

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/changelog"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/parser"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/templates"
)

const errFieldsTable = "[ERROR generating fields table]"

// Anchored headings for the type-definition sections. The anchors are explicit
// so the ids do not depend on the idprefix/idseparator attributes of the
// rendering toolchain. The scalars anchor lives in ScalarTemplate instead.
const (
	scalarsSectionHeading    = "== Scalars"
	typesSectionHeading      = "[[types]]\n== Types"
	enumsSectionHeading      = "[[enums]]\n== Enums"
	inputsSectionHeading     = "[[inputs]]\n== Inputs"
	directivesSectionHeading = "[[directives]]\n== Directives"
)

func (g *Generator) generateTypes(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) int {
	g.metrics.LogProgress("Types", "Starting types generation")

	var typeInfos []TypeInfo
	count := 0

	for _, t := range sortedDefs {
		if t.Kind != ast.Object || parser.IsBuiltInGraphQLType(t.Name) {
			continue
		}

		// Generate fields table
		fieldsTableString, err := g.getTypeFieldsTableString(t, definitionsMap)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating fields table for type %s: %v\n", t.Name, err)
			fieldsTableString = errFieldsTable
		}

		// Process type description and extract changelog
		processedDesc, changelogText := changelog.ProcessWithChangelog(t.Description, parser.ProcessDescription)

		typeInfo := TypeInfo{
			Name:        t.Name,
			Kind:        string(t.Kind),
			AnchorName:  "type_" + parser.CamelToSnake(t.Name),
			Description: processedDesc,
			FieldsTable: fieldsTableString,
			IsInterface: t.Kind == ast.Interface,
			Changelog:   changelogText,
		}
		typeInfos = append(typeInfos, typeInfo)
		count++
	}

	if len(typeInfos) > 0 {
		data := struct {
			TypesTag string
			Types    []TypeInfo
		}{
			TypesTag: typesSectionHeading,
			Types:    typeInfos,
		}

		if err := g.executeTemplate("types", templates.TypeSectionTemplate, data); err != nil {
			g.metrics.LogProgress("Types", fmt.Sprintf("Generated %d types", count))
			return count
		}
	}

	g.metrics.LogProgress("Types", fmt.Sprintf("Generated %d types", count))
	return count
}

func (g *Generator) generateEnums(sortedDefs []*ast.Definition) int {
	g.metrics.LogProgress("Enums", "Starting enums generation")

	var enumInfos []EnumInfo
	count := 0

	// Filter for enum definitions
	for _, def := range sortedDefs {
		if def.Kind != ast.Enum {
			continue
		}

		// Generate values table
		valuesTableString := g.getEnumValuesTableString(def)

		// Process enum description and extract changelog
		processedDesc, changelogText := changelog.ProcessWithChangelog(def.Description, parser.ProcessDescription)

		enumInfo := EnumInfo{
			Name:        def.Name,
			AnchorName:  "enum_" + parser.CamelToSnake(def.Name),
			Description: processedDesc,
			ValuesTable: valuesTableString,
			Changelog:   changelogText,
		}
		enumInfos = append(enumInfos, enumInfo)
		count++
	}

	if len(enumInfos) > 0 {
		data := struct {
			EnumsTag string
			Enums    []EnumInfo
		}{
			EnumsTag: enumsSectionHeading,
			Enums:    enumInfos,
		}
		g.renderOrNote("Enums", "enums", templates.EnumSectionTemplate, data, count)
	} else {
		g.writeEmptySectionNote(enumsSectionHeading, "No enums exist in this schema.")
	}

	g.metrics.LogProgress("Enums", fmt.Sprintf("Generated %d enums", count))
	return count
}

// renderOrNote executes a section template, logging progress on failure so the
// caller can simply return its count either way.
func (g *Generator) renderOrNote(section, name, tmpl string, data any, count int) {
	if err := g.executeTemplate(name, tmpl, data); err != nil {
		g.metrics.LogProgress(section, fmt.Sprintf("Generated %d %s", count, strings.ToLower(section)))
	}
}

// writeEmptySectionNote writes a section heading followed by a NOTE block
// explaining that the schema defines nothing of that kind.
func (g *Generator) writeEmptySectionNote(heading, note string) {
	fmt.Fprintln(g.writer, heading)
	fmt.Fprintln(g.writer)
	fmt.Fprintln(g.writer, "[NOTE]")
	fmt.Fprintln(g.writer, "====")
	fmt.Fprintln(g.writer, note)
	fmt.Fprintln(g.writer, "====")
	fmt.Fprintln(g.writer)
}

func (g *Generator) generateInputs(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) int {
	g.metrics.LogProgress("Inputs", "Starting inputs generation")

	var inputInfos []InputInfo
	count := 0

	// Filter for input object definitions
	for _, def := range sortedDefs {
		if def.Kind != ast.InputObject {
			continue
		}

		fieldsTableString := g.getInputFieldsTableString(def, definitionsMap)

		processedDesc, changelogText := changelog.ProcessWithChangelog(def.Description, parser.ProcessDescription)

		inputInfo := InputInfo{
			Name:        def.Name,
			AnchorName:  "input_" + parser.CamelToSnake(def.Name),
			Description: processedDesc,
			FieldsTable: fieldsTableString,
			Changelog:   changelogText,
		}
		inputInfos = append(inputInfos, inputInfo)
		count++
	}

	if len(inputInfos) > 0 {
		data := struct {
			InputsTag string
			Inputs    []InputInfo
		}{
			InputsTag: inputsSectionHeading,
			Inputs:    inputInfos,
		}
		g.renderOrNote("Inputs", "inputs", templates.InputSectionTemplate, data, count)
	} else {
		g.writeEmptySectionNote(inputsSectionHeading, "No input types exist in this schema.")
	}

	g.metrics.LogProgress("Inputs", fmt.Sprintf("Generated %d inputs", count))
	return count
}

func (g *Generator) getInputFieldsTableString(
	def *ast.Definition,
	definitionsMap map[string]*ast.Definition,
) string {
	var builder strings.Builder

	builder.WriteString(".input: " + def.Name + "\n")
	builder.WriteString("[options=\"header\",cols=\"2a,2m,2m,5a\"]\n")
	builder.WriteString("|===\n")
	builder.WriteString("| Field | Type | Default | Description \n")

	for _, field := range def.Fields {
		typeName := parser.ProcessTypeName(field.Type.String(), definitionsMap)
		processedDesc, changelogText := changelog.ProcessWithChangelog(field.Description, parser.ProcessDescription)
		desc := processedDesc
		if changelogText != "" {
			desc += "\n" + changelogText
		}
		if field.DefaultValue != nil {
			fmt.Fprintf(&builder, "| `%s` | %s | `%s` | %s\n", field.Name, typeName, field.DefaultValue.String(), desc)
		} else {
			fmt.Fprintf(&builder, "| `%s` | %s | _none_ | %s\n", field.Name, typeName, desc)
		}
	}

	builder.WriteString("|===\n")
	return builder.String()
}

func (g *Generator) generateDirectives() int {
	g.metrics.LogProgress("Directives", "Starting directives generation")

	if len(g.schema.Directives) == 0 {
		g.metrics.LogProgress("Directives", "No directives found")
		return 0
	}

	fmt.Fprintln(g.writer, directivesSectionHeading)
	fmt.Fprintln(g.writer)
	fmt.Fprintln(g.writer, "// tag::DIRECTIVES[]")
	fmt.Fprintln(g.writer)

	// Sort directives by name for consistent output
	var directiveNames []string
	for name := range g.schema.Directives {
		directiveNames = append(directiveNames, name)
	}
	sort.Strings(directiveNames)

	count := 0
	for _, name := range directiveNames {
		directive := g.schema.Directives[name]
		g.generateDirective(directive)
		count++
	}

	fmt.Fprintln(g.writer, "// end::DIRECTIVES[]")

	g.metrics.LogProgress("Directives", fmt.Sprintf("Generated %d directives", count))
	return count
}

// generateDirective generates documentation for a single directive
func (g *Generator) generateDirective(directive *ast.DirectiveDefinition) {
	fmt.Fprintf(g.writer, "// tag::directive-%s[]\n", directive.Name)
	fmt.Fprintln(g.writer)
	fmt.Fprintf(g.writer, "[[directive_%s]]\n", parser.CamelToSnake(directive.Name))
	fmt.Fprintf(g.writer, "=== @%s\n", directive.Name)
	fmt.Fprintln(g.writer)

	// Process description and extract changelog
	processedDesc, changelogText := changelog.ProcessWithChangelog(directive.Description, parser.ProcessDescription)
	if directive.Description != "" {
		fmt.Fprintf(g.writer, "// tag::directive-description-%s[]\n", directive.Name)
		fmt.Fprint(g.writer, processedDesc)
		fmt.Fprintln(g.writer)
		fmt.Fprintf(g.writer, "// end::directive-description-%s[]\n", directive.Name)
		fmt.Fprintln(g.writer)
	}

	// The changelog tag pair is emitted even when empty, so a downstream
	// include of directive-changelog-<name> always resolves.
	fmt.Fprintf(g.writer, "// tag::directive-changelog-%s[]\n", directive.Name)
	if changelogText != "" {
		fmt.Fprint(g.writer, changelogText)
		fmt.Fprintln(g.writer)
	}
	fmt.Fprintf(g.writer, "// end::directive-changelog-%s[]\n", directive.Name)
	fmt.Fprintln(g.writer)

	g.writeDirectiveSignature(directive)
	g.writeDirectiveArguments(directive)
	g.writeDirectiveLocations(directive)
	g.writeDirectiveRepeatable(directive)

	fmt.Fprintf(g.writer, "// end::directive-%s[]\n", directive.Name)
	fmt.Fprintln(g.writer)
}

// writeDirectiveSignature writes the directive declaration as a source block.
func (g *Generator) writeDirectiveSignature(directive *ast.DirectiveDefinition) {
	fmt.Fprintf(g.writer, "// tag::directive-signature-%s[]\n", directive.Name)
	fmt.Fprintln(g.writer, ".Directive Signature")
	fmt.Fprintln(g.writer, "[source, graphql]")
	fmt.Fprintln(g.writer, "----")
	fmt.Fprintf(g.writer, "directive @%s", directive.Name)

	if len(directive.Arguments) > 0 {
		fmt.Fprint(g.writer, "(")
		for i, arg := range directive.Arguments {
			if i > 0 {
				fmt.Fprint(g.writer, ", ")
			}
			fmt.Fprintf(g.writer, "%s: %s", arg.Name, arg.Type.String())
			if arg.DefaultValue != nil {
				fmt.Fprintf(g.writer, " = %s", arg.DefaultValue.String())
			}
		}
		fmt.Fprint(g.writer, ")")
	}

	if len(directive.Locations) > 0 {
		fmt.Fprint(g.writer, " on ")
		for i, location := range directive.Locations {
			if i > 0 {
				fmt.Fprint(g.writer, " | ")
			}
			fmt.Fprint(g.writer, string(location))
		}
	}

	fmt.Fprintln(g.writer)
	fmt.Fprintln(g.writer, "----")
	fmt.Fprintf(g.writer, "// end::directive-signature-%s[]\n", directive.Name)
	fmt.Fprintln(g.writer)
}

// writeDirectiveArguments writes the directive's arguments table, if any.
func (g *Generator) writeDirectiveArguments(directive *ast.DirectiveDefinition) {
	if len(directive.Arguments) == 0 {
		return
	}

	fmt.Fprintf(g.writer, "// tag::directive-arguments-%s[]\n", directive.Name)
	fmt.Fprintf(g.writer, ".@%s Arguments\n", directive.Name)
	fmt.Fprintln(g.writer, "[options=\"header\",stripes=\"even\"]")
	fmt.Fprintln(g.writer, "|===")
	fmt.Fprintln(g.writer, "| Argument | Type | Default | Description")

	for _, arg := range directive.Arguments {
		fmt.Fprintf(g.writer, "| `%s`", arg.Name)
		fmt.Fprintf(g.writer, " | `%s`", arg.Type.String())

		if arg.DefaultValue != nil {
			fmt.Fprintf(g.writer, " | `%s`", arg.DefaultValue.String())
		} else {
			fmt.Fprint(g.writer, " | _none_")
		}

		if arg.Description != "" {
			fmt.Fprintf(g.writer, " | %s", parser.ProcessDescription(arg.Description))
		} else {
			fmt.Fprint(g.writer, " | _No description_")
		}
		fmt.Fprintln(g.writer)
	}

	fmt.Fprintln(g.writer, "|===")
	fmt.Fprintf(g.writer, "// end::directive-arguments-%s[]\n", directive.Name)
	fmt.Fprintln(g.writer)
}

// writeDirectiveLocations lists where the directive may be applied.
func (g *Generator) writeDirectiveLocations(directive *ast.DirectiveDefinition) {
	if len(directive.Locations) == 0 {
		return
	}

	fmt.Fprintf(g.writer, "// tag::directive-locations-%s[]\n", directive.Name)
	fmt.Fprintf(g.writer, ".@%s Usage Locations\n", directive.Name)
	for _, location := range directive.Locations {
		fmt.Fprintf(g.writer, "* `%s`\n", string(location))
	}
	fmt.Fprintf(g.writer, "// end::directive-locations-%s[]\n", directive.Name)
	fmt.Fprintln(g.writer)
}

// writeDirectiveRepeatable notes whether the directive may be repeated.
func (g *Generator) writeDirectiveRepeatable(directive *ast.DirectiveDefinition) {
	if !directive.IsRepeatable {
		return
	}

	fmt.Fprintf(g.writer, "// tag::directive-repeatable-%s[]\n", directive.Name)
	fmt.Fprintln(g.writer, "NOTE: This directive is repeatable and can be used multiple times on the same element.")
	fmt.Fprintf(g.writer, "// end::directive-repeatable-%s[]\n", directive.Name)
	fmt.Fprintln(g.writer)
}

func (g *Generator) generateScalars(sortedDefs []*ast.Definition) int {
	g.metrics.LogProgress("Scalars", "Starting scalars generation")

	var scalarInfos []ScalarInfo
	count := 0

	// Filter for scalar definitions and exclude built-in scalars
	for _, def := range sortedDefs {
		if def.Kind == ast.Scalar && !isBuiltInScalar(def.Name) {
			// Process description and extract changelog
			processedDesc, changelogText := changelog.ProcessWithChangelog(def.Description, parser.ProcessDescription)

			scalarInfo := ScalarInfo{
				Name:        def.Name,
				Description: processedDesc,
				Changelog:   changelogText,
			}
			scalarInfos = append(scalarInfos, scalarInfo)
			count++
		}
	}

	// Prepare data for template
	data := ScalarData{
		ScalarTag:    scalarsSectionHeading,
		FoundScalars: len(scalarInfos) > 0,
		Scalars:      scalarInfos,
	}

	if err := g.executeTemplate("scalars", templates.ScalarTemplate, data); err != nil {
		g.metrics.LogProgress("Scalars", fmt.Sprintf("Generated %d scalars", count))
		return count
	}

	g.metrics.LogProgress("Scalars", fmt.Sprintf("Generated %d scalars", count))
	return count
}

// getTypeFieldsTableString builds the fields table for a type definition
func (g *Generator) getTypeFieldsTableString(
	t *ast.Definition,
	definitionsMap map[string]*ast.Definition,
) (string, error) {
	var builder strings.Builder

	builder.WriteString(".type: " + t.Name + "\n")
	builder.WriteString("[options=\"header\",cols=\"2a,2m,5a\"]\n")
	builder.WriteString("|===\n")
	builder.WriteString("| Type | Field | Description \n")

	for _, f := range t.Fields {
		typeName := parser.ProcessTypeName(f.Type.String(), definitionsMap)
		processedDesc, changelogText := changelog.ProcessWithChangelog(f.Description, parser.ProcessDescription)

		data := FieldData{
			Type:            typeName,
			Name:            f.Name,
			Description:     processedDesc,
			RequiredOrArray: strings.Contains(typeName, "!") || strings.Contains(typeName, "["),
			Changelog:       changelogText,
		}

		tmpl, err := template.New("field").Funcs(template.FuncMap{
			"processDescription": parser.ProcessDescription,
		}).Parse(templates.FieldTemplate)
		if err != nil {
			return "", err
		}

		err = tmpl.Execute(&builder, data)
		if err != nil {
			return "", err
		}
	}

	builder.WriteString("|===\n")
	return builder.String(), nil
}

func (g *Generator) getEnumValuesTableString(e *ast.Definition) string {
	var builder strings.Builder

	builder.WriteString(".enum: " + e.Name + "\n")
	builder.WriteString("[options=\"header\",cols=\"1m,3a\"]\n")
	builder.WriteString("|===\n")
	builder.WriteString("| Value | Description \n")

	for _, value := range e.EnumValues {
		processedDesc, changelogText := changelog.ProcessWithChangelog(value.Description, parser.ProcessDescription)
		fmt.Fprintf(&builder, "| `%s` | %s\n", value.Name, processedDesc)
		if changelogText != "" {
			fmt.Fprintf(&builder, "%s\n", changelogText)
		}
	}

	builder.WriteString("|===\n")
	return builder.String()
}

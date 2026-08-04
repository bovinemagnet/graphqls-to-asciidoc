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

// mutationSectionHeading is the anchored heading for the mutation detail
// section. The anchor is explicit so the id does not depend on the
// idprefix/idseparator attributes of the rendering toolchain.
const mutationSectionHeading = "[[mutation]]\n== Mutation"

// generateMutations generates the mutations section
func (g *Generator) generateMutations(definitionsMap map[string]*ast.Definition) int {
	g.metrics.LogProgress("Mutations", "Starting mutations generation")

	if g.schema.Mutation == nil || len(g.schema.Mutation.Fields) == 0 {
		g.writeEmptyMutations()
		g.metrics.LogProgress("Mutations", "Generated 0 mutations")
		return 0
	}

	mutationInfos := g.collectMutationInfos(definitionsMap)

	// Sort mutations alphabetically by name
	sort.Slice(mutationInfos, func(i, j int) bool {
		return mutationInfos[i].Name < mutationInfos[j].Name
	})

	mutationObjectDescription := ""
	if g.schema.Mutation.Description != "" {
		mutationObjectDescription = parser.ProcessDescription(g.schema.Mutation.Description)
	}

	data := struct {
		MutationTag               string
		MutationObjectDescription string
		FoundMutations            bool
		Mutations                 []MutationInfo
	}{
		MutationTag:               mutationSectionHeading,
		MutationObjectDescription: mutationObjectDescription,
		FoundMutations:            len(mutationInfos) > 0,
		Mutations:                 mutationInfos,
	}

	if err := g.executeTemplate("mutation", templates.MutationTemplate, data); err != nil {
		g.metrics.LogProgress("Mutations", "Generated 0 mutations (template error)")
		return 0
	}

	g.metrics.LogProgress("Mutations", fmt.Sprintf("Generated %d mutations", len(mutationInfos)))
	return len(mutationInfos)
}

// writeEmptyMutations renders the mutation section for a schema that defines
// none, falling back to a plain note if the template will not parse.
func (g *Generator) writeEmptyMutations() {
	tmpl, err := template.New("mutation").Parse(templates.MutationTemplate)
	if err != nil {
		g.writeEmptySectionNote(mutationSectionHeading, "No mutations exist in this schema.")
		return
	}

	if execErr := tmpl.Execute(g.writer, struct {
		MutationTag               string
		MutationObjectDescription string
		FoundMutations            bool
		Mutations                 []MutationInfo
	}{
		MutationTag:               mutationSectionHeading,
		MutationObjectDescription: "",
		FoundMutations:            false,
		Mutations:                 nil,
	}); execErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: template execution error for empty mutations: %v\n", execErr)
	}
}

// collectMutationInfos filters and renders each mutation field.
func (g *Generator) collectMutationInfos(definitionsMap map[string]*ast.Definition) []MutationInfo {
	var mutationInfos []MutationInfo
	for _, f := range g.schema.Mutation.Fields {
		if !g.shouldIncludeField(f.Name, f.Description, f.Directives) {
			continue
		}

		processedDesc, changelogText := changelog.ProcessWithChangelog(f.Description, parser.ProcessDescription)

		numberedRefs := ""
		if len(f.Arguments) > 0 && f.Description != "" {
			processedDesc, numberedRefs = splitOnArgumentsMarker(processedDesc)
		}

		mutationInfos = append(mutationInfos, MutationInfo{
			Name:                 f.Name,
			AnchorName:           "mutation_" + parser.CamelToSnake(f.Name),
			Description:          f.Description,
			CleanedDescription:   processedDesc,
			TypeName:             parser.ProcessTypeName(f.Type.String(), definitionsMap),
			MethodSignatureBlock: g.getMethodSignatureBlock(f, definitionsMap),
			Arguments:            g.getArgumentsBlock(f, definitionsMap),
			Directives:           g.getDirectivesBlock(f),
			HasArguments:         len(f.Arguments) > 0,
			HasDirectives:        len(f.Directives) > 0,
			IsInternal:           isInternal(f.Name, f.Description),
			Changelog:            changelogText,
			NumberedRefs:         parser.CrossReferenceTypeNames(numberedRefs, definitionsMap),
		})
	}
	return mutationInfos
}

// getMethodSignatureBlock builds the method signature block for a mutation
func (g *Generator) getMethodSignatureBlock(f *ast.FieldDefinition, definitionsMap map[string]*ast.Definition) string {
	var b strings.Builder
	fmt.Fprintf(&b, ".mutation: %s\n", f.Name)
	fmt.Fprintln(&b, "[source, kotlin]")
	fmt.Fprintln(&b, "----")
	fmt.Fprintf(&b, "%s(\n", f.Name)
	for i, arg := range f.Arguments {
		typeName := parser.ProcessTypeNameForSignature(arg.Type.String(), definitionsMap)
		fmt.Fprintf(&b, "  %s: %s%s", arg.Name, typeName, formatDefaultValue(arg.DefaultValue))
		if i < len(f.Arguments)-1 {
			fmt.Fprint(&b, " ,")
		}
		fmt.Fprintf(&b, " <%d> \n", i+1)
	}
	fmt.Fprintf(&b, "): %s <%d>\n",
		parser.ProcessTypeNameForSignature(f.Type.String(), definitionsMap),
		len(f.Arguments)+1)
	fmt.Fprint(&b, "----")
	return b.String()
}

// getArgumentsBlock builds the arguments list for a mutation
func (g *Generator) getArgumentsBlock(f *ast.FieldDefinition, definitionsMap map[string]*ast.Definition) string {
	if len(f.Arguments) == 0 {
		return ""
	}
	var b strings.Builder
	for _, arg := range f.Arguments {
		typeName := parser.ProcessTypeName(arg.Type.String(), definitionsMap)
		fmt.Fprint(&b, formatArgumentListItem(arg.Name, typeName, arg.DefaultValue, arg.Directives))
	}
	return b.String()
}

// getDirectivesBlock builds the directives list for a mutation
func (g *Generator) getDirectivesBlock(f *ast.FieldDefinition) string {
	if len(f.Directives) == 0 {
		return ""
	}
	var b strings.Builder
	for _, d := range f.Directives {
		fmt.Fprintf(&b, "* @%s\n", d.Name)
	}
	return b.String()
}

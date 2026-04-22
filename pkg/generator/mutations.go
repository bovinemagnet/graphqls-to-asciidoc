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

// generateMutations generates the mutations section
func (g *Generator) generateMutations(definitionsMap map[string]*ast.Definition) int {
	g.metrics.LogProgress("Mutations", "Starting mutations generation")

	if g.schema.Mutation == nil || len(g.schema.Mutation.Fields) == 0 {
		// No mutations exist
		tmpl, err := template.New("mutation").Parse(templates.MutationTemplate)
		if err == nil {
			if execErr := tmpl.Execute(g.writer, struct {
				MutationTag               string
				MutationObjectDescription string
				FoundMutations            bool
				Mutations                 []MutationInfo
			}{
				MutationTag:               "== Mutations",
				MutationObjectDescription: "",
				FoundMutations:            false,
				Mutations:                 nil,
			}); execErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: template execution error for empty mutations: %v\n", execErr)
			}
		} else {
			fmt.Fprintln(g.writer, "== Mutations")
			fmt.Fprintln(g.writer)
			fmt.Fprintln(g.writer, "[NOTE]")
			fmt.Fprintln(g.writer, "====")
			fmt.Fprintln(g.writer, "No mutations exist in this schema.")
			fmt.Fprintln(g.writer, "====")
			fmt.Fprintln(g.writer)
		}
		g.metrics.LogProgress("Mutations", "Generated 0 mutations")
		return 0
	}

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

		methodSignature := g.getMethodSignatureBlock(f, definitionsMap)
		argsBlock := g.getArgumentsBlock(f, definitionsMap)
		directivesBlock := g.getDirectivesBlock(f)
		mutationInfo := MutationInfo{
			Name:                 f.Name,
			AnchorName:           "mutation_" + parser.CamelToSnake(f.Name),
			Description:          f.Description,
			CleanedDescription:   processedDesc,
			TypeName:             parser.ProcessTypeName(f.Type.String(), definitionsMap),
			MethodSignatureBlock: methodSignature,
			Arguments:            argsBlock,
			Directives:           directivesBlock,
			HasArguments:         len(f.Arguments) > 0,
			HasDirectives:        len(f.Directives) > 0,
			IsInternal:           isInternal(f.Name, f.Description),
			Changelog:            changelogText,
			NumberedRefs:         parser.CrossReferenceTypeNames(numberedRefs, definitionsMap),
		}
		mutationInfos = append(mutationInfos, mutationInfo)
	}

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
		MutationTag:               "== Mutations",
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

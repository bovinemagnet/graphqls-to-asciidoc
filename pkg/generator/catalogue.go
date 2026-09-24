package generator

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/changelog"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/parser"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/templates"
)

// Mutation catalogue group names, derived from the mutation name prefix.
const (
	groupAdds    = "Adds"
	groupUpdates = "Updates"
	groupDeletes = "Deletes"
	groupSaves   = "Saves"
	groupGeneral = "General"
)

// collectCatalogueEntries collects catalogue entries from a schema definition's fields
func (g *Generator) collectCatalogueEntries(def *ast.Definition) []CatalogueEntry {
	if def == nil {
		return nil
	}

	var entries []CatalogueEntry
	for _, field := range def.Fields {
		if !g.shouldIncludeField(field.Name, field.Description, field.Directives) {
			continue
		}

		var description, changelogText string
		if g.config.IncludeChangelog {
			processedDesc, clText := changelog.ProcessWithChangelog(field.Description, parser.ProcessDescription)
			description = parser.ExtractFirstSentence(processedDesc)
			changelogText = clText
		} else {
			description = parser.ExtractFirstSentence(field.Description)
		}

		entries = append(entries, CatalogueEntry{
			Name:        field.Name,
			Signature:   fieldSignature(field),
			Status:      fieldStatus(field.Name, field.Description, field.Directives),
			Description: description,
			Changelog:   changelogText,
		})
	}
	return entries
}

// fieldSignature renders a field as a one-line GraphQL signature, e.g.
// user(id: ID!, includeDeleted: Boolean = false): User
func fieldSignature(field *ast.FieldDefinition) string {
	var b strings.Builder
	b.WriteString(field.Name)
	if len(field.Arguments) > 0 {
		b.WriteString("(")
		for i, arg := range field.Arguments {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(arg.Name)
			b.WriteString(": ")
			b.WriteString(arg.Type.String())
			b.WriteString(formatDefaultValue(arg.DefaultValue))
		}
		b.WriteString(")")
	}
	b.WriteString(": ")
	b.WriteString(field.Type.String())
	return b.String()
}

// collectCatalogueData collects and organises catalogue data for queries, mutations, and subscriptions
func (g *Generator) collectCatalogueData() CatalogueData {
	queries := g.collectCatalogueEntries(g.schema.Query)
	mutations := g.collectCatalogueEntries(g.schema.Mutation)
	subscriptions := g.collectCatalogueEntries(g.schema.Subscription)

	// Sort queries alphabetically by name
	sort.Slice(queries, func(i, j int) bool {
		return queries[i].Name < queries[j].Name
	})

	// Sort mutations alphabetically by name
	sort.Slice(mutations, func(i, j int) bool {
		return mutations[i].Name < mutations[j].Name
	})

	// Group mutations by type
	mutationGroups := groupMutationsByType(mutations)

	// Sort subscriptions alphabetically by name
	sort.Slice(subscriptions, func(i, j int) bool {
		return subscriptions[i].Name < subscriptions[j].Name
	})

	return CatalogueData{
		SubTitle:         g.config.SubTitle,
		IncludeSignature: g.config.IncludeSignature,
		RevDate:          time.Now().Format("Mon, 02 Jan 2006 15:04:05 MST"),
		CommandLine:      strings.Join(os.Args, " "),
		KrokiServerURL:   g.config.KrokiDocumentURL(),
		Queries:          queries,
		Mutations:        mutations,
		MutationGroups:   mutationGroups,
		Subscriptions:    subscriptions,
	}
}

// writeCatalogueSection writes the catalogue tables section to the output.
// This is used in standard documentation mode to include catalogue at the top.
func (g *Generator) writeCatalogueSection() error {
	// Skip catalogue section if all components are disabled
	if !g.config.IncludeQueries && !g.config.IncludeMutations && !g.config.IncludeSubscriptions {
		return nil
	}

	data := g.collectCatalogueData()

	// Build template based on what's enabled
	var templateParts []string

	// Add queries section if enabled and schema defines queries
	if g.config.IncludeQueries && g.schema.Query != nil {
		templateParts = append(templateParts, templates.CatalogueQueriesSection)
	}

	// Add mutations section if enabled and schema defines mutations
	if g.config.IncludeMutations && g.schema.Mutation != nil {
		templateParts = append(templateParts, templates.CatalogueMutationsSection)
	}

	// Add subscriptions section if enabled and schema defines subscriptions
	if g.config.IncludeSubscriptions && g.schema.Subscription != nil {
		templateParts = append(templateParts, templates.CatalogueSubscriptionsSection)
	}

	// Combine all enabled sections
	catalogueBody := strings.Join(templateParts, "")

	tmpl, err := template.New("catalogue-body").Parse(catalogueBody)
	if err != nil {
		return fmt.Errorf("error parsing catalogue body template: %v", err)
	}

	err = tmpl.Execute(g.writer, data)
	if err != nil {
		return fmt.Errorf("error executing catalogue body template: %v", err)
	}

	return nil
}

// generateCatalogue generates the complete catalogue document (catalogue mode)
func (g *Generator) generateCatalogue() error {
	data := g.collectCatalogueData()

	tmpl, err := template.New("catalogue").Parse(templates.CatalogueTemplate)
	if err != nil {
		return fmt.Errorf("error parsing catalogue template: %v", err)
	}

	err = tmpl.Execute(g.writer, data)
	if err != nil {
		return fmt.Errorf("error executing catalogue template: %v", err)
	}

	if g.config.Verbose {
		fmt.Fprintf(os.Stderr, "Generated catalogue with %d queries, %d mutations, and %d subscriptions\n",
			len(data.Queries), len(data.Mutations), len(data.Subscriptions))
	}

	return nil
}

// groupMutationsByType groups mutations by their naming prefix (add, update, delete, save, general)
func groupMutationsByType(mutations []CatalogueEntry) []MutationGroup {
	groupOrder := []string{groupAdds, groupUpdates, groupDeletes, groupSaves, groupGeneral}
	groupMap := make(map[string][]CatalogueEntry)

	for _, mutation := range mutations {
		groupName := getMutationGroupName(mutation.Name)
		groupMap[groupName] = append(groupMap[groupName], mutation)
	}

	var result []MutationGroup
	for _, groupName := range groupOrder {
		if mutations, exists := groupMap[groupName]; exists && len(mutations) > 0 {
			result = append(result, MutationGroup{
				GroupName: groupName,
				Mutations: mutations,
			})
		}
	}

	return result
}

// getMutationGroupName determines the group name based on mutation name prefix
func getMutationGroupName(mutationName string) string {
	lowerName := strings.ToLower(mutationName)

	if strings.HasPrefix(lowerName, "add") {
		return groupAdds
	}
	if strings.HasPrefix(lowerName, "update") {
		return groupUpdates
	}
	if strings.HasPrefix(lowerName, "delete") {
		return groupDeletes
	}
	if strings.HasPrefix(lowerName, "save") {
		return groupSaves
	}

	return groupGeneral
}

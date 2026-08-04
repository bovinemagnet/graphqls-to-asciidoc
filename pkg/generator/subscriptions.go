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

// subscriptionSectionHeading is the anchored heading for the subscription
// detail section.
const subscriptionSectionHeading = "[[subscription]]\n== Subscription"

// generateSubscriptions generates the subscriptions section
func (g *Generator) generateSubscriptions(definitionsMap map[string]*ast.Definition) int {
	g.metrics.LogProgress("Subscriptions", "Starting subscriptions generation")

	if g.schema.Subscription == nil || len(g.schema.Subscription.Fields) == 0 {
		g.writeEmptySubscriptions()
		g.metrics.LogProgress("Subscriptions", "Generated 0 subscriptions")
		return 0
	}

	subscriptionInfos := g.collectSubscriptionInfos(definitionsMap)

	data := struct {
		FoundSubscriptions bool
		Subscriptions      []SubscriptionInfo
	}{
		FoundSubscriptions: len(subscriptionInfos) > 0,
		Subscriptions:      subscriptionInfos,
	}

	if err := g.executeTemplate("subscription", templates.SubscriptionTemplate, data); err != nil {
		g.metrics.LogProgress("Subscriptions", "Generated 0 subscriptions (template error)")
		return 0
	}

	g.metrics.LogProgress("Subscriptions", fmt.Sprintf("Generated %d subscriptions", len(subscriptionInfos)))
	return len(subscriptionInfos)
}

// writeEmptySubscriptions renders the subscription section for a schema that
// defines none, falling back to a plain note if the template will not parse.
func (g *Generator) writeEmptySubscriptions() {
	tmpl, err := template.New("subscription").Parse(templates.SubscriptionTemplate)
	if err != nil {
		g.writeEmptySectionNote(subscriptionSectionHeading, "No subscriptions exist in this schema.")
		return
	}

	if execErr := tmpl.Execute(g.writer, struct {
		FoundSubscriptions bool
		Subscriptions      []SubscriptionInfo
	}{
		FoundSubscriptions: false,
		Subscriptions:      nil,
	}); execErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: template execution error for empty subscriptions: %v\n", execErr)
	}
}

// collectSubscriptionInfos filters, sorts and renders each subscription field.
func (g *Generator) collectSubscriptionInfos(
	definitionsMap map[string]*ast.Definition,
) []SubscriptionInfo {
	var subscriptionFields []*ast.FieldDefinition
	for _, f := range g.schema.Subscription.Fields {
		if !g.shouldIncludeField(f.Name, f.Description, f.Directives) {
			continue
		}
		subscriptionFields = append(subscriptionFields, f)
	}

	sort.Slice(subscriptionFields, func(i, j int) bool {
		return subscriptionFields[i].Name < subscriptionFields[j].Name
	})

	var subscriptionInfos []SubscriptionInfo
	for _, f := range subscriptionFields {
		processedDesc, _ := changelog.ProcessWithChangelog(f.Description, parser.ProcessDescription)
		subscriptionInfos = append(subscriptionInfos, SubscriptionInfo{
			Description: processedDesc,
			Details:     g.getSubscriptionDetails(f, definitionsMap),
		})
	}
	return subscriptionInfos
}

// getSubscriptionDetails builds detailed documentation for a subscription field
func (g *Generator) getSubscriptionDetails(f *ast.FieldDefinition, definitionsMap map[string]*ast.Definition) string {
	var b strings.Builder

	// Generate subscription signature
	fmt.Fprintf(&b, "// tag::subscription-%s[]\n", f.Name)
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "[[subscription_%s]]\n", parser.CamelToSnake(f.Name))
	fmt.Fprintf(&b, "=== %s\n", f.Name)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b)

	writeSubscriptionSignature(&b, f, definitionsMap)

	// Add subscription name
	fmt.Fprintf(&b, "// tag::subscription-name-%s[]\n", f.Name)
	fmt.Fprintf(&b, "*Subscription Name:* _%s_\n", f.Name)
	fmt.Fprintf(&b, "// end::subscription-name-%s[]\n", f.Name)
	fmt.Fprintln(&b)

	// Add return type
	fmt.Fprintf(&b, "// tag::subscription-return-%s[]\n", f.Name)
	fmt.Fprintf(&b, "*Return:* %s\n", parser.ProcessTypeName(f.Type.String(), definitionsMap))
	fmt.Fprintf(&b, "// end::subscription-return-%s[]\n", f.Name)
	fmt.Fprintln(&b)

	// Add changelog. The tag pair is emitted even when the subscription has no
	// changelog, so a downstream include of subscription-changelog-<name>
	// always resolves.
	_, changelogText := changelog.ProcessWithChangelog(f.Description, parser.ProcessDescription)
	fmt.Fprintf(&b, "// tag::subscription-changelog-%s[]\n", f.Name)
	if changelogText != "" {
		fmt.Fprint(&b, changelogText)
		fmt.Fprintln(&b)
	}
	fmt.Fprintf(&b, "// end::subscription-changelog-%s[]\n", f.Name)
	fmt.Fprintln(&b)

	writeSubscriptionArguments(&b, f)
	writeSubscriptionDirectives(&b, f)

	fmt.Fprintf(&b, "// end::subscription-%s[]\n", f.Name)
	fmt.Fprintln(&b)

	return b.String()
}

// writeSubscriptionSignature writes the subscription call signature block.
func writeSubscriptionSignature(
	b *strings.Builder,
	f *ast.FieldDefinition,
	definitionsMap map[string]*ast.Definition,
) {
	fmt.Fprintf(b, "// tag::subscription-signature-%s[]\n", f.Name)
	fmt.Fprintf(b, ".subscription: %s\n", f.Name)
	fmt.Fprintln(b, "[source, kotlin]")
	fmt.Fprintln(b, "----")
	fmt.Fprintf(b, "%s(\n", f.Name)

	for i, arg := range f.Arguments {
		argType := parser.ProcessTypeNameForSignature(arg.Type.String(), definitionsMap)
		fmt.Fprintf(b, "  %s: %s%s", arg.Name, argType, formatDefaultValue(arg.DefaultValue))
		if i < len(f.Arguments)-1 {
			fmt.Fprint(b, " ,")
		}
		fmt.Fprintf(b, " <%d> \n", i+1)
	}

	fmt.Fprintf(b, "): %s <%d>\n",
		parser.ProcessTypeNameForSignature(f.Type.String(), definitionsMap),
		len(f.Arguments)+1)
	fmt.Fprintln(b, "----")
	fmt.Fprintf(b, "// end::subscription-signature-%s[]\n", f.Name)
	fmt.Fprintln(b)
}

// writeSubscriptionArguments writes the subscription's argument list, if any.
func writeSubscriptionArguments(b *strings.Builder, f *ast.FieldDefinition) {
	if len(f.Arguments) == 0 {
		return
	}
	fmt.Fprintf(b, "// tag::subscription-arguments-%s[]\n", f.Name)
	fmt.Fprintln(b, ".Arguments")
	for _, arg := range f.Arguments {
		fmt.Fprint(b, formatArgumentListItem(arg.Name, arg.Type.String(), arg.DefaultValue, arg.Directives))
	}
	fmt.Fprintf(b, "// end::subscription-arguments-%s[]\n", f.Name)
	fmt.Fprintln(b)
}

// writeSubscriptionDirectives writes the subscription's directive list, if any.
func writeSubscriptionDirectives(b *strings.Builder, f *ast.FieldDefinition) {
	if len(f.Directives) == 0 {
		return
	}
	fmt.Fprintf(b, "// tag::subscription-directives-%s[]\n", f.Name)
	fmt.Fprintln(b, ".Directives")
	for _, d := range f.Directives {
		fmt.Fprintf(b, "* @%s\n", d.Name)
	}
	fmt.Fprintf(b, "// end::subscription-directives-%s[]\n", f.Name)
	fmt.Fprintln(b)
}

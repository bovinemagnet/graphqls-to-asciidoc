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

// generateSubscriptions generates the subscriptions section
func (g *Generator) generateSubscriptions(definitionsMap map[string]*ast.Definition) int {
	g.metrics.LogProgress("Subscriptions", "Starting subscriptions generation")

	if g.schema.Subscription == nil || len(g.schema.Subscription.Fields) == 0 {
		// No subscriptions exist
		tmpl, err := template.New("subscription").Parse(templates.SubscriptionTemplate)
		if err == nil {
			if execErr := tmpl.Execute(g.writer, struct {
				FoundSubscriptions bool
				Subscriptions      []SubscriptionInfo
			}{
				FoundSubscriptions: false,
				Subscriptions:      nil,
			}); execErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: template execution error for empty subscriptions: %v\n", execErr)
			}
		} else {
			fmt.Fprintln(g.writer, "== Subscription")
			fmt.Fprintln(g.writer)
			fmt.Fprintln(g.writer, "[NOTE]")
			fmt.Fprintln(g.writer, "====")
			fmt.Fprintln(g.writer, "No subscriptions exist in this schema.")
			fmt.Fprintln(g.writer, "====")
			fmt.Fprintln(g.writer)
		}
		g.metrics.LogProgress("Subscriptions", "Generated 0 subscriptions")
		return 0
	}

	// Collect and filter subscriptions
	var subscriptionFields []*ast.FieldDefinition
	for _, f := range g.schema.Subscription.Fields {
		if !g.shouldIncludeField(f.Name, f.Description, f.Directives) {
			continue
		}
		subscriptionFields = append(subscriptionFields, f)
	}

	// Sort subscriptions alphabetically by name
	sort.Slice(subscriptionFields, func(i, j int) bool {
		return subscriptionFields[i].Name < subscriptionFields[j].Name
	})

	// Generate subscription info for each subscription
	var subscriptionInfos []SubscriptionInfo
	for _, f := range subscriptionFields {
		processedDesc, _ := changelog.ProcessWithChangelog(f.Description, parser.ProcessDescription)
		details := g.getSubscriptionDetails(f, definitionsMap)

		subscriptionInfo := SubscriptionInfo{
			Description: processedDesc,
			Details:     details,
		}
		subscriptionInfos = append(subscriptionInfos, subscriptionInfo)
	}

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

// getSubscriptionDetails builds detailed documentation for a subscription field
func (g *Generator) getSubscriptionDetails(f *ast.FieldDefinition, definitionsMap map[string]*ast.Definition) string {
	var b strings.Builder

	// Generate subscription signature
	fmt.Fprintf(&b, "// tag::subscription-%s[]\n", f.Name)
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "[[subscription_%s]]\n", strings.ToLower(f.Name))
	fmt.Fprintf(&b, "=== %s\n", f.Name)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b)

	// Generate method signature
	fmt.Fprintf(&b, "// tag::subscription-signature-%s[]\n", f.Name)
	fmt.Fprintf(&b, ".subscription: %s\n", f.Name)
	fmt.Fprintln(&b, "[source, kotlin]")
	fmt.Fprintln(&b, "----")
	fmt.Fprintf(&b, "%s(\n", f.Name)

	// Generate arguments
	for i, arg := range f.Arguments {
		argType := parser.ProcessTypeNameForSignature(arg.Type.String(), definitionsMap)
		fmt.Fprintf(&b, "  %s: %s%s", arg.Name, argType, formatDefaultValue(arg.DefaultValue))
		if i < len(f.Arguments)-1 {
			fmt.Fprint(&b, " ,")
		}
		fmt.Fprintf(&b, " <%d> \n", i+1)
	}

	fmt.Fprintf(&b, "): %s <%d>\n", parser.ProcessTypeNameForSignature(f.Type.String(), definitionsMap), len(f.Arguments)+1)
	fmt.Fprintln(&b, "----")
	fmt.Fprintf(&b, "// end::subscription-signature-%s[]\n", f.Name)
	fmt.Fprintln(&b)

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

	// Add arguments if any
	if len(f.Arguments) > 0 {
		fmt.Fprintf(&b, "// tag::subscription-arguments-%s[]\n", f.Name)
		fmt.Fprintln(&b, ".Arguments")
		for _, arg := range f.Arguments {
			fmt.Fprint(&b, formatArgumentListItem(arg.Name, arg.Type.String(), arg.DefaultValue, arg.Directives))
		}
		fmt.Fprintf(&b, "// end::subscription-arguments-%s[]\n", f.Name)
		fmt.Fprintln(&b)
	}

	// Add directives if any
	if len(f.Directives) > 0 {
		fmt.Fprintf(&b, "// tag::subscription-directives-%s[]\n", f.Name)
		fmt.Fprintln(&b, ".Directives")
		for _, d := range f.Directives {
			fmt.Fprintf(&b, "* @%s\n", d.Name)
		}
		fmt.Fprintf(&b, "// end::subscription-directives-%s[]\n", f.Name)
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "// end::subscription-%s[]\n", f.Name)
	fmt.Fprintln(&b)

	return b.String()
}

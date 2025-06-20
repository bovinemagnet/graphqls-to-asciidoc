package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

var Version = "development"

var excludeInternal = flag.Bool("exclude-internal", false, "Exclude internal queries from output")
var schemaFile = flag.String("schema", "", "Path to the GraphQL schema file")
var includeMutations = flag.Bool("mutations", true, "Include mutations in the output")
var includeQueries = flag.Bool("queries", true, "Include queries in the output")
var includeSubscriptions = flag.Bool("subscriptions", false, "Include subscriptions in the output")
var includeDirectives = flag.Bool("directives", true, "Include directives in the output")
var includeTypes = flag.Bool("types", true, "Include types in the output")
var includeEnums = flag.Bool("enums", true, "Include enums in the output")
var includeInputs = flag.Bool("inputs", true, "Include inputs in the output")
var includeScalars = flag.Bool("scalars", true, "Include scalars in the output")

//var outputFile = flag.String("output", "", "Output file for the documentation")
//var showVersion = flag.Bool("version", false, "Show program version")

var fieldTemplate = `
| {{.Type}} | {{.Name}} | {{processDescription .Description}}
{{- if .RequiredOrArray}}

.Notes:
{{- end}}
{{- if .Required}}

.Required:
* {{.Required}}
{{- end}}
{{- if .IsArray}}
.Array:
* True
{{- end}}
{{- if .Directives}}

.Directives:
{{.Directives}}
{{- end }}
{{- if .Changelog}}
{{.Changelog}}
{{- end }}
`

const scalarTemplate = `
// tag::scalar[]
[[scalars]]
{{.ScalarTag}}

GraphQL specifies a basic set of well-defined Scalar types: Int, Float, String, Boolean, and ID.
{{- if .FoundScalars }}

The following custom scalar types are defined in this schema:

{{- range .Scalars }}
// tag::scalar-{{.Name}}[]
[[scalar-{{.Name}}]]
=== {{.Name}}

{{- if .Description }}

// tag::scalar-description-{{.Name}}[]
{{ .Description | printAsciiDocTagsTmpl }}
// end::scalar-description-{{.Name}}[]

{{ end }}
// end::scalar-{{.Name}}[]

{{ end }}
{{- else }}
[NOTE]
====
No custom scalars exist in this schema.
====
{{ end }}
// end::scalar[]
`

const subscriptionTemplate = `
// tag::subscription[]
== Subscription

{{- if .FoundSubscriptions }}
{{- range .Subscriptions }}
{{- if .Description }}
{{ .Description | printAsciiDocTagsTmpl }}
{{ end }}

{{ .Details }}

{{ end }}
{{- else }}
[NOTE]
====
No subscriptions exist in this schema.
====
{{ end }}
// end::subscription[]
`

const mutationTemplate = `
// tag::mutation[]
[[mutations]]
{{.MutationTag}}

{{- if .MutationObjectDescription }}
{{ .MutationObjectDescription | printAsciiDocTagsTmpl }}
{{- end }}

GraphQL Mutations are entry points on a GraphQL server that provides write access to our data sources.

{{- if .FoundMutations }}

{{- range .Mutations }}
// tag::mutation-{{.Name}}[]
[[{{.AnchorName}}]]
// tag::method-signature-{{.Name}}[]
=== {{.Name}}{{ if .IsInternal }} [INTERNAL]{{ end }}
// end::method-signature-{{.Name}}[]

// tag::method-description-{{.Name}}[]
{{- if .CleanedDescription }}
{{ .CleanedDescription | printAsciiDocTagsTmpl }}
{{- end }}
// end::method-description-{{.Name}}[]

{{ .MethodSignatureBlock }}

// tag::method-args-{{.Name}}[]
{{ convertDescriptionToRefNumbers .Description true }}
// end::method-args-{{.Name}}[]

// tag::mutation-name-{{.Name}}[]
*Mutation Name:* _{{ .Name }}_
// end::mutation-name-{{.Name}}[]

// tag::mutation-return-{{.Name}}[]
*Return:* {{ .TypeName }}
// end::mutation-return-{{.Name}}[]

{{- if .HasArguments }}
// tag::arguments-{{.Name}}[]
.Arguments
{{ .Arguments }}
// end::arguments-{{.Name}}[]
{{- end }}

{{- if .HasDirectives }}
// tag::mutation-directives-{{.Name}}[]
.Directives
{{ .Directives }}
// end::mutation-directives-{{.Name}}[]
{{- end }}

{{- if .Changelog }}
// tag::mutation-changelog-{{.Name}}[]
{{ .Changelog }}
// end::mutation-changelog-{{.Name}}[]
{{- end }}

// end::mutation-{{.Name}}[]
{{ end }}
{{- else }}
[NOTE]
====
No mutations exist in this schema.
====
{{ end }}
// end::mutation[]
`

// Add this template constant
const typeSectionTemplate = `
{{.TypesTag}}
{{range .Types}}
// tag::type-{{.Name}}[]
[[{{.AnchorName}}]]
=== {{.Name}}

{{- if .Description }}
// tag::type-description-{{.Name}}[]
{{ .Description | printAsciiDocTagsTmpl }}
// end::type-description-{{.Name}}[]
{{- end }}

{{- if .Changelog }}
// tag::type-changelog-{{.Name}}[]
{{ .Changelog }}
// end::type-changelog-{{.Name}}[]
{{- end }}

{{ .FieldsTable }}

// end::type-{{.Name}}[]

{{end}}
`

const enumSectionTemplate = `
{{.EnumsTag}}
{{range .Enums}}
// tag::enum-def-{{.Name}}[]
[[{{.AnchorName}}]]

=== {{.Name}}

{{- if .Description }}
// tag::enum-description-{{.Name}}[]
{{ .Description | printAsciiDocTagsTmpl }}
// end::enum-description-{{.Name}}[]
{{- end }}

{{ .ValuesTable }}

// end::enum-def-{{.Name}}[]

{{end}}
`

const directiveSectionTemplate = `
{{.DirectivesTag}}
{{- if .FoundDirectives }}

{{ .TableOptions }}
|===
| Directive | Arguments | Description
{{- range .Directives }}
| @{{.Name}} | {{.Arguments}} | {{.Description}}
{{- end }}
|===

{{- else }}
[NOTE]
====
No custom directives exist in this schema.
====
{{- end }}
`

const inputSectionTemplate = `
{{.InputsTag}}
{{range .Inputs}}
// tag::input-def-{{.Name}}[]
[[{{.AnchorName}}]]

=== {{.Name}}

{{- if .Description }}
// tag::input-description-{{.Name}}[] // Using generic description tags
{{ .Description | printAsciiDocTagsTmpl }}
// end::input-description-{{.Name}}[]
{{- end }}

{{- if .Changelog }}
// tag::input-changelog-{{.Name}}[]
{{ .Changelog }}
// end::input-changelog-{{.Name}}[]
{{- end }}

{{ .FieldsTable }}

// end::input-def-{{.Name}}[]

{{end}}
`

const (
	// include tags
	includeAdocLineTags = true
	// AsciiDoc tags
	CROSS_REF      = "[[%s]]\n"
	L2_TAG         = "== %s\n\n"
	L3_TAG         = "=== %s\n\n"
	TYPES_TAG      = "== Types\n"
	ENUM_TAG       = "== Enumerations\n"
	INPUT_TAG      = "== Inputs\n"
	DIRECTIVES_TAG = "== Directives\n"
	SCALAR_TAG     = "== Scalars\n"

	ADOC_INPUT_DEF_START_TAG = "// tag::input-def-%s[]\n"
	ADOC_INPUT_DEF_END_TAG   = "// end::input-def-%s[]\n"
	ADOC_INPUT_START_TAG     = "// tag::input-%s[]\n"
	ADOC_INPUT_END_TAG       = "// end::input-%s[]\n"

	ADOC_NODE_START_TAG = "// tag::node-%s[]\n"
	ADOC_NODE_END_TAG   = "// end::node-%s[]\n"

	ADOC_QUERY_START_TAG = "// tag::query-%s[]\n"
	ADOC_QUERY_END_TAG   = "// end::query-%s[]\n"

	ADOC_ARGUMENTS_START_TAG = "// tag::arguments-%s[]\n"
	ADOC_ARGUMENTS_END_TAG   = "// end::arguments-%s[]\n"

	ADOC_ENUM_START_TAG      = "// tag::enum-%s[]\n"
	ADOC_ENUM_END_TAG        = "// end::enum-%s[]\n"
	ADOC_ENUM_DEF_START_TAG  = "// tag::enum-def-%s[]\n"
	ADOC_ENUM_DEF_END_TAG    = "// end::enum-def-%s[]\n"
	ADOC_ENUM_DESC_START_TAG = "// tag::enum-description-%s[]\n"
	ADOC_ENUM_DESC_END_TAG   = "// end::enum-description-%s[]\n"

	ADOC_SCALAR_SEC_START_TAG  = "// tag::scalar[]\n"
	ADOC_SCALAR_SEC_END_TAG    = "// end::scalar[]\n"
	ADOC_SCALAR_START_TAG      = "// tag::scalar-%s[]\n"
	ADOC_SCALAR_END_TAG        = "// end::scalar-%s[]\n"
	ADOC_SCALAR_DESC_START_TAG = "// tag::scalar-description-%s[]\n"
	ADOC_SCALAR_DESC_END_TAG   = "// end::scalar-description-%s[]\n"

	ADOC_METHOD_SIG_START_TAG  = "// tag::method-signature-%s[]\n"
	ADOC_METHOD_SIG_END_TAG    = "// end::method-signature-%s[]\n"
	ADOC_METHOD_DESC_START_TAG = "// tag::method-description-%s[]\n"
	ADOC_METHOD_DESC_END_TAG   = "// end::method-description-%s[]\n"
	ADOC_METHOD_ARGS_START_TAG = "// tag::method-args-%s[]\n"
	ADOC_METHOD_ARGS_END_TAG   = "// end::method-args-%s[]\n"

	ADOC_MUTATION_START_TAG      = "// tag::mutation-%s[]\n"
	ADOC_MUTATION_END_TAG        = "// end::mutation-%s[]\n"
	ADOC_MUTATION_DESC_START_TAG = "// tag::mutation-description-%s[]\n"
	ADOC_MUTATION_DESC_END_TAG   = "// end::mutation-description-%s[]\n"
	ADOC_MUTATION_ARGS_START_TAG = "// tag::mutation-args-%s[]\n"
	ADOC_MUTATION_ARGS_END_TAG   = "// end::mutation-args-%s[]\n"

	ADOC_SUBSCRIPTION_START_TAG      = "// tag::subscription-%s[]\n"
	ADOC_SUBSCRIPTION_END_TAG        = "// end::subscription-%s[]\n"
	ADOC_SUBSCRIPTION_DESC_START_TAG = "// tag::subscription-description-%s[]\n"
	ADOC_SUBSCRIPTION_DESC_END_TAG   = "// end::subscription-description-%s[]\n"
	ADOC_SUBSCRIPTION_ARGS_START_TAG = "// tag::subscription-args-%s[]\n"
	ADOC_SUBSCRIPTION_ARGS_END_TAG   = "// end::subscription-args-%s[]\n"

	ADOC_TYPE_START_TAG           = "// tag::type-%s[]\n"
	ADOC_TYPE_END_TAG             = "// end::type-%s[]\n"
	ADOC_TYPE_DESC_START_TAG      = "// tag::type-description-%s[]\n"
	ADOC_TYPE_DESC_END_TAG        = "// end::type-description-%s[]\n"
	ADOC_TYPE_DEF_START_TAG       = "// tag::type-def-%s[]\n"
	ADOC_TYPE_DEF_END_TAG         = "// end::type-def-%s[]\n"
	ADOC_TYPE_DIRECTIVE_START_TAG = "// tag::type-directive-%s[]\n"
	ADOC_TYPE_DIRECTIVE_END_TAG   = "// end::type-directive-%s[]\n"
	TABLE_SE                      = "|==="

	SOURCE_HEAD = "[source, kotlin]\n"

	TABLE_OPTIONS_2 = "[width=\"90%\", cols=\"2a,6a\" options=\"header\" orientation=\"landscape\" grid=\"none\" stripes=\"even\"  , frame=\"topbot\"]"
	TABLE_OPTIONS_3 = "[width=\"90%\", cols=\"2a,2a,6a\" options=\"header\" orientation=\"landscape\" grid=\"none\" stripes=\"even\" , frame=\"topbot\"]"
	TABLE_OPTIONS_4 = "[width=\"90%\", cols=\"2a,5a,6a,4a\" options=\"header\" orientation=\"landscape\" grid=\"none\" stripes=\"even\" , frame=\"topbot\"]"
	TABLE_OPTIONS_5 = "[width=\"90%\", cols=\"2a,2a,4a,4a\" options=\"header\" orientation=\"landscape\" grid=\"none\" stripes=\"even\" , frame=\"topbot\"]"
)

type FieldData struct {
	Type            string
	Name            string
	Description     string
	RequiredOrArray bool
	Required        string
	IsArray         bool
	Directives      string
	Changelog       string
}

type ScalarData struct {
	ScalarTag    string
	FoundScalars bool
	Scalars      []Scalar
}

type Scalar struct {
	Name        string
	Description string
	CrossRef    string
	L3Tag       string
}

type SubscriptionData struct {
	FoundSubscriptions bool
	Subscriptions      []Subscription
}

type Subscription struct {
	Description string
	Details     string
}

func init() {
	template.Must(template.New("field").Funcs(template.FuncMap{
		"processDescription": processDescription,
	}).Parse(fieldTemplate))
}

type MutationData struct {
	Mutations                 []Mutation
	MutationObjectDescription string
	FoundMutations            bool
	MutationTag               string
}

type Mutation struct {
	Name                 string
	Description          string
	CleanedDescription   string
	TypeName             string
	Arguments            string
	Directives           string
	HasArguments         bool
	HasDirectives        bool
	AnchorName           string
	IsInternal           bool
	MethodSignatureBlock string
	Changelog            string
}

// Add these structs
type TypeSectionData struct {
	TypesTag string
	Types    []TypeInfo
}

type TypeInfo struct {
	Name        string
	Kind        string // "Object" or "Interface"
	AnchorName  string
	Description string
	FieldsTable string // Pre-rendered AsciiDoc table for fields
	IsInterface bool
	Changelog   string
}

// Add these structs
type DirectiveSectionData struct {
	DirectivesTag   string
	FoundDirectives bool
	Directives      []DirectiveInfo
	TableOptions    string // Added field for table options
}

type DirectiveInfo struct {
	Name        string
	Arguments   string // Pre-formatted arguments string
	Description string
}

// Add these structs
type EnumSectionData struct {
	EnumsTag string
	Enums    []EnumInfo
}

type EnumInfo struct {
	Name        string
	AnchorName  string
	Description string
	ValuesTable string // Pre-rendered AsciiDoc table for values
}

// Add these structs
type InputSectionData struct {
	InputsTag string
	Inputs    []InputInfo
}

type InputInfo struct {
	Name        string
	AnchorName  string
	Description string
	FieldsTable string // Pre-rendered AsciiDoc table for fields
	Changelog   string
}

func main() {

	// Parse command-line flags
	flag.Parse()

	// Get non-flag arguments
	//args := flag.Args()
	//if len(args) != 1 {
	//	log.Fatal("Usage: ./program [options] schema.graphql")
	//}

	b, err := os.ReadFile(*schemaFile)
	if err != nil {
		log.Fatalf("Failed to read schema file %s: %v", *schemaFile, err)
	}

	source := &ast.Source{
		Name:  "GraphQL schema",
		Input: string(b),
	}

	doc, gqlErr := parser.ParseSchema(source)
	if gqlErr != nil {
		log.Fatal(gqlErr)
	}

	definitionsMap := make(map[string]*ast.Definition)
	for _, def := range doc.Definitions {
		definitionsMap[def.Name] = def
	}

	// Sort definitions by name
	sortedDefs := make([]*ast.Definition, len(doc.Definitions))
	copy(sortedDefs, doc.Definitions)
	sort.Slice(sortedDefs, func(i, j int) bool {
		return sortedDefs[i].Name < sortedDefs[j].Name
	})

	// Print heading and current time
	fmt.Println("= GraphQL Documentation")
	fmt.Println(":toc: left")
	fmt.Printf(":revdate: %s\n", time.Now().Format(time.RFC1123))
	fmt.Printf(":commandline: %s\n", strings.Join(os.Args, " "))
	fmt.Printf(":sourceFile: %s\n", *schemaFile)
	fmt.Println(":reproducible:")
	fmt.Println(":page-partial:")
	fmt.Println(":sect-anchors:")
	fmt.Println(":table-caption!:")
	fmt.Println(":table-stripes: even")

	fmt.Println(":pdf-page-size: A4")
	fmt.Println(":tags: api, GraphQL, nodes, types, query")

	fmt.Print("\n\n")

	fmt.Println("[IMPORTANT]")
	fmt.Println("====")
	fmt.Printf("This is automatically generated from the schema file `%s`. +\n", *schemaFile)
	fmt.Println("Do not edit this file directly. +")
	fmt.Println("Last generated _{revdate}_")
	fmt.Println("====")
	// Add a blank line.
	fmt.Println()

	if *includeQueries {
		printQueries(sortedDefs, definitionsMap)
		fmt.Println()
	}

	if *includeMutations {
		//printMutations(sortedDefs, definitionsMap)
		printMutationsTmpl(sortedDefs, definitionsMap)
		fmt.Println()
	}

	if *includeSubscriptions {
		//printSubscriptions(sortedDefs, definitionsMap)
		printSubscriptionsTmpl(sortedDefs, definitionsMap)
		fmt.Println()
	}

	if *includeTypes {
		//printTypes(sortedDefs, definitionsMap) // Comment out the old call
		printTypesTmpl(sortedDefs, definitionsMap) // Add the new call
		fmt.Println()
	}

	if *includeEnums {
		// printEnums(sortedDefs, definitionsMap) // Comment out old call
		printEnumsTmpl(sortedDefs, definitionsMap) // Add new call
		fmt.Println()
	}

	if *includeInputs {

		// printInputs(sortedDefs, definitionsMap) // Comment out old call
		printInputsTmpl(sortedDefs, definitionsMap) // Add new call
		fmt.Println()
	}
	if *includeDirectives {
		// Add directives documentation
		// printDirectives(doc) // Comment out the old call
		printDirectivesTmpl(doc) // Add the new call
		fmt.Println()
	}

	if *includeScalars {
		//printScalars(sortedDefs, definitionsMap) // Add this line
		printScalarsTmpl(sortedDefs, definitionsMap)
		fmt.Println()
	}
}

/**
 * Print the type details
 */
func printTypesTmpl(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) {
	var typeInfos []TypeInfo

	for _, t := range sortedDefs {
		// Include Objects (except Query) and Interfaces
		if (t.Kind == ast.Object && t.Name != "Query") || t.Kind == ast.Interface {

			fieldsTableString, err := getTypeFieldsTableString(t, definitionsMap)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating fields table for type %s: %v\n", t.Name, err)
				// Decide if you want to skip this type or continue with an empty table
				fieldsTableString = "[ERROR generating fields table]"
			}

			// Process type description and extract changelog
			processedDesc, changelog := processDescriptionWithChangelog(t.Description)
			
			typeInfo := TypeInfo{
				Name:        t.Name,
				Kind:        string(t.Kind),
				AnchorName:  "type_" + camelToSnake(t.Name), // Use type_ prefix consistently
				Description: processedDesc,
				FieldsTable: fieldsTableString,
				IsInterface: t.Kind == ast.Interface,
				Changelog:   changelog,
			}
			typeInfos = append(typeInfos, typeInfo)
		}
	}

	data := TypeSectionData{
		TypesTag: TYPES_TAG, // Use existing constant
		Types:    typeInfos,
	}

	funcMap := template.FuncMap{
		"printAsciiDocTagsTmpl": printAsciiDocTagsTmpl,
	}

	tmpl, err := template.New("typeSectionTemplate").Funcs(funcMap).Parse(typeSectionTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing type section template: %v\n", err)
		return
	}

	err = tmpl.Execute(os.Stdout, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing type section template: %v\n", err)
	}
}

func processDescription(description string) string {
	// First convert markdown code blocks to AsciiDoc format
	processed := convertMarkdownCodeBlocks(description)
	
	// Format @deprecated directives with backticks if not already enclosed
	processed = formatDeprecatedDirectives(processed)
	
	// Replace * and - with newline followed by the character, but only when they start list items
	// Use regex to replace asterisks only when they start list items
	reAsterisk := regexp.MustCompile(`(^|\s)\*\s`)
	processed = reAsterisk.ReplaceAllString(processed, "${1}* ")
	
	// Use regex to replace hyphens only when they start list items
	// Match: start of line OR whitespace, followed by hyphen, followed by space
	reHyphen := regexp.MustCompile(`(^|\s)-\s`)
	processed = reHyphen.ReplaceAllString(processed, "${1}* ")
	
	// Remove any double newlines that might have been created
	//processed = strings.ReplaceAll(processed, "\n\n", "\n")
	// Remove newline at start if present
	return strings.TrimPrefix(processed, "\n")
}

// formatDeprecatedDirectives wraps @deprecated directives in backticks if not already enclosed
func formatDeprecatedDirectives(description string) string {
	// Regex to match @deprecated directives with optional arguments
	re := regexp.MustCompile(`@deprecated(?:\([^)]*\))?`)
	
	return re.ReplaceAllStringFunc(description, func(match string) string {
		// Check if the match is already surrounded by backticks by examining the context
		matchIndex := strings.Index(description, match)
		if matchIndex > 0 && description[matchIndex-1] == '`' {
			// Check if there's a closing backtick after the match
			endIndex := matchIndex + len(match)
			if endIndex < len(description) && description[endIndex] == '`' {
				return match // Already enclosed in backticks
			}
		}
		
		// Not already in backticks, so wrap it
		return "`" + match + "`"
	})
}

// convertMarkdownCodeBlocks converts markdown code blocks (```lang) to AsciiDoc format ([source,lang] ----)
func convertMarkdownCodeBlocks(description string) string {
	// Regex to match markdown code blocks: ```language\ncontent\n```
	// Supports optional language specification
	re := regexp.MustCompile("(?s)```(\\w*)\n(.*?)\n```")
	
	return re.ReplaceAllStringFunc(description, func(match string) string {
		// Extract language and content from the match
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match // Return original if parsing fails
		}
		
		language := submatches[1]
		content := submatches[2]
		
		// Default to generic source block if no language specified
		if language == "" {
			language = "text"
		}
		
		// Convert to AsciiDoc format
		return fmt.Sprintf("[source,%s]\n----\n%s\n----", language, content)
	})
}

// extractChangelog extracts version annotations and formats them as AsciiDoc changelog
func extractChangelog(description string) string {
	// Regex to match version annotations: action.version: version_number
	re := regexp.MustCompile(`(?m)^\s*(add|update|deprecated|removed)\.version:\s*(.+)$`)
	matches := re.FindAllStringSubmatch(description, -1)
	
	if len(matches) == 0 {
		return ""
	}
	
	// Group versions by action type
	changelog := map[string][]string{
		"add":        {},
		"update":     {},
		"deprecated": {},
		"removed":    {},
	}
	
	for _, match := range matches {
		if len(match) >= 3 {
			action := match[1]
			version := strings.TrimSpace(match[2])
			if _, exists := changelog[action]; exists {
				changelog[action] = append(changelog[action], version)
			}
		}
	}
	
	// Build AsciiDoc changelog
	var changelogBuilder strings.Builder
	changelogBuilder.WriteString("\n.Changelog\n")
	
	// Order: add, update, deprecated, removed
	actions := []string{"add", "update", "deprecated", "removed"}
	for _, action := range actions {
		versions := changelog[action]
		if len(versions) > 0 {
			if len(versions) == 1 {
				changelogBuilder.WriteString(fmt.Sprintf("* %s: %s\n", action, versions[0]))
			} else {
				changelogBuilder.WriteString(fmt.Sprintf("* %s: %s\n", action, strings.Join(versions, ", ")))
			}
		}
	}
	
	return changelogBuilder.String()
}

// processDescriptionWithChangelog processes description and extracts changelog separately
func processDescriptionWithChangelog(description string) (processedDesc, changelog string) {
	// Extract changelog first
	changelog = extractChangelog(description)
	
	// Remove version annotations from description for regular processing
	versionRe := regexp.MustCompile(`(?m)^\s*(add|update|deprecated|removed)\.version:\s*.+$\n?`)
	cleanedDesc := versionRe.ReplaceAllString(description, "")
	
	// Process the cleaned description normally
	processedDesc = processDescription(cleanedDesc)
	
	return processedDesc, changelog
}

/**
 * Print the enumeration details
 */
func printEnumsTmpl(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) {
	var enumInfos []EnumInfo

	for _, t := range sortedDefs {
		if t.Kind == ast.Enum {
			valuesTableString := getEnumValuesTableString(t)

			enumInfo := EnumInfo{
				Name:        t.Name,
				AnchorName:  camelToSnake(t.Name), // Anchor name for enum
				Description: t.Description,
				ValuesTable: valuesTableString,
			}
			enumInfos = append(enumInfos, enumInfo)
		}
	}

	data := EnumSectionData{
		EnumsTag: ENUM_TAG, // Use existing constant
		Enums:    enumInfos,
	}

	funcMap := template.FuncMap{
		"printAsciiDocTagsTmpl": printAsciiDocTagsTmpl,
	}

	tmpl, err := template.New("enumSectionTemplate").Funcs(funcMap).Parse(enumSectionTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing enum section template: %v\n", err)
		return
	}

	err = tmpl.Execute(os.Stdout, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing enum section template: %v\n", err)
	}
}

func printInputsTmpl(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) {
	var inputInfos []InputInfo

	for _, t := range sortedDefs {
		if t.Kind == ast.InputObject {

			fieldsTableString, err := getInputFieldsTableString(t, definitionsMap)
			if err != nil {
				// Log error but potentially continue?
				fmt.Fprintf(os.Stderr, "Error generating fields table for input %s: %v\n", t.Name, err)
				fieldsTableString = "[ERROR generating fields table]"
			}

			// Process input description and extract changelog
			processedDesc, changelog := processDescriptionWithChangelog(t.Description)
			
			inputInfo := InputInfo{
				Name:        t.Name,
				AnchorName:  "input_" + camelToSnake(t.Name), // Use input_ prefix
				Description: processedDesc,
				FieldsTable: fieldsTableString,
				Changelog:   changelog,
			}
			inputInfos = append(inputInfos, inputInfo)
		}
	}

	data := InputSectionData{
		InputsTag: INPUT_TAG, // Use existing constant
		Inputs:    inputInfos,
	}

	funcMap := template.FuncMap{
		"printAsciiDocTagsTmpl": printAsciiDocTagsTmpl,
	}

	tmpl, err := template.New("inputSectionTemplate").Funcs(funcMap).Parse(inputSectionTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input section template: %v\n", err)
		return
	}

	err = tmpl.Execute(os.Stdout, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing input section template: %v\n", err)
	}
}

func getInputFieldsTableString(t *ast.Definition, definitionsMap map[string]*ast.Definition) (string, error) {
	if len(t.Fields) == 0 {
		return "", nil // No fields
	}

	var builder strings.Builder

	// Use a local template instance for fields
	fieldTmpl, err := template.New("field").Funcs(template.FuncMap{
		"processDescription": processDescription,
	}).Parse(fieldTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse field template for input %s: %w", t.Name, err)
	}

	// Add tags and headers specific to inputs
	// Note: Replicating tags from printObjectFields logic for Inputs
	builder.WriteString(fmt.Sprintf(ADOC_INPUT_DEF_START_TAG, t.Name))
	builder.WriteString(fmt.Sprintf("[[input_%s]]\n", strings.ToLower(t.Name)))
	builder.WriteString(fmt.Sprintf(".input: %s\n", t.Name))

	// Use TABLE_OPTIONS_4 for inputs as per printObjectFields
	builder.WriteString(TABLE_OPTIONS_3 + "\n")
	builder.WriteString(TABLE_SE + "\n")
	//builder.WriteString("| Type | Field | Description | Directives\n") // Header for Input Objects
	builder.WriteString("| Type | Field | Description\n") // Header for Input Objects

	for _, f := range t.Fields {
		typeName := processTypeName(f.Type.String(), definitionsMap)
		directives := getDirectivesStringTpl(f.Directives) // Using Tpl version for directives
		
		// Process description and extract changelog for input fields too
		processedDesc, changelog := processDescriptionWithChangelog(f.Description)

		data := FieldData{
			Type:            typeName,
			Name:            f.Name,
			Description:     processedDesc, // Use processed description
			RequiredOrArray: strings.Contains(typeName, "!") || strings.Contains(typeName, "["),
			Required:        isRequiredTypeTpl(typeName),
			IsArray:         strings.Contains(typeName, "["),
			Directives:      directives, // Pass processed directives
			Changelog:       changelog,
		}

		// Execute the field template into the builder
		err := fieldTmpl.Execute(&builder, data)
		if err != nil {
			// Attempt to continue, maybe log error?
			builder.WriteString(fmt.Sprintf("| ERROR executing template for field %s | | |\n", f.Name))
			// return "", fmt.Errorf("failed to execute field template for input field %s: %w", f.Name, err)
		} else {
			builder.WriteString("\n") // Ensure newline after each row if template executed successfully
		}
	}

	builder.WriteString(TABLE_SE + "\n")
	builder.WriteString(fmt.Sprintf(ADOC_INPUT_DEF_END_TAG, t.Name))

	return builder.String(), nil
}

/**
 * Print the query details
 */
func processQueryOld(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) {
	fmt.Println("== Query")
	for _, t := range sortedDefs {
		if t.Kind == ast.Object && t.Name == "Query" {
			fmt.Printf(CROSS_REF, t.Name) // Add anchor
			//fmt.Printf("=== %s\n\n", t.Name)

			if t.Description != "" {
				printAsciiDocTags(t.Description)
			}

			printQuery(t, definitionsMap)

			fmt.Println()
		}
	}
}

/**
 * Print the query details
 */
func printQueries(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) {
	fmt.Println("== Query")
	fmt.Println()

	for _, t := range sortedDefs {
		if t.Kind == ast.Object && t.Name == "Query" {
			//fmt.Printf(CROSS_REF, camelToSnake(t.Name)) // Add anchor
			fmt.Println()

			if t.Description != "" {
				printAsciiDocTags(t.Description)
			}
			fmt.Println()

			printQueryDetails(t, definitionsMap)

			fmt.Println()
		}
	}
}

func printObjectFieldsTpl(data FieldData) {

	//func printObjectFieldsTpl(t *ast.Definition, definitionsMap map[string]*ast.Definition) {
	//tmpl := template.Must(template.New("field").Parse(fieldTemplate))

	tmpl := template.Must(template.New("field").Funcs(template.FuncMap{
		"processDescription": processDescription,
	}).Parse(fieldTemplate))

	//	for _, f := range t.Fields {
	//		typeName := processTypeName(f.Type.String(), definitionsMap)
	//		data := FieldData{
	//			Type:            typeName,
	//			Name:            f.Name,
	//			Description:     f.Description,
	//			RequiredOrArray: strings.Contains(typeName, "!") || strings.Contains(typeName, "["),
	//			Required:        isRequiredTypeTpl(typeName),
	//			IsArray:         strings.Contains(typeName, "["),
	//			Directives:      getDirectivesStringTpl(f.Directives),
	//		}
	tmpl.Execute(os.Stdout, data)
	// }
}

func getRequiredString(typeName string) string {
	switch {
	case strings.Contains(typeName, "]!"):
		return "True (more than one)"
	case strings.Contains(typeName, "!]"):
		return "True (at least one, if provided)"
	case strings.Contains(typeName, "!"):
		return "True"
	default:
		return ""
	}
}

/**
 * Print the objects to a table.
 */
func printObjectFields(t *ast.Definition, definitionsMap map[string]*ast.Definition) {
	if len(t.Fields) > 0 {
		if t.Name != "Query" {
			if t.IsInputType() {
				fmt.Printf(ADOC_INPUT_DEF_START_TAG, t.Name)          // Add type tag to table
				fmt.Printf("[[input_%s]]\n", strings.ToLower(t.Name)) // Add input tag to table
				fmt.Printf(".input: %s\n", t.Name)                    // Add input header to table
			} else {
				fmt.Printf(ADOC_TYPE_DEF_START_TAG, t.Name)          // Add type tag to table
				fmt.Printf("[[type-%s]]\n", strings.ToLower(t.Name)) // Add type tag to table
				fmt.Printf(".type: %s\n", t.Name)                    // Add type header to table
			}
		}
		if t.Name == "Query" {

			fmt.Println(TABLE_OPTIONS_3)
			fmt.Println(TABLE_SE)
			fmt.Println("| Return | Function | Description")
		} else if t.Name == "InputObject" {
			fmt.Println(TABLE_OPTIONS_4)
			fmt.Println(TABLE_SE)
			//fmt.Println("| Type | Field | Description")
			fmt.Println("| Type | Field | Description | Directives")
		} else {
			fmt.Println(TABLE_OPTIONS_3)
			fmt.Println(TABLE_SE)
			fmt.Println("| Type | Field | Description ")
		}

		for _, f := range t.Fields {
			typeName := f.Type.String()
			typeName = processTypeName(typeName, definitionsMap)
			directives := getDirectivesString(f.Directives)

			if t.Name == "Query" {
				// if it is the query type
				fmt.Printf("| %s | %s | %s\n", typeName, f.Name, processDescription(f.Description))
			} else if t.Name == "InputObject" {
				fmt.Printf("| %s | %s | %s | %s\n", typeName, f.Name, f.Description, prependStringIfNotEmpty(directives, "\n"))
			} else {

				fmt.Println()

				data := FieldData{
					Type:            typeName,
					Name:            f.Name,
					Description:     f.Description,
					RequiredOrArray: strings.Contains(typeName, "!") || strings.Contains(typeName, "["),
					Required:        isRequiredTypeTpl(typeName),
					IsArray:         strings.Contains(typeName, "["),
					Directives:      getDirectivesStringTpl(f.Directives),
				}
				printObjectFieldsTpl(data)

				// if it is not the query type
				//fmt.Printf("| %s | %s | %s%s%s%s%s\n", typeName, f.Name, addStringIfNotEmpty(processDescription(f.Description), "\n"),
				//	prependStringIfNotEmpty(isRequiredOrArrayType(typeName), "\n"),
				//	prependStringIfNotEmpty(isRequiredType(typeName), "\n"),
				//	isArrayType(typeName),
				//	prependStringIfNotEmpty(directives, "\n"))
			}
		}

		fmt.Println(TABLE_SE)

		if t.IsInputType() {
			fmt.Printf(ADOC_INPUT_DEF_END_TAG, t.Name) // Add input tag to table
		} else {
			fmt.Printf(ADOC_TYPE_DEF_END_TAG, t.Name) // Add type tag to table
		}
	}
}

/*

func processDescription(description string) string {
	// Replace * and - with newline followed by the character
	processed := strings.ReplaceAll(description, "*", "\n*")
	processed = strings.ReplaceAll(processed, "-", "\n-")
	// Remove any double newlines that might have been created
	//processed = strings.ReplaceAll(processed, "\n\n", "\n")
	// Remove newline at start if present
	return strings.TrimPrefix(processed, "\n")
}
*/

func addStringIfNotEmpty(s string, add string) string {
	if s != "" {
		return s + add
	}
	return s
}

func prependStringIfNotEmpty(s string, add string) string {
	if s != "" {
		return add + s
	}
	return s
}

func addPlusIfNotEmpty(s string) string {
	if s != "" {
		return s + " +"
	}
	return s
}

func addPlusIfBothNotEmpty(s1 string, s2 string) string {
	if s1 != "" && s2 != "" {
		return s1 + ""
	}
	return s1
}

func addStringIfBothNotEmpty(s1 string, s2 string, add string) string {
	if s1 != "" && s2 != "" {
		return s1 + add
	}
	return s1
}

/**
 * Print enum values in a table
 */
func printEnumValues(t *ast.Definition) {
	if len(t.EnumValues) > 0 {
		fmt.Printf(ADOC_ENUM_START_TAG, t.Name)
		fmt.Printf("[[enum_%s]]\n", camelToSnake(t.Name))
		fmt.Printf(".enum_%s\n", camelToSnake(t.Name))
		fmt.Println(TABLE_OPTIONS_2)
		fmt.Println(TABLE_SE)
		fmt.Println("| Value | Description")
		for _, v := range t.EnumValues {
			fmt.Printf("| %s | %s\n", v.Name, v.Description)
		}
		fmt.Println(TABLE_SE)
		fmt.Printf(ADOC_ENUM_END_TAG, t.Name)
	}
}

/**
 * print asciidoc tags.
 */
/*func printAsciiDocTags(description string) {
	lines := strings.Split(description, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# tag::") || strings.HasPrefix(trimmed, "# end::") {
			fmt.Println("//" + trimmed[1:]) // remove the '#' as AsciiDoc comments start with '//' not '#'
		} else if !strings.HasPrefix(trimmed, "# ") {
			// If it is not a comment, then print the line.
			fmt.Println(trimmed)
		}
	}
}
*/
func printAsciiDocTags(description string) {
	re := regexp.MustCompile(`^#\s*(tag::|end::)(.*)`)
	lines := strings.Split(description, "\n")
	for _, line := range lines {
		if matches := re.FindStringSubmatch(line); matches != nil {
			fmt.Println("//" + matches[1] + matches[2])
		} else {
			fmt.Println(line)
		}
	}
}

func printAsciiDocTagsTmpl(description string) string {
	re := regexp.MustCompile(`^#\s*(tag::|end::)(.*)`)
	lines := strings.Split(description, "\n")
	var result strings.Builder
	for _, line := range lines {
		if matches := re.FindStringSubmatch(line); matches != nil {
			result.WriteString("//" + matches[1] + matches[2] + "\n")
		} else {
			result.WriteString(line + "\n")
		}
	}
	return result.String()
}

/**
 * Print query as a table
 */
func printQuery(t *ast.Definition, definitionsMap map[string]*ast.Definition) {
	if len(t.Fields) > 0 {
		fmt.Println(TABLE_OPTIONS_4)
		fmt.Println(TABLE_SE)
		if t.Name == "Query" {
			fmt.Println("| Return | Function | Description | params")
		} else {
			fmt.Println("| Type | Field | Description | params")
		}

		for _, f := range t.Fields {
			typeName := f.Type.String()
			typeName = processTypeName(typeName, definitionsMap)
			if t.Name == "Query" {
				fmt.Println("| ", typeName)
				fmt.Println("| ", f.Name)
				fmt.Println("| ")
				fmt.Println(f.Description)
				fmt.Println("|", getArgsString(f.Arguments))
				fmt.Println(" ")
				fmt.Println(" ")
			} else {
				// Replace "{" with a space
				stringWithoutBraces := strings.Replace(getArgsString(f.Arguments), "{", " ", -1)
				// Replace "}" with a space
				stringWithoutBraces = strings.Replace(stringWithoutBraces, "}", " ", -1)
				fmt.Printf("| %s | %s | %s | %s \n", typeName, f.Name, f.Description, stringWithoutBraces)
			}
		}
		fmt.Printf("%s", TABLE_SE)
	}
}

/**
 * Get the arguments for reconstructing the method signature
 */
func getArgsString(args ast.ArgumentDefinitionList) string {
	var argsStrings []string

	for _, arg := range args {
		argString := fmt.Sprintf("* `%s : %s", arg.Name, arg.Type.String())
		if arg.DefaultValue != nil {
			argString += fmt.Sprintf(" (Default:%s)`", arg.DefaultValue.String())
		} else {
			argString += "`"
		}

		argsStrings = append(argsStrings, argString)
	}
	return strings.Join(argsStrings, " + \n")
}

/**
 * Get the type arguments.
 */
func getArgsMethodTypeString(args ast.ArgumentDefinitionList) (string, int) {
	var argsStrings []string
	var counter = 0
	for _, arg := range args {
		counter++
		argString := fmt.Sprintf("  %s: %s", arg.Name, arg.Type.String())
		if arg.DefaultValue != nil {
			argString += fmt.Sprintf(" = %s", arg.DefaultValue.String())
		}
		// if there is another argument, add a comma
		if arg != args[len(args)-1] {
			argString += " ,"
		}
		if includeAdocLineTags {
			argString += fmt.Sprintf(" <%d> ", counter)
		}

		argString += "\n"
		argsStrings = append(argsStrings, argString)
	}

	return strings.Join(argsStrings, ""), counter
}

/**
 * Print query details
 */
func printQueryDetails(t *ast.Definition, definitionsMap map[string]*ast.Definition) {
	if len(t.Fields) > 0 {
		for _, f := range t.Fields {
			// Check if the query is internal
			isInternal := strings.Contains(f.Description, "INTERNAL")

			// Skip internal queries if excludeInternal is true
			if *excludeInternal && isInternal {
				continue
			}

			if f.Directives.ForName("deprecated") != nil {
				// Handle deprecated field
				//continue // or mark it as deprecated in the documentation

			}

			fmt.Printf(ADOC_QUERY_START_TAG, f.Name)
			fmt.Println()
			fmt.Printf("[[query_%s]]\n", strings.ToLower(f.Name))

			// if f.Description contains INTERNAL then add teh word INTERNAL to the tag
			if strings.Contains(f.Description, "INTERNAL") {
				fmt.Printf(L3_TAG, f.Name+" [INTERNAL]")
			} else {
				fmt.Printf(L3_TAG, f.Name)
			}
			fmt.Println()
			// Process description and extract changelog for queries
			processedDesc, changelog := processDescriptionWithChangelog(f.Description)
			
			if includeAdocLineTags {
				fmt.Printf(ADOC_METHOD_DESC_START_TAG, f.Name)
				fmt.Println(cleanDescription(processedDesc, "-"))
				fmt.Printf(ADOC_METHOD_DESC_END_TAG, f.Name)
				fmt.Println()
			}
			fmt.Printf(ADOC_METHOD_SIG_START_TAG, f.Name)
			fmt.Printf(".query: %s\n", f.Name)
			fmt.Print(SOURCE_HEAD)
			fmt.Println("----")
			argsString, counter := getArgsMethodTypeString(f.Arguments)
			if includeAdocLineTags {
				counter++
				fmt.Printf("%s(\n%s): %s <%d>\n", f.Name, argsString, f.Type.String(), counter)
			} else {
				fmt.Printf("%s(\n%s): %s\n", f.Name, argsString, f.Type.String())
			}
			fmt.Println("----")
			fmt.Printf(ADOC_METHOD_SIG_END_TAG, f.Name)
			fmt.Println()
			fmt.Printf(ADOC_METHOD_ARGS_START_TAG, f.Name)
			if includeAdocLineTags {
				fmt.Println(convertDescriptionToRefNumbers(processedDesc, true))
			} else {
				fmt.Println(processedDesc)
			}
			fmt.Printf(ADOC_METHOD_ARGS_END_TAG, f.Name)
			fmt.Println()

			typeName := f.Type.String()

			typeName = processTypeName(typeName, definitionsMap)
			fmt.Printf("// tag::query-name-%s[]\n", f.Name)
			fmt.Printf("*Query Name:* _%s_\n", f.Name)
			fmt.Printf("// end::query-name-%s[]\n", f.Name)
			fmt.Println()
			fmt.Printf("// tag::query-return-%s[]\n", f.Name)
			fmt.Printf("*Return:* %s\n", typeName)
			fmt.Printf("// end::query-return-%s[]\n", f.Name)
			fmt.Println()

			if len(f.Arguments) > 0 {
				fmt.Printf("// tag::arguments-%s[]\n", f.Name)
				fmt.Printf(".Arguments\n")
				// Replace "{" with a space
				stringWithoutBraces := strings.Replace(getArgsString(f.Arguments), "{", " ", -1)
				// Replace "}" with a space
				stringWithoutBraces = strings.Replace(stringWithoutBraces, "}", " ", -1)
				fmt.Println(stringWithoutBraces)
				fmt.Printf("// end::arguments-%s[]\n", f.Name)
				fmt.Println()
			}
			
			// Add changelog section for queries
			if changelog != "" {
				fmt.Printf("// tag::query-changelog-%s[]\n", f.Name)
				fmt.Print(changelog)
				fmt.Printf("// end::query-changelog-%s[]\n", f.Name)
				fmt.Println()
			}
			
			fmt.Printf(ADOC_QUERY_END_TAG, f.Name)
			fmt.Println()
		}
	}
}

func isRequiredOrArrayType(typeName string) string {
	//if strings.Contains(typeName, "!") || strings.Contains(typeName, "[") || strings.Contains(typeName, "]") {
	//	return "\n\n.Notes\n"
	//}
	return ""
}

func isRequiredType(typeName string) string {
	if strings.Contains(typeName, "]!") {
		return "\n.Required:\n* `True` (more than one) "
	} else {
		if strings.Contains(typeName, "!]") {
			return "\n.Required:\n* `True` (at least one, if provided) "
		} else {
			if strings.Contains(typeName, "!") {
				return "\n.Required:\n* `True` "
			} else {
				return ""
			}
		}
	}
}

func isRequiredTypeTpl(typeName string) string {
	if strings.Contains(typeName, "]!") {
		return "`True` (more than one) "
	} else {
		if strings.Contains(typeName, "!]") {
			return "`True` (at least one, if provided) "
		} else {
			if strings.Contains(typeName, "!") {
				return "`True` "
			} else {
				return ""
			}
		}
	}
}

func isArrayType(typeName string) string {
	if strings.Contains(typeName, "[") && strings.Contains(typeName, "]") {
		return "\n.Array:\n* `True` "
	} else {
		return ""
	}
}
func isArrayTypeTpl(typeName string) string {
	if strings.Contains(typeName, "[") && strings.Contains(typeName, "]") {
		return "* `True` "
	} else {
		return ""
	}
}

func hasDirectives(typeName string) string {
	if strings.Contains(typeName, "@") {
		// search between the @ symbol and the first space or new line

		// Find the index of the @ symbol
		//atIndex := strings.Index(typeName, "@")
		// Find the index of the first space after the @ symbol
		//spaceIndex := strings.Index(typeName[atIndex:], " ")
		// Find the index of the first new line after the @ symbol
		//newLineIndex := strings.Index(typeName[atIndex:], "\n")

		return "\n* `Directives: True` "
		//+ "+ \n* `Directivetype: `" + typeName[atIndex:atIndex+newLineIndex] +'`'

	} else {
		return ""
	}
}

/**
 * Process the type name, adding the asciidoc links if needed.
 */
func processTypeName(typeName string, definitionsMap map[string]*ast.Definition) string {
	// Assuming typeName may be a list type like "[TypeName]"
	// Trim the square brackets if they exist to extract the actual type name
	trimmedTypeName := strings.Trim(typeName, "[]")

	if definitionsMap[trimmedTypeName] != nil && definitionsMap[trimmedTypeName].Kind == ast.Object {
		// If it's a list type, add back the square brackets
		if strings.HasPrefix(typeName, "[") && strings.HasSuffix(typeName, "]") {
			typeName = fmt.Sprintf("[<<%s,`%s`>>]", trimmedTypeName, trimmedTypeName)
		} else {
			typeName = fmt.Sprintf("<<%s,`%s`>>", trimmedTypeName, trimmedTypeName)
		}
	} else {
		// If it's a type, and it is greater than 0, but is is not a special type, then just wrap it.
		if len(typeName) > 0 {
			typeName = fmt.Sprintf("`%s`", typeName)
		}
	}

	return typeName
}

func camelToSnake(s string) string {
	var result strings.Builder
	for i, v := range s {
		if unicode.IsUpper(v) {
			if i != 0 {
				result.WriteByte('_')
			}
			result.WriteRune(unicode.ToLower(v))
		} else {
			result.WriteRune(v)
		}
	}
	return result.String()
}

/*
A function to convert the text

- <optional-filter> `studentCode`: String - Single or multiple student ids or student codes.
- <optional-filter> `subjectCode`: String - Single or multiple subject codes.

to the text
<1> <optional-filter> `studentCode`: String - Single or multiple student ids or student codes.
<2> <optional-filter> `subjectCode`: String - Single or multiple subject codes.
*/
/*func convertDescriptionToRefNumbers(text string, skipNonDash bool) string {
	lines := strings.Split(text, "\n")
	var result strings.Builder

	refNum := 1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") {
			// If the line starts with a hyphen, then add the counter and the line
			// not including the - in the line
			result.WriteString(fmt.Sprintf("<%d> %s\n", refNum, line[1:]))
			refNum++
		} else {
			if !skipNonDash {
				// If the line does not start with a hyphen, then add the line
				result.WriteString(line + "\n")
			}
		}
		i++
	}
	return result.String()
}
*/
func convertDescriptionToRefNumbers(text string, skipNonDash bool) string {
	lines := strings.Split(text, "\n")
	var result strings.Builder
	refNum := 1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// ignore if the string starts with --
		if strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "--") {
			lineContent := strings.TrimSpace(trimmed[1:])
			result.WriteString(fmt.Sprintf("<%d> %s\n", refNum, lineContent))
			refNum++
		} else if !skipNonDash {
			result.WriteString(line + "\n")
		}
	}
	return result.String()
}

/*
A function to convert the text

# My fancy description

- <optional-filter> `studentCode`: String - Single or multiple student ids or student codes.
- <optional-filter> `subjectCode`: String - Single or multiple subject codes.

to the text

My fancy description
*/
func cleanDescription(text string, skipCharacter string) string {
	lines := strings.Split(text, "\n")
	var result strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Don't skip AsciiDoc code block delimiters (----) even if skipCharacter is "-"
		if !strings.HasPrefix(trimmed, skipCharacter) || trimmed == "----" {
			result.WriteString(line + "\n")
		}
	}
	return result.String()
}

func printAsciiDocTag(startTag, content, endTag string) {
	fmt.Printf(startTag)
	fmt.Println(content)
	fmt.Printf(endTag)
}

func getTypeName(t *ast.Type) string {
	if t.Elem != nil {
		return "[" + getTypeName(t.Elem) + "]"
	}
	return t.NamedType
}

func getDirectivesStringTpl(directives ast.DirectiveList) string {
	if len(directives) == 0 {
		return ""
	}

	var directiveStrings []string
	for _, dir := range directives {
		dirStr := fmt.Sprintf("@%s", dir.Name)

		if len(dir.Arguments) > 0 {
			args := make([]string, 0, len(dir.Arguments))
			for _, arg := range dir.Arguments {
				args = append(args, fmt.Sprintf("%s: %s", arg.Name, arg.Value.String()))
			}
			dirStr += fmt.Sprintf("(%s)", strings.Join(args, ", "))
		}

		directiveStrings = append(directiveStrings, ("`" + dirStr + "`"))
	}

	return "\n* " + strings.Join(directiveStrings, "\n")
}

func getDirectivesString(directives ast.DirectiveList) string {
	if len(directives) == 0 {
		return ""
	}

	var directiveStrings []string
	for _, dir := range directives {
		dirStr := fmt.Sprintf("@%s", dir.Name)

		if len(dir.Arguments) > 0 {
			args := make([]string, 0, len(dir.Arguments))
			for _, arg := range dir.Arguments {
				args = append(args, fmt.Sprintf("%s: %s", arg.Name, arg.Value.String()))
			}
			dirStr += fmt.Sprintf("(%s)", strings.Join(args, ", "))
		}

		directiveStrings = append(directiveStrings, ("`" + dirStr + "`"))
	}

	return "\n.Directives:\n* " + strings.Join(directiveStrings, "\n")
}

func printDirectives(doc *ast.SchemaDocument) {
	if len(doc.Directives) == 0 {
		return
	}

	fmt.Print(DIRECTIVES_TAG)
	fmt.Println()
	fmt.Println(TABLE_OPTIONS_3)
	fmt.Println(TABLE_SE)
	fmt.Println("| Directive | Arguments | Description")

	for _, dir := range doc.Directives {
		args := make([]string, 0, len(dir.Arguments))
		for _, arg := range dir.Arguments {
			args = append(args, fmt.Sprintf("%s: %s", arg.Name, arg.Type.String()))
		}

		fmt.Printf("| @%s | %s | %s\n",
			dir.Name,
			strings.Join(args, ", "),
			dir.Description)
	}

	fmt.Println(TABLE_SE)
	fmt.Println()
}

// Template version of printing scalar information.
func printScalarsTmpl(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) {
	var scalars []Scalar
	for _, t := range sortedDefs {
		if t.Kind == ast.Scalar {
			scalars = append(scalars, Scalar{
				Name:        t.Name,
				Description: t.Description,
				CrossRef:    fmt.Sprintf(CROSS_REF, camelToSnake(t.Name)),
				L3Tag:       fmt.Sprintf(L3_TAG, t.Name),
			})
		}
	}

	data := ScalarData{
		ScalarTag:    SCALAR_TAG,
		FoundScalars: len(scalars) > 0,
		Scalars:      scalars,
	}

	tmpl, err := template.New("scalarTemplate").Funcs(template.FuncMap{
		"printAsciiDocTagsTmpl": printAsciiDocTagsTmpl,
	}).Parse(scalarTemplate)
	if err != nil {
		fmt.Println("Error parsing template:", err)
		return
	}

	err = tmpl.Execute(os.Stdout, data)
	if err != nil {
		fmt.Println("Error executing template:", err)
	}
}

func printSubscriptionsTmpl(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) {
	var subscriptions []Subscription
	foundSubscriptions := false

	for _, t := range sortedDefs {
		if t.Kind == ast.Object && t.Name == "Subscription" {
			foundSubscriptions = true

			var description string
			if t.Description != "" {
				description = printAsciiDocTagsTmpl(t.Description)
			}

			details := getSubscriptionDetailsTmpl(t, definitionsMap)

			subscriptions = append(subscriptions, Subscription{
				Description: description,
				Details:     details,
			})
		}
	}

	data := SubscriptionData{
		FoundSubscriptions: foundSubscriptions,
		Subscriptions:      subscriptions,
	}

	tmpl, err := template.New("subscriptionTemplate").Funcs(template.FuncMap{
		"printAsciiDocTagsTmpl": printAsciiDocTagsTmpl,
	}).Parse(subscriptionTemplate)
	if err != nil {
		fmt.Println("Error parsing template:", err)
		return
	}

	err = tmpl.Execute(os.Stdout, data)
	if err != nil {
		fmt.Println("Error executing template:", err)
	}
}

func getSubscriptionDetailsTmpl(t *ast.Definition, definitionsMap map[string]*ast.Definition) string {
	var details strings.Builder

	if len(t.Fields) > 0 {
		for _, f := range t.Fields {
			// Check if the subscription is internal
			isInternal := strings.Contains(f.Description, "INTERNAL")

			// Skip internal subscriptions if excludeInternal is true
			if *excludeInternal && isInternal {
				continue
			}

			if f.Directives.ForName("deprecated") != nil {
				details.WriteString(fmt.Sprintf("// tag::deprecated-%s[]\n", f.Name))
				details.WriteString(fmt.Sprintf(".Deprecated: %s\n", f.Name))
				details.WriteString(fmt.Sprintf("// end::deprecated-%s[]\n", f.Name))
				details.WriteString("\n")
			}

			details.WriteString(fmt.Sprintf("=== %s\n", f.Name))
			details.WriteString("\n")

			if f.Description != "" {
				details.WriteString(printAsciiDocTagsTmpl(f.Description))
				details.WriteString("\n")
			}

			details.WriteString(fmt.Sprintf(".subscription: %s\n", f.Name))
			details.WriteString(SOURCE_HEAD)
			details.WriteString("----\n")
			argsString, counter := getArgsMethodTypeString(f.Arguments)
			if includeAdocLineTags {
				counter++
				details.WriteString(fmt.Sprintf("%s(\n%s): %s <%d>\n", f.Name, argsString, f.Type.String(), counter))
			} else {
				details.WriteString(fmt.Sprintf("%s(\n%s): %s\n", f.Name, argsString, f.Type.String()))
			}
			details.WriteString("----\n")
			details.WriteString("\n")

			if len(f.Directives) > 0 {
				details.WriteString(fmt.Sprintf("// tag::subscription-directives-%s[]\n", f.Name))
				details.WriteString(".Directives\n")
				details.WriteString(getDirectivesString(f.Directives) + "\n")
				details.WriteString(fmt.Sprintf("// end::subscription-directives-%s[]\n", f.Name))
				details.WriteString("\n")
			}

			details.WriteString(fmt.Sprintf(ADOC_QUERY_END_TAG, f.Name))
			details.WriteString("\n")
		}
	}

	return details.String()
}

func printMutationsTmpl(sortedDefs []*ast.Definition, definitionsMap map[string]*ast.Definition) {
	var mutations []Mutation
	var mutationObjectDescription string
	foundMutations := false
	mutationDefinition := (*ast.Definition)(nil) // Find the Mutation object definition

	for _, t := range sortedDefs {
		if t.Kind == ast.Object && t.Name == "Mutation" {
			mutationDefinition = t
			if t.Description != "" {
				// Store the description for the template
				mutationObjectDescription = t.Description
			}
			break // Found the Mutation object
		}
	}

	if mutationDefinition != nil {
		for _, f := range mutationDefinition.Fields {
			// Check if the mutation is internal
			isInternal := strings.Contains(f.Description, "INTERNAL")

			// Skip internal mutations if excludeInternal is true
			if *excludeInternal && isInternal {
				continue
			}

			// Skip deprecated mutations
			if f.Directives.ForName("deprecated") != nil {
				continue
			}

			foundMutations = true // We found at least one non-skipped mutation

			typeName := processTypeName(f.Type.String(), definitionsMap)
			arguments := ""
			if len(f.Arguments) > 0 {
				arguments = getArgsString(f.Arguments)
			}
			directives := ""
			if len(f.Directives) > 0 {
				directives = getDirectivesString(f.Directives)
			}
			// Process description and extract changelog
			processedDesc, changelog := processDescriptionWithChangelog(f.Description)
			cleanedDescription := cleanDescription(processedDesc, "-")

			// --- Construct Method Signature Block ---
			var signatureBlock strings.Builder
			signatureBlock.WriteString(fmt.Sprintf("// tag::method-signature-%s[]\n", f.Name)) // Re-add tag here for consistency if needed, or rely on outer one
			signatureBlock.WriteString(fmt.Sprintf(".mutation: %s\n", f.Name))
			signatureBlock.WriteString(SOURCE_HEAD)
			signatureBlock.WriteString("----\n")
			argsString, counter := getArgsMethodTypeString(f.Arguments)
			// Always use line tags for consistency with printMutationDetails
			counter++
			signatureBlock.WriteString(fmt.Sprintf("%s(\n%s): %s <%d>\n", f.Name, argsString, f.Type.String(), counter))
			signatureBlock.WriteString("----\n")
			signatureBlock.WriteString(fmt.Sprintf("// end::method-signature-%s[]\n", f.Name)) // Re-add tag here for consistency
			// --- End Method Signature Block ---

			mutations = append(mutations, Mutation{
				Name:                 f.Name,
				Description:          processedDesc,      // Pass processed description for convertDescriptionToRefNumbers
				CleanedDescription:   cleanedDescription, // Pass cleaned description
				TypeName:             typeName,
				Arguments:            arguments,
				Directives:           directives,
				HasArguments:         len(f.Arguments) > 0,
				HasDirectives:        len(f.Directives) > 0,
				AnchorName:           "mutation_" + strings.ToLower(f.Name), // Generate anchor name
				IsInternal:           isInternal,                            // Pass internal flag
				MethodSignatureBlock: signatureBlock.String(),               // Pass signature block
				Changelog:            changelog,                             // Pass changelog
			})
		}
	}

	data := MutationData{
		Mutations:                 mutations,
		MutationObjectDescription: mutationObjectDescription, // Pass object description
		FoundMutations:            foundMutations,            // Pass flag
		MutationTag:               "== Mutations",            // Pass header tag
	}

	// Add printAsciiDocTagsTmpl to FuncMap
	funcMap := template.FuncMap{
		"convertDescriptionToRefNumbers": convertDescriptionToRefNumbers,
		"printAsciiDocTagsTmpl":          printAsciiDocTagsTmpl,
	}

	tmpl, err := template.New("mutationTemplate").Funcs(funcMap).Parse(mutationTemplate)
	if err != nil {
		fmt.Println("Error parsing mutation template:", err)
		return
	}

	err = tmpl.Execute(os.Stdout, data)
	if err != nil {
		fmt.Println("Error executing mutation template:", err)
	}
}

// Add this helper function
func getTypeFieldsTableString(t *ast.Definition, definitionsMap map[string]*ast.Definition) (string, error) {
	var builder strings.Builder

	if len(t.Fields) == 0 {
		return "", nil // No fields, return empty string
	}

	// Use a local template instance for fields
	fieldTmpl, err := template.New("field").Funcs(template.FuncMap{
		"processDescription": processDescription,
	}).Parse(fieldTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse field template: %w", err)
	}

	// Add tags and headers specific to types (not inputs)
	builder.WriteString(fmt.Sprintf(ADOC_TYPE_DEF_START_TAG, t.Name))
	builder.WriteString(fmt.Sprintf("[[type_def_%s]]\n", strings.ToLower(t.Name)))
	builder.WriteString(fmt.Sprintf(".type: %s\n", t.Name))

	builder.WriteString(TABLE_OPTIONS_3 + "\n")
	builder.WriteString(TABLE_SE + "\n")
	builder.WriteString("| Type | Field | Description \n") // Header for types

	for _, f := range t.Fields {
		typeName := processTypeName(f.Type.String(), definitionsMap)
		
		// Process description and extract changelog
		processedDesc, changelog := processDescriptionWithChangelog(f.Description)

		data := FieldData{
			Type:            typeName,
			Name:            f.Name,
			Description:     processedDesc,
			RequiredOrArray: strings.Contains(typeName, "!") || strings.Contains(typeName, "["),
			Required:        isRequiredTypeTpl(typeName), // Use Tpl version
			IsArray:         strings.Contains(typeName, "["),
			Directives:      getDirectivesStringTpl(f.Directives), // Use Tpl version
			Changelog:       changelog,
		}

		// Execute the field template into the builder
		err := fieldTmpl.Execute(&builder, data)
		if err != nil {
			return "", fmt.Errorf("failed to execute field template for field %s: %w", f.Name, err)
		}
		builder.WriteString("\n") // Ensure newline after each row
	}

	builder.WriteString(TABLE_SE + "\n")
	builder.WriteString(fmt.Sprintf(ADOC_TYPE_DEF_END_TAG, t.Name))

	return builder.String(), nil
}

// Add this new function
func printDirectivesTmpl(doc *ast.SchemaDocument) {
	var directiveInfos []DirectiveInfo
	foundDirectives := len(doc.Directives) > 0

	if foundDirectives {
		for _, dir := range doc.Directives {
			// Format arguments
			args := make([]string, 0, len(dir.Arguments))
			for _, arg := range dir.Arguments {
				args = append(args, fmt.Sprintf("%s: %s", arg.Name, arg.Type.String()))
			}
			argumentsString := strings.Join(args, ", ") // Create the formatted string

			directiveInfos = append(directiveInfos, DirectiveInfo{
				Name:        dir.Name,
				Arguments:   argumentsString,
				Description: processDescription(dir.Description),
			})
		}
	}

	// Define table options string in Go code
	tableOptions := `[width="90%", cols="2a,2a,6a" options="header" orientation="landscape" grid="none" stripes="even" , frame="topbot"]`

	data := DirectiveSectionData{
		DirectivesTag:   DIRECTIVES_TAG, // Use existing constant
		FoundDirectives: foundDirectives,
		Directives:      directiveInfos,
		TableOptions:    tableOptions, // Pass the string
	}

	// No custom functions needed for this simple template
	tmpl, err := template.New("directiveSectionTemplate").Parse(directiveSectionTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing directive section template: %v\n", err)
		return
	}

	err = tmpl.Execute(os.Stdout, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing directive section template: %v\n", err)
	}
}

// Add this helper function
func getEnumValuesTableString(t *ast.Definition) string {
	if len(t.EnumValues) == 0 {
		return "" // No values, return empty string
	}

	var builder strings.Builder

	builder.WriteString(fmt.Sprintf(ADOC_ENUM_START_TAG, t.Name))
	builder.WriteString(fmt.Sprintf("[[enum_%s]]\n", camelToSnake(t.Name)))
	builder.WriteString(fmt.Sprintf(".enum_%s\n", camelToSnake(t.Name)))
	builder.WriteString(TABLE_OPTIONS_2 + "\n")
	builder.WriteString(TABLE_SE + "\n")
	builder.WriteString("| Value | Description\n")
	for _, v := range t.EnumValues {
		builder.WriteString(fmt.Sprintf("| %s | %s\n", v.Name, v.Description))
	}
	builder.WriteString(TABLE_SE + "\n")
	builder.WriteString(fmt.Sprintf(ADOC_ENUM_END_TAG, t.Name))

	return builder.String()
}

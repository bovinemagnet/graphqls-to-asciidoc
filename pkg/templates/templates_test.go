package templates

import (
	"strings"
	"testing"
	"text/template"
)

// TestTemplatesSyntax tests that all templates have valid Go template syntax
func TestTemplatesSyntax(t *testing.T) {
	// Create a common function map with all needed functions
	funcMap := template.FuncMap{
		"processDescription":              func(s string) string { return s },
		"printAsciiDocTagsTmpl":          func(s string) string { return s },
		"convertDescriptionToRefNumbers": func(s string, b bool) string { return s },
	}

	testCases := []struct {
		name         string
		templateText string
	}{
		{"FieldTemplate", FieldTemplate},
		{"ScalarTemplate", ScalarTemplate},
		{"SubscriptionTemplate", SubscriptionTemplate},
		{"MutationTemplate", MutationTemplate},
		{"TypeSectionTemplate", TypeSectionTemplate},
		{"EnumSectionTemplate", EnumSectionTemplate},
		{"DirectiveSectionTemplate", DirectiveSectionTemplate},
		{"InputSectionTemplate", InputSectionTemplate},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test that template parses without errors when functions are provided
			_, err := template.New(tc.name).Funcs(funcMap).Parse(tc.templateText)
			if err != nil {
				t.Errorf("Template %s has syntax error: %v", tc.name, err)
			}
		})
	}
}

// TestFieldTemplateExecution tests that FieldTemplate executes correctly with sample data
func TestFieldTemplateExecution(t *testing.T) {
	// Mock function for template
	funcMap := template.FuncMap{
		"processDescription": func(s string) string { return s },
	}

	tmpl, err := template.New("field").Funcs(funcMap).Parse(FieldTemplate)
	if err != nil {
		t.Fatalf("Failed to parse FieldTemplate: %v", err)
	}

	testCases := []struct {
		name     string
		data     interface{}
		contains []string
		excludes []string
	}{
		{
			name: "basic field data",
			data: struct {
				Type            string
				Name            string
				Description     string
				RequiredOrArray bool
				Required        string
				IsArray         bool
				Directives      string
				Changelog       string
			}{
				Type:        "`String`",
				Name:        "testField",
				Description: "Test description",
			},
			contains: []string{"`String`", "testField", "Test description"},
			excludes: []string{".Notes:", ".Required:", ".Array:", ".Directives:", ".Changelog"},
		},
		{
			name: "field with required and array",
			data: struct {
				Type            string
				Name            string
				Description     string
				RequiredOrArray bool
				Required        string
				IsArray         bool
				Directives      string
				Changelog       string
			}{
				Type:            "`[String!]`",
				Name:            "arrayField",
				Description:     "Array field",
				RequiredOrArray: true,
				Required:        "This field is required",
				IsArray:         true,
				Directives:      "@deprecated",
				Changelog:       "\n.Changelog\n* add: 1.0.0\n",
			},
			contains: []string{
				"`[String!]`", "arrayField", "Array field",
				".Notes:", ".Required:", "This field is required",
				".Array:", "True", ".Directives:", "@deprecated",
				".Changelog", "* add: 1.0.0",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			err := tmpl.Execute(&buf, tc.data)
			if err != nil {
				t.Fatalf("Failed to execute template: %v", err)
			}

			result := buf.String()

			// Check for expected content
			for _, expected := range tc.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected template output to contain %q, but it didn't. Output:\n%s", expected, result)
				}
			}

			// Check for excluded content
			for _, excluded := range tc.excludes {
				if strings.Contains(result, excluded) {
					t.Errorf("Expected template output to NOT contain %q, but it did. Output:\n%s", excluded, result)
				}
			}
		})
	}
}

// TestScalarTemplateExecution tests that ScalarTemplate executes correctly
func TestScalarTemplateExecution(t *testing.T) {
	funcMap := template.FuncMap{
		"printAsciiDocTagsTmpl": func(s string) string { return s },
	}

	tmpl, err := template.New("scalar").Funcs(funcMap).Parse(ScalarTemplate)
	if err != nil {
		t.Fatalf("Failed to parse ScalarTemplate: %v", err)
	}

	testCases := []struct {
		name     string
		data     interface{}
		contains []string
		excludes []string
	}{
		{
			name: "no custom scalars",
			data: struct {
				ScalarTag    string
				FoundScalars bool
				Scalars      []interface{}
			}{
				ScalarTag:    "== Scalars",
				FoundScalars: false,
				Scalars:      []interface{}{},
			},
			contains: []string{
				"== Scalars",
				"GraphQL specifies a basic set",
				"No custom scalars exist",
			},
			excludes: []string{"The following custom scalar types"},
		},
		{
			name: "with custom scalars",
			data: struct {
				ScalarTag    string
				FoundScalars bool
				Scalars      []struct{ Name, Description string }
			}{
				ScalarTag:    "== Scalars",
				FoundScalars: true,
				Scalars: []struct{ Name, Description string }{
					{Name: "DateTime", Description: "A date-time string"},
					{Name: "JSON", Description: "A JSON scalar"},
				},
			},
			contains: []string{
				"== Scalars",
				"The following custom scalar types",
				"DateTime", "A date-time string",
				"JSON", "A JSON scalar",
			},
			excludes: []string{"No custom scalars exist"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			err := tmpl.Execute(&buf, tc.data)
			if err != nil {
				t.Fatalf("Failed to execute template: %v", err)
			}

			result := buf.String()

			for _, expected := range tc.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected template output to contain %q, but it didn't. Output:\n%s", expected, result)
				}
			}

			for _, excluded := range tc.excludes {
				if strings.Contains(result, excluded) {
					t.Errorf("Expected template output to NOT contain %q, but it did. Output:\n%s", excluded, result)
				}
			}
		})
	}
}

// TestDirectiveSectionTemplateExecution tests DirectiveSectionTemplate
func TestDirectiveSectionTemplateExecution(t *testing.T) {
	tmpl, err := template.New("directive").Parse(DirectiveSectionTemplate)
	if err != nil {
		t.Fatalf("Failed to parse DirectiveSectionTemplate: %v", err)
	}

	testCases := []struct {
		name     string
		data     interface{}
		contains []string
		excludes []string
	}{
		{
			name: "no custom directives",
			data: struct {
				DirectivesTag   string
				FoundDirectives bool
				TableOptions    string
				Directives      []interface{}
			}{
				DirectivesTag:   "== Directives",
				FoundDirectives: false,
				TableOptions:    "[options=\"header\"]",
				Directives:      []interface{}{},
			},
			contains: []string{
				"== Directives",
				"No custom directives exist",
			},
			excludes: []string{"|===", "| Directive | Arguments | Description"},
		},
		{
			name: "with custom directives",
			data: struct {
				DirectivesTag   string
				FoundDirectives bool
				TableOptions    string
				Directives      []struct{ Name, Arguments, Description string }
			}{
				DirectivesTag:   "== Directives",
				FoundDirectives: true,
				TableOptions:    "[options=\"header\"]",
				Directives: []struct{ Name, Arguments, Description string }{
					{Name: "deprecated", Arguments: "reason: String", Description: "Marks field as deprecated"},
					{Name: "auth", Arguments: "requires: Permission!", Description: "Requires authentication"},
				},
			},
			contains: []string{
				"== Directives",
				"|===",
				"| Directive | Arguments | Description",
				"| @deprecated", "reason: String", "Marks field as deprecated",
				"| @auth", "requires: Permission!", "Requires authentication",
			},
			excludes: []string{"No custom directives exist"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			err := tmpl.Execute(&buf, tc.data)
			if err != nil {
				t.Fatalf("Failed to execute template: %v", err)
			}

			result := buf.String()

			for _, expected := range tc.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected template output to contain %q, but it didn't. Output:\n%s", expected, result)
				}
			}

			for _, excluded := range tc.excludes {
				if strings.Contains(result, excluded) {
					t.Errorf("Expected template output to NOT contain %q, but it did. Output:\n%s", excluded, result)
				}
			}
		})
	}
}

// TestTemplateFunctionRequirements tests that templates contain expected function calls
func TestTemplateFunctionRequirements(t *testing.T) {
	testCases := []struct {
		name          string
		templateText  string
		requiredFuncs []string
	}{
		{
			name:          "FieldTemplate",
			templateText:  FieldTemplate,
			requiredFuncs: []string{}, // processDescription is now done before template execution
		},
		{
			name:          "ScalarTemplate",
			templateText:  ScalarTemplate,
			requiredFuncs: []string{"printAsciiDocTagsTmpl"},
		},
		{
			name:          "MutationTemplate",
			templateText:  MutationTemplate,
			requiredFuncs: []string{"printAsciiDocTagsTmpl"},
		},
		{
			name:          "TypeSectionTemplate",
			templateText:  TypeSectionTemplate,
			requiredFuncs: []string{"printAsciiDocTagsTmpl"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Check that required functions are actually used in the template
			for _, funcName := range tc.requiredFuncs {
				if !strings.Contains(tc.templateText, funcName) {
					t.Errorf("Template %s should contain function %s but doesn't", tc.name, funcName)
				}
			}
		})
	}
}

// TestTemplateConstants tests that all template constants are non-empty and properly formatted
func TestTemplateConstants(t *testing.T) {
	templates := map[string]string{
		"FieldTemplate":           FieldTemplate,
		"ScalarTemplate":          ScalarTemplate,
		"SubscriptionTemplate":    SubscriptionTemplate,
		"MutationTemplate":        MutationTemplate,
		"TypeSectionTemplate":     TypeSectionTemplate,
		"EnumSectionTemplate":     EnumSectionTemplate,
		"DirectiveSectionTemplate": DirectiveSectionTemplate,
		"InputSectionTemplate":    InputSectionTemplate,
	}

	for name, tmpl := range templates {
		t.Run(name, func(t *testing.T) {
			// Check template is not empty
			if strings.TrimSpace(tmpl) == "" {
				t.Errorf("Template %s is empty", name)
			}

			// Check for balanced template actions
			openActions := strings.Count(tmpl, "{{")
			closeActions := strings.Count(tmpl, "}}")
			if openActions != closeActions {
				t.Errorf("Template %s has unbalanced template actions: %d open, %d close", name, openActions, closeActions)
			}

			// Check for common AsciiDoc patterns
			if strings.Contains(name, "SectionTemplate") && name != "DirectiveSectionTemplate" {
				if !strings.Contains(tmpl, "{{range") {
					t.Errorf("Section template %s should contain {{range}} for iteration", name)
				}
			}
		})
	}
}
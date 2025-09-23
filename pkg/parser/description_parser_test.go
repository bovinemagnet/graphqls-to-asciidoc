package parser

import (
	"strings"
	"testing"
)

func TestIsStructuredDescription(t *testing.T) {
	parser := NewDescriptionParser()

	tests := []struct {
		name        string
		description string
		expected    bool
	}{
		{
			name:        "Empty description",
			description: "",
			expected:    false,
		},
		{
			name:        "Simple unstructured description",
			description: "This is a simple description without any structure",
			expected:    false,
		},
		{
			name:        "Description with markdown headers",
			description: "## Overview\nThis is an overview\n## Parameters\nSome params",
			expected:    true,
		},
		{
			name:        "Description with @param annotations",
			description: "Get user by ID\n@param id - The user ID",
			expected:    true,
		},
		{
			name:        "Description with @returns annotation",
			description: "Get user\n@returns User object",
			expected:    true,
		},
		{
			name:        "Description with @throws annotation",
			description: "Delete user\n@throws USER_NOT_FOUND",
			expected:    true,
		},
		{
			name:        "Description with @example annotation",
			description: "Query users\n@example query { users { id } }",
			expected:    true,
		},
		{
			name:        "Description with ### Examples header",
			description: "### Examples\nSome examples here",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.isStructuredDescription(tt.description)
			if result != tt.expected {
				t.Errorf("expected %v, got %v for description: %s", tt.expected, result, tt.description)
			}
		})
	}
}

func TestParseSections(t *testing.T) {
	parser := NewDescriptionParser()

	description := `This is the main description before any sections.

## Overview
This is the overview section content.
Multiple lines are supported here.

## Parameters
These are the parameters.
- param1: First parameter
- param2: Second parameter

## Returns
The return value documentation.

## Custom Section
This is a custom section that should be preserved.`

	structure := &DescriptionStructure{
		Sections: make(map[string]string),
	}

	parser.parseSections(description, structure)

	// Check overview extraction
	if !strings.Contains(structure.Overview, "overview section content") {
		t.Errorf("Overview section not properly extracted: %s", structure.Overview)
	}

	// Check that custom sections are preserved
	if _, exists := structure.Sections["Custom Section"]; !exists {
		t.Error("Custom section was not preserved")
	}

	// Check returns section
	if !strings.Contains(structure.Returns, "return value documentation") {
		t.Errorf("Returns section not properly extracted: %s", structure.Returns)
	}
}

func TestParseJSDocAnnotations(t *testing.T) {
	parser := NewDescriptionParser()

	tests := []struct {
		name        string
		description string
		validate    func(*DescriptionStructure) bool
	}{
		{
			name: "Single @param",
			description: `Get user
@param id - The user identifier`,
			validate: func(s *DescriptionStructure) bool {
				return len(s.Parameters) == 1 &&
					s.Parameters[0].Name == "id" &&
					strings.Contains(s.Parameters[0].Description, "user identifier")
			},
		},
		{
			name: "Multiple @param annotations",
			description: `Create user
@param name - User name
@param email - User email address
@param age - User age in years`,
			validate: func(s *DescriptionStructure) bool {
				return len(s.Parameters) == 3
			},
		},
		{
			name: "Nested @param annotations",
			description: `Create product
@param input - The product input
@param input.name - Product name
@param input.price - Product price
@param input.category - Product category`,
			validate: func(s *DescriptionStructure) bool {
				for _, p := range s.Parameters {
					if p.Name == "input" {
						return len(p.SubParams) == 3
					}
				}
				return false
			},
		},
		{
			name: "@returns annotation",
			description: `Get all users
@returns Array of user objects`,
			validate: func(s *DescriptionStructure) bool {
				return strings.Contains(s.Returns, "Array of user objects")
			},
		},
		{
			name: "@throws annotations",
			description: `Delete user
@throws USER_NOT_FOUND - User does not exist
@throws PERMISSION_DENIED - Insufficient permissions`,
			validate: func(s *DescriptionStructure) bool {
				return len(s.Errors) == 2 &&
					s.Errors[0].Code == "USER_NOT_FOUND" &&
					s.Errors[1].Code == "PERMISSION_DENIED"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structure := &DescriptionStructure{
				Parameters: []ParameterDoc{},
				Errors:     []ErrorDoc{},
			}
			parser.parseJSDocAnnotations(tt.description, structure)
			if !tt.validate(structure) {
				t.Errorf("JSDoc annotation parsing failed for: %s", tt.description)
			}
		})
	}
}

func TestParseChangelog(t *testing.T) {
	parser := NewDescriptionParser()

	description := `User query
@version add.1.0.0
@version update.1.5.0 Added email field
@version deprecate.2.0.0 Use getUser instead
@version remove.3.0.0`

	structure := &DescriptionStructure{
		Changelog: []ChangelogEntry{},
	}

	parser.parseChangelog(description, structure)

	if len(structure.Changelog) != 4 {
		t.Errorf("Expected 4 changelog entries, got %d", len(structure.Changelog))
	}

	// Check specific entries
	if structure.Changelog[0].Type != "add" || structure.Changelog[0].Version != "1.0.0" {
		t.Error("First changelog entry not parsed correctly")
	}

	if structure.Changelog[1].Type != "update" || !strings.Contains(structure.Changelog[1].Description, "email field") {
		t.Error("Update changelog entry with description not parsed correctly")
	}
}

func TestParseExamples(t *testing.T) {
	parser := NewDescriptionParser()

	description := `Query users

### Basic Example
` + "```" + `graphql
query {
  users {
    id
    name
  }
}
` + "```" + `

### Advanced Example
` + "```" + `graphql
query($filter: UserFilter) {
  users(filter: $filter) {
    id
    name
    email
  }
}
` + "```" + `

@example { users { id } }`

	structure := &DescriptionStructure{
		Examples: []Example{},
	}

	parser.parseExamples(description, structure)

	if len(structure.Examples) != 3 {
		t.Errorf("Expected 3 examples, got %d", len(structure.Examples))
	}

	// Check that languages are set correctly
	if structure.Examples[0].Language != "graphql" {
		t.Errorf("Expected language 'graphql', got %s", structure.Examples[0].Language)
	}

	// Check titles
	if !strings.Contains(structure.Examples[0].Title, "Basic Example") {
		t.Error("Example title not parsed correctly")
	}
}

func TestParseMetadata(t *testing.T) {
	parser := NewDescriptionParser()

	tests := []struct {
		name        string
		description string
		validate    func(map[string]string) bool
	}{
		{
			name:        "@since annotation",
			description: "Get user\n@since 1.0.0",
			validate: func(m map[string]string) bool {
				return m["since"] == "1.0.0"
			},
		},
		{
			name:        "@deprecated annotation",
			description: "Old query\n@deprecated 2.0.0 Use newQuery instead",
			validate: func(m map[string]string) bool {
				return strings.Contains(m["deprecated"], "Use newQuery")
			},
		},
		{
			name:        "@beta flag",
			description: "Beta feature\n@beta",
			validate: func(m map[string]string) bool {
				return m["beta"] == "true"
			},
		},
		{
			name:        "@experimental flag",
			description: "Experimental API\n@experimental",
			validate: func(m map[string]string) bool {
				return m["experimental"] == "true"
			},
		},
		{
			name:        "@internal flag",
			description: "Internal use only\n@internal",
			validate: func(m map[string]string) bool {
				return m["internal"] == "true"
			},
		},
		{
			name:        "Multiple metadata",
			description: "Query\n@since 1.0.0\n@beta\n@internal",
			validate: func(m map[string]string) bool {
				return m["since"] == "1.0.0" && m["beta"] == "true" && m["internal"] == "true"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structure := &DescriptionStructure{
				Metadata: make(map[string]string),
			}
			parser.parseMetadata(tt.description, structure)
			if !tt.validate(structure.Metadata) {
				t.Errorf("Metadata parsing failed for: %s", tt.description)
			}
		})
	}
}

func TestCalculateMetrics(t *testing.T) {
	parser := NewDescriptionParser()

	tests := []struct {
		name      string
		structure *DescriptionStructure
		validate  func(*DescriptionMetrics) bool
	}{
		{
			name: "Complete documentation",
			structure: &DescriptionStructure{
				Overview:   "This is an overview with multiple words",
				Parameters: []ParameterDoc{{Name: "id", Description: "User ID"}},
				Returns:    "User object",
				Errors:     []ErrorDoc{{Code: "NOT_FOUND", Description: "Not found"}},
				Examples:   []Example{{Title: "Example", Code: "query{}"}},
				Changelog:  []ChangelogEntry{{Type: "add", Version: "1.0.0"}},
			},
			validate: func(m *DescriptionMetrics) bool {
				return m.HasOverview && m.HasParameters && m.HasReturns &&
					m.HasErrors && m.HasExamples && m.HasChangelog &&
					m.WordCount > 0 && m.Completeness > 0.8
			},
		},
		{
			name: "Minimal documentation",
			structure: &DescriptionStructure{
				Overview: "Simple description",
			},
			validate: func(m *DescriptionMetrics) bool {
				return m.HasOverview && !m.HasParameters && !m.HasReturns &&
					m.Complexity == "simple" && m.Completeness < 0.5
			},
		},
		{
			name:      "Empty documentation",
			structure: &DescriptionStructure{},
			validate: func(m *DescriptionMetrics) bool {
				return !m.HasOverview && m.WordCount == 0 && m.Completeness == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := parser.calculateMetrics(tt.structure, "")
			if !tt.validate(metrics) {
				t.Errorf("Metrics calculation failed for %s", tt.name)
			}
		})
	}
}

func TestExtractParameterType(t *testing.T) {
	parser := NewDescriptionParser()

	tests := []struct {
		description    string
		expectedType   string
		expectedDesc   string
	}{
		{
			description:    "(String) The user name",
			expectedType:   "String",
			expectedDesc:   "The user name",
		},
		{
			description:    "[Int] Array of numbers",
			expectedType:   "Int",
			expectedDesc:   "Array of numbers",
		},
		{
			description:    "{UserInput} User input object",
			expectedType:   "UserInput",
			expectedDesc:   "User input object",
		},
		{
			description:    "<ID> User identifier",
			expectedType:   "ID",
			expectedDesc:   "User identifier",
		},
		{
			description:    "Plain description without type",
			expectedType:   "",
			expectedDesc:   "Plain description without type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			paramType, cleanDesc := parser.ExtractParameterType(tt.description)
			if paramType != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, paramType)
			}
			if cleanDesc != tt.expectedDesc {
				t.Errorf("Expected description %s, got %s", tt.expectedDesc, cleanDesc)
			}
		})
	}
}

func TestExtractDefault(t *testing.T) {
	parser := NewDescriptionParser()

	tests := []struct {
		description     string
		expectedDefault string
		expectedDesc    string
	}{
		{
			description:     "Page size (default: 10)",
			expectedDefault: "10",
			expectedDesc:    "Page size",
		},
		{
			description:     "Sort order default: ASC for ascending",
			expectedDefault: "ASC",
			expectedDesc:    "Sort order for ascending",
		},
		{
			description:     "Include metadata (default true)",
			expectedDefault: "true",
			expectedDesc:    "Include metadata",
		},
		{
			description:     "Description without default",
			expectedDefault: "",
			expectedDesc:    "Description without default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			defaultValue, cleanDesc := parser.ExtractDefault(tt.description)
			if defaultValue != tt.expectedDefault {
				t.Errorf("Expected default %s, got %s", tt.expectedDefault, defaultValue)
			}
			if strings.TrimSpace(cleanDesc) != tt.expectedDesc {
				t.Errorf("Expected description %s, got %s", tt.expectedDesc, cleanDesc)
			}
		})
	}
}

func TestParseDescriptionIntegration(t *testing.T) {
	parser := NewDescriptionParser()

	// Test a complex structured description
	description := `Get user information by ID.

## Overview
Retrieves detailed user information from the system.

## Parameters
@param id - The unique identifier of the user
@param includeMetadata - (Boolean) Include metadata (default: false)

## Returns
Returns a User object containing the requested information.

## Errors
@throws USER_NOT_FOUND - The specified user ID does not exist
@throws PERMISSION_DENIED - Insufficient permissions

## Examples
### Basic Usage
` + "```" + `graphql
query {
  getUser(id: "123") {
    id
    name
  }
}
` + "```" + `

@version add.1.0.0
@version update.2.0.0 Added includeMetadata parameter
@since 1.0.0
@beta`

	parsed := parser.ParseDescription(description)

	if parsed.Structured == nil {
		t.Fatal("Expected structured description, got nil")
	}

	// Verify structure was parsed correctly
	if !parsed.Structured.IsStructured {
		t.Error("Description should be marked as structured")
	}

	if len(parsed.Structured.Parameters) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(parsed.Structured.Parameters))
	}

	if len(parsed.Structured.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(parsed.Structured.Errors))
	}

	if len(parsed.Structured.Examples) != 1 {
		t.Errorf("Expected 1 example, got %d", len(parsed.Structured.Examples))
	}

	if len(parsed.Structured.Changelog) != 2 {
		t.Errorf("Expected 2 changelog entries, got %d", len(parsed.Structured.Changelog))
	}

	// Check metrics
	if parsed.Metrics == nil {
		t.Fatal("Expected metrics, got nil")
	}

	if !parsed.Metrics.HasOverview || !parsed.Metrics.HasParameters || !parsed.Metrics.HasReturns {
		t.Error("Metrics should indicate presence of overview, parameters, and returns")
	}
}

func TestBackwardCompatibility(t *testing.T) {
	parser := NewDescriptionParser()

	// Test that simple descriptions still work
	simpleDesc := "This is a simple description without any special formatting or annotations."

	parsed := parser.ParseDescription(simpleDesc)

	if parsed.Structured != nil && parsed.Structured.IsStructured {
		t.Error("Simple description should not be parsed as structured")
	}

	if parsed.Unstructured != simpleDesc {
		t.Errorf("Unstructured description should match input: got %s", parsed.Unstructured)
	}
}
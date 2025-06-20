package parser

import (
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
)

func TestCamelToSnake(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"TestFunction", "test_function"},
		{"CLogExample", "c_log_example"},
		{"UserProfile", "user_profile"},
		{"ID", "i_d"},
		{"HTTPServer", "h_t_t_p_server"},
		{"XMLParser", "x_m_l_parser"},
		{"", ""},
		{"lowercase", "lowercase"},
		{"A", "a"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := CamelToSnake(tc.input)
			if result != tc.expected {
				t.Errorf("CamelToSnake(%q) = %q; expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestFormatDeprecatedDirectives(t *testing.T) {
	testCases := []struct {
		name        string
		description string
		expected    string
	}{
		{
			name:        "simple deprecated",
			description: "This field is @deprecated",
			expected:    "This field is `@deprecated`",
		},
		{
			name:        "deprecated with reason",
			description: "This field is @deprecated(reason: \"Use newField instead\")",
			expected:    "This field is `@deprecated(reason: \"Use newField instead\")`",
		},
		{
			name:        "deprecated with simple argument",
			description: "This field is @deprecated(\"Use newField instead\")",
			expected:    "This field is `@deprecated(\"Use newField instead\")`",
		},
		{
			name:        "already formatted deprecated",
			description: "This field is `@deprecated(reason: \"test\")`",
			expected:    "This field is `@deprecated(reason: \"test\")`",
		},
		{
			name:        "multiple deprecated directives",
			description: "Field1 is @deprecated and field2 is @deprecated(reason: \"test\")",
			expected:    "Field1 is `@deprecated` and field2 is `@deprecated(reason: \"test\")`",
		},
		{
			name:        "no deprecated directives",
			description: "This is a normal field description",
			expected:    "This is a normal field description",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatDeprecatedDirectives(tc.description)
			if result != tc.expected {
				t.Errorf("FormatDeprecatedDirectives(%q) = %q; expected %q", tc.description, result, tc.expected)
			}
		})
	}
}

func TestConvertMarkdownCodeBlocks(t *testing.T) {
	testCases := []struct {
		name        string
		description string
		expected    string
	}{
		{
			name: "simple graphql code block",
			description: `Example query:

` + "```graphql" + `
query {
  user {
    name
  }
}
` + "```" + ``,
			expected: `Example query:

[source,graphql]
----
query {
  user {
    name
  }
}
----`,
		},
		{
			name: "code block without language",
			description: `Example:

` + "```" + `
some code
` + "```" + ``,
			expected: `Example:

[source,text]
----
some code
----`,
		},
		{
			name:        "no code blocks",
			description: "Just a normal description",
			expected:    "Just a normal description",
		},
		{
			name: "multiple code blocks",
			description: `First block:

` + "```javascript" + `
console.log("hello");
` + "```" + `

Second block:

` + "```graphql" + `
query { user }
` + "```" + ``,
			expected: `First block:

[source,javascript]
----
console.log("hello");
----

Second block:

[source,graphql]
----
query { user }
----`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ConvertMarkdownCodeBlocks(tc.description)
			if result != tc.expected {
				t.Errorf("ConvertMarkdownCodeBlocks() = %q; expected %q", result, tc.expected)
			}
		})
	}
}

func TestProcessDescription(t *testing.T) {
	testCases := []struct {
		name        string
		description string
		expected    string
	}{
		{
			name:        "simple description",
			description: "This is a simple description.",
			expected:    "This is a simple description.",
		},
		{
			name: "description with list items",
			description: `This field has options:

- Option 1
- Option 2
- Option 3`,
			expected: `This field has options:

* Option 1
* Option 2
* Option 3`,
		},
		{
			name: "description with deprecated and code block",
			description: `This field is deprecated.

@deprecated(reason: "Use newField")

Example:

` + "```graphql" + `
query { field }
` + "```" + `

- Note: This will be removed`,
			expected: `This field is deprecated.

` + "`@deprecated(reason: \"Use newField\")`" + `

Example:

[source,graphql]
----
query { field }
----

* Note: This will be removed`,
		},
		{
			name:        "preserve dates and URLs",
			description: "Updated on 2024-01-08 at https://example.com/path",
			expected:    "Updated on 2024-01-08 at https://example.com/path",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ProcessDescription(tc.description)
			if result != tc.expected {
				t.Errorf("ProcessDescription() = %q; expected %q", result, tc.expected)
			}
		})
	}
}

func TestProcessTypeName(t *testing.T) {
	// Create a mock definitions map for testing
	mockDefinitionsMap := map[string]*ast.Definition{
		"User":    {Name: "User", Kind: ast.Object},
		"Message": {Name: "Message", Kind: ast.Object},
	}

	testCases := []struct {
		name     string
		typeName string
		expected string
	}{
		{
			name:     "simple scalar type",
			typeName: "String",
			expected: "`String`",
		},
		{
			name:     "object type reference",
			typeName: "User",
			expected: "<<User,`User`>>",
		},
		{
			name:     "list of objects",
			typeName: "[User]",
			expected: "[<<User,`User`>>]",
		},
		{
			name:     "required scalar",
			typeName: "String!",
			expected: "`String!`",
		},
		{
			name:     "required object",
			typeName: "User!",
			expected: "<<User,`User`>>!",
		},
		{
			name:     "list of required objects",
			typeName: "[User!]",
			expected: "[<<User,`User`>>!]",
		},
		{
			name:     "required list of objects",
			typeName: "[User]!",
			expected: "[<<User,`User`>>]!",
		},
		{
			name:     "unknown type",
			typeName: "UnknownType",
			expected: "`UnknownType`",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ProcessTypeName(tc.typeName, mockDefinitionsMap)
			if result != tc.expected {
				t.Errorf("ProcessTypeName(%q) = %q; expected %q", tc.typeName, result, tc.expected)
			}
		})
	}
}

func TestCleanDescription(t *testing.T) {
	testCases := []struct {
		name          string
		text          string
		skipCharacter string
		expected      string
	}{
		{
			name:          "remove lines starting with dash",
			text:          "Description\n- Skip this line\nKeep this line\n- Skip this too",
			skipCharacter: "-",
			expected:      "Description\nKeep this line\n",
		},
		{
			name:          "preserve asciidoc code block delimiters",
			text:          "Description\n- Skip this\n----\nCode block\n----\n- Skip this too",
			skipCharacter: "-",
			expected:      "Description\n----\nCode block\n----\n",
		},
		{
			name:          "no lines to skip",
			text:          "Normal description\nWith multiple lines",
			skipCharacter: "-",
			expected:      "Normal description\nWith multiple lines\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := CleanDescription(tc.text, tc.skipCharacter)
			if result != tc.expected {
				t.Errorf("CleanDescription() = %q; expected %q", result, tc.expected)
			}
		})
	}
}

func TestConvertDescriptionToRefNumbers(t *testing.T) {
	testCases := []struct {
		name        string
		text        string
		skipNonDash bool
		expected    string
	}{
		{
			name:        "convert dash list to numbered refs",
			text:        "Description\n- First item\n- Second item\nNormal line",
			skipNonDash: false,
			expected:    "Description\n<1> First item\n<2> Second item\nNormal line\n",
		},
		{
			name:        "skip non-dash lines",
			text:        "Description\n- First item\nNormal line\n- Second item",
			skipNonDash: true,
			expected:    "<1> First item\n<2> Second item\n",
		},
		{
			name:        "ignore double dashes",
			text:        "Description\n-- This is ignored\n- This is converted",
			skipNonDash: false,
			expected:    "Description\n-- This is ignored\n<1> This is converted\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ConvertDescriptionToRefNumbers(tc.text, tc.skipNonDash)
			if result != tc.expected {
				t.Errorf("ConvertDescriptionToRefNumbers() = %q; expected %q", result, tc.expected)
			}
		})
	}
}
package parser

import (
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
)

func TestNormalizeIndentation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no indentation",
			input:    "Line 1\nLine 2\nLine 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "consistent indentation",
			input:    "    Line 1\n    Line 2\n    Line 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "mixed indentation with empty lines",
			input:    "    Line 1\n\n    Line 2\n        Indented more\n    Line 3",
			expected: "Line 1\n\nLine 2\n    Indented more\nLine 3",
		},
		{
			name:     "GraphQL-style description",
			input:    "    ## Overview\n    This is the overview.\n    \n    ## Details\n    Some details here.",
			expected: "## Overview\nThis is the overview.\n\n## Details\nSome details here.",
		},
		{
			name:     "leading and trailing empty lines",
			input:    "\n\n    Content\n    More content\n\n\n",
			expected: "Content\nMore content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeIndentation(tt.input)
			if result != tt.expected {
				t.Errorf("expected:\n%q\ngot:\n%q", tt.expected, result)
			}
		})
	}
}

func TestConvertMarkdownHeadersToAsciiDoc(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single level header",
			input:    "# Main Title",
			expected: "== Main Title",
		},
		{
			name:     "second level header",
			input:    "## Section Header",
			expected: "=== Section Header",
		},
		{
			name:     "third level header",
			input:    "### Subsection",
			expected: "==== Subsection",
		},
		{
			name:     "mixed content with headers",
			input:    "Some text\n## Overview\nContent here\n### Details\nMore content",
			expected: "Some text\n=== Overview\nContent here\n==== Details\nMore content",
		},
		{
			name:     "no headers",
			input:    "Just plain text\nNo headers here",
			expected: "Just plain text\nNo headers here",
		},
		{
			name:     "header with trailing spaces",
			input:    "## Header   ",
			expected: "=== Header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertMarkdownHeadersToAsciiDoc(tt.input)
			if result != tt.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", tt.expected, result)
			}
		})
	}
}

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

func TestConvertAdmonitionBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single line note with asterisks",
			input:    "**NOTE**: This is a simple note",
			expected: "[NOTE]\n====\nThis is a simple note\n====",
		},
		{
			name:     "single line warning without asterisks",
			input:    "WARNING: Be careful here",
			expected: "[WARNING]\n====\nBe careful here\n====",
		},
		{
			name:     "multi-line important block",
			input:    "**IMPORTANT**\nThis is a multi-line\nimportant message",
			expected: "[IMPORTANT]\n====\nThis is a multi-line\nimportant message\n====",
		},
		{
			name:     "multiple admonition types",
			input:    "**NOTE**: First note\n\n**WARNING**: Then a warning\n\nTIP: Finally a tip",
			expected: "[NOTE]\n====\nFirst note\n====\n\n[WARNING]\n====\nThen a warning\n====\n\n[TIP]\n====\nFinally a tip\n====",
		},
		{
			name:     "no admonitions",
			input:    "Just regular text with no special formatting",
			expected: "Just regular text with no special formatting",
		},
		{
			name:     "caution admonition",
			input:    "CAUTION: This operation is dangerous",
			expected: "[CAUTION]\n====\nThis operation is dangerous\n====",
		},
		{
			name:     "tip admonition",
			input:    "TIP: You can optimize this by caching",
			expected: "[TIP]\n====\nYou can optimize this by caching\n====",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertAdmonitionBlocks(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertAdmonitionBlocks() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestProcessCallouts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "parentheses callouts",
			input:    "function test() (1)\n  return value (2)",
			expected: "function test() <1>\n  return value <2>",
		},
		{
			name:     "double slash comment callouts",
			input:    "let x = 5; // 1\nlet y = 10; // 2",
			expected: "let x = 5; <1>\nlet y = 10; <2>",
		},
		{
			name:     "hash comment callouts",
			input:    "x = 5 # 1\ny = 10 # 2",
			expected: "x = 5 <1>\ny = 10 <2>",
		},
		{
			name:     "block comment callouts",
			input:    "function test() /* 1 */\n  return value /* 2 */",
			expected: "function test() <1>\n  return value <2>",
		},
		{
			name:     "mixed callout patterns",
			input:    "code (1)\nmore code // 2\neven more # 3\nfinal /* 4 */",
			expected: "code <1>\nmore code <2>\neven more <3>\nfinal <4>",
		},
		{
			name:     "no callouts",
			input:    "regular code without callouts\nnothing special here",
			expected: "regular code without callouts\nnothing special here",
		},
		{
			name:     "ignore non-end-of-line comments",
			input:    "// this is a comment\n# this is also a comment\ncode // 1",
			expected: "// this is a comment\n# this is also a comment\ncode <1>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessCallouts(tt.input)
			if result != tt.expected {
				t.Errorf("ProcessCallouts() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertMarkdownCodeBlocksWithCallouts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "code block with parentheses callouts",
			input:    "Example:\n\n```javascript\nfunction test() (1)\n  return value (2)\n```",
			expected: "Example:\n\n[source,javascript]\n----\nfunction test() <1>\n  return value <2>\n----",
		},
		{
			name:     "code block with comment callouts",
			input:    "Example:\n\n```python\nx = 5 # 1\ny = 10 # 2\n```",
			expected: "Example:\n\n[source,python]\n----\nx = 5 <1>\ny = 10 <2>\n----",
		},
		{
			name:     "code block without callouts",
			input:    "Example:\n\n```graphql\nquery {\n  user {\n    name\n  }\n}\n```",
			expected: "Example:\n\n[source,graphql]\n----\nquery {\n  user {\n    name\n  }\n}\n----",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertMarkdownCodeBlocks(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertMarkdownCodeBlocks() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestProcessAnchorsAndLabels(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "convert [#id] to [[id]]",
			input:    "See section [#introduction] for details",
			expected: "See section [[introduction]] for details",
		},
		{
			name:     "preserve existing [[anchor]] format",
			input:    "See section [[existing-anchor]] for details",
			expected: "See section [[existing-anchor]] for details",
		},
		{
			name:     "preserve existing cross-references",
			input:    "See <<reference,text>> and <<simple-ref>>",
			expected: "See <<reference,text>> and <<simple-ref>>",
		},
		{
			name:     "convert standalone [label] to [[label]]",
			input:    "This is a section\n[important-note]\nThis note is important",
			expected: "This is a section\n[[important-note]]\nThis note is important",
		},
		{
			name:     "convert {ref:anchor} to <<anchor>>",
			input:    "Please see {ref:installation} section",
			expected: "Please see <<installation>> section",
		},
		{
			name:     "convert {link:anchor|text} to <<anchor,text>>",
			input:    "Read {link:setup|the setup guide} first",
			expected: "Read <<setup,the setup guide>> first",
		},
		{
			name:     "mixed anchor patterns",
			input:    "See [#intro], then {ref:setup}, and finally {link:deploy|deployment guide}",
			expected: "See [[intro]], then <<setup>>, and finally <<deploy,deployment guide>>",
		},
		{
			name:     "no anchor patterns",
			input:    "Regular text with no special formatting",
			expected: "Regular text with no special formatting",
		},
		{
			name:     "complex example with multiple patterns",
			input:    "[#main-section]\nThis is the main section.\n\nRefer to {ref:subsection} and {link:conclusion|the conclusion}.\nAlso see <<existing-ref,existing reference>>.",
			expected: "[[main-section]]\nThis is the main section.\n\nRefer to <<subsection>> and <<conclusion,the conclusion>>.\nAlso see <<existing-ref,existing reference>>.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessAnchorsAndLabels(tt.input)
			if result != tt.expected {
				t.Errorf("ProcessAnchorsAndLabels() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestProcessDescriptionWithAnchors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "description with anchors and admonitions",
			input:    "[#example-section]\nThis is an example.\n\n**NOTE**: See {ref:related-section} for more info.\n\n{link:api-docs|API documentation} contains details.",
			expected: "[[example-section]]\nThis is an example.\n\n[NOTE]\n====\nSee <<related-section>> for more info.\n====\n\n<<api-docs,API documentation>> contains details.",
		},
		{
			name:     "description with code blocks and anchors",
			input:    "[#code-example]\n\n```javascript\nfunction example() { (1)\n  return value; // 2\n}\n```\n\nSee {ref:implementation} for details.",
			expected: "[[code-example]]\n\n[source,javascript]\n----\nfunction example() { <1>\n  return value; <2>\n}\n----\n\nSee <<implementation>> for details.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessDescription(tt.input)
			if result != tt.expected {
				t.Errorf("ProcessDescription() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestProcessTables(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "preserve existing AsciiDoc table",
			input:    "[options=\"header\"]\n|===\n| Name | Type | Description\n| id | String | User ID\n| name | String | User name\n|===",
			expected: "[options=\"header\"]\n|===\n| Name | Type | Description\n| id | String | User ID\n| name | String | User name\n|===",
		},
		{
			name:     "no tables in content",
			input:    "This is regular text with no tables.",
			expected: "This is regular text with no tables.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessTables(tt.input)
			if result != tt.expected {
				t.Errorf("ProcessTables() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertMarkdownTables(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple markdown table",
			input:    "| Name | Type | Description |\n|------|------|-------------|\n| id | String | User ID |\n| name | String | User name |",
			expected: "[options=\"header\"]\n|===\n| Name | Type | Description\n| id | String | User ID\n| name | String | User name\n|===",
		},
		{
			name:     "markdown table with alignment",
			input:    "| Name | Type | Description |\n|:-----|:----:|------------:|\n| id | String | User ID |\n| name | String | User name |",
			expected: "[options=\"header\"]\n|===\n| Name | Type | Description\n| id | String | User ID\n| name | String | User name\n|===",
		},
		{
			name:     "markdown table without header separator",
			input:    "| Name | Type | Description |\n| id | String | User ID |\n| name | String | User name |",
			expected: "[options=\"header\"]\n|===\n| Name | Type | Description\n| id | String | User ID\n| name | String | User name\n|===",
		},
		{
			name:     "table with mixed content",
			input:    "Here's a table:\n\n| Field | Required | Notes |\n|-------|----------|-------|\n| email | Yes | Must be valid |\n| phone | No | Optional field |\n\nEnd of table.",
			expected: "Here's a table:\n\n[options=\"header\"]\n|===\n| Field | Required | Notes\n| email | Yes | Must be valid\n| phone | No | Optional field\n|===\n\nEnd of table.",
		},
		{
			name:     "no markdown tables",
			input:    "This text has no tables in it.",
			expected: "This text has no tables in it.",
		},
		{
			name:     "malformed table row",
			input:    "| Name | Type\n|------|------|\n| id | String |",
			expected: "[options=\"header\"]\n|===\n| Name | Type\n| id | String\n|===",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertMarkdownTables(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertMarkdownTables() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseTableRow(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple row",
			input:    "| Name | Type | Description |",
			expected: []string{"Name", "Type", "Description"},
		},
		{
			name:     "row without outer pipes",
			input:    "Name | Type | Description",
			expected: []string{"Name", "Type", "Description"},
		},
		{
			name:     "row with extra spaces",
			input:    "|  Name  |  Type  |  Description  |",
			expected: []string{"Name", "Type", "Description"},
		},
		{
			name:     "empty row",
			input:    "| | | |",
			expected: []string{},
		},
		{
			name:     "single cell",
			input:    "| Single Cell |",
			expected: []string{"Single Cell"},
		},
		{
			name:     "completely empty",
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTableRow(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parseTableRow() length = %d, want %d", len(result), len(tt.expected))
				return
			}
			for i, cell := range result {
				if cell != tt.expected[i] {
					t.Errorf("parseTableRow()[%d] = %q, want %q", i, cell, tt.expected[i])
				}
			}
		})
	}
}

func TestProcessDescriptionWithTables(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "markdown table with other features",
			input:    "**NOTE**: Check the parameters table below.\n\n| Parameter | Type | Required |\n|-----------|------|----------|\n| id | String | Yes |\n| name | String | No |\n\nSee {ref:examples} for usage.",
			expected: "[NOTE]\n====\nCheck the parameters table below.\n====\n\n[options=\"header\"]\n|===\n| Parameter | Type | Required\n| id | String | Yes\n| name | String | No\n|===\n\nSee <<examples>> for usage.",
		},
		{
			name:     "existing AsciiDoc table preservation",
			input:    "Parameters:\n\n|===\n| Name | Description\n| limit | Maximum results\n| offset | Starting position\n|===\n\nTable complete.",
			expected: "Parameters:\n\n|===\n| Name | Description\n| limit | Maximum results\n| offset | Starting position\n|===\n\nTable complete.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessDescription(tt.input)
			if result != tt.expected {
				t.Errorf("ProcessDescription() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestStripCodeBlocksFromDescriptions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "strip AsciiDoc code block from description",
			input: `type Query {
	"""
	Get user by ID.
	
	.Example:
	[source,kotlin]
	----
	input DateFilter {
	  startDateTime: LocalDateTime!
	  endDateTime: LocalDateTime!
	}
	----
	"""
	getUser(id: ID!): User
}`,
			expected: `type Query {
	"""
	Get user by ID.
	
	.Example:
	[CODE_BLOCK_REMOVED]
	"""
	getUser(id: ID!): User
}`,
		},
		{
			name: "strip markdown code block from description",
			input: `type Query {
	"""
	Example query.
	
	` + "```graphql" + `
	query {
	  user {
	    name
	  }
	}
	` + "```" + `
	"""
	test: String
}`,
			expected: `type Query {
	"""
	Example query.
	
	[CODE_BLOCK_REMOVED]
	"""
	test: String
}`,
		},
		{
			name: "strip multiple code blocks",
			input: `"""
Multiple examples.

[source,graphql]
----
query { field1 }
----

And another:

` + "```json" + `
{ "key": "value" }
` + "```" + `
"""`,
			expected: `"""
Multiple examples.

[CODE_BLOCK_REMOVED]

And another:

[CODE_BLOCK_REMOVED]
"""`,
		},
		{
			name: "preserve non-code content",
			input: `"""
This is a description with:
- List items
- More items

**Important:** Some text
"""`,
			expected: `"""
This is a description with:
- List items
- More items

**Important:** Some text
"""`,
		},
		{
			name: "no descriptions",
			input: `type Query {
	test: String
}`,
			expected: `type Query {
	test: String
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripCodeBlocksFromDescriptions(tt.input)
			if result != tt.expected {
				t.Errorf("StripCodeBlocksFromDescriptions() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConvertArgumentsPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "convert .Arguments: to .Arguments",
			input:    "Some description\n\n.Arguments:\n- item 1\n- item 2",
			expected: "Some description\n\n.Arguments\n- item 1\n- item 2",
		},
		{
			name:     "convert **Arguments:** to .Arguments",
			input:    "Some description\n\n**Arguments:**\n- item 1\n- item 2",
			expected: "Some description\n\n.Arguments\n- item 1\n- item 2",
		},
		{
			name:     "convert both patterns",
			input:    "First section\n.Arguments:\n- item 1\n\nSecond section\n**Arguments:**\n- item 2",
			expected: "First section\n.Arguments\n- item 1\n\nSecond section\n.Arguments\n- item 2",
		},
		{
			name:     "no arguments patterns",
			input:    "Just a regular description with no arguments section",
			expected: "Just a regular description with no arguments section",
		},
		{
			name:     "arguments pattern with extra spaces",
			input:    "Description\n\n.Arguments:   \n- item 1",
			expected: "Description\n\n.Arguments\n- item 1",
		},
		{
			name:     "arguments pattern with extra spaces in markdown",
			input:    "Description\n\n**Arguments:**   \n- item 1",
			expected: "Description\n\n.Arguments\n- item 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertArgumentsPatterns(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertArgumentsPatterns() = %q, want %q", result, tt.expected)
			}
		})
	}
}

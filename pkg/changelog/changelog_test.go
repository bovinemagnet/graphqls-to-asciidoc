package changelog

import (
	"testing"
)

func TestExtract(t *testing.T) {
	testCases := []struct {
		name        string
		description string
		expected    string
	}{
		{
			name: "single add version",
			description: `This is a test field.

add.version: 1.0.0`,
			expected: "\n.Changelog\n* add: 1.0.0\n",
		},
		{
			name: "multiple version types",
			description: `This is a test field.

add.version: 1.0.0
update.version: 1.2.3
update.version: 2.0.5
deprecated.version: 2.6.0
removed.version: 2.7.8`,
			expected: "\n.Changelog\n* add: 1.0.0\n* update: 1.2.3, 2.0.5\n* deprecated: 2.6.0\n* removed: 2.7.8\n",
		},
		{
			name:        "no version annotations",
			description: "This is a test field with no versions.",
			expected:    "",
		},
		{
			name: "indented version annotations",
			description: `This is a test field.

    add.version: 1.0.0
    update.version: 1.2.3`,
			expected: "\n.Changelog\n* add: 1.0.0\n* update: 1.2.3\n",
		},
		{
			name:        "empty description",
			description: "",
			expected:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := Extract(tc.description)
			if result != tc.expected {
				t.Errorf("Extract() = %q; expected %q", result, tc.expected)
			}
		})
	}
}

func TestProcessWithChangelog(t *testing.T) {
	// Mock processor function
	mockProcessor := func(desc string) string {
		// Simple mock - just trim and convert lists
		return desc
	}

	testCases := []struct {
		name              string
		description       string
		expectedDesc      string
		expectedChangelog string
	}{
		{
			name: "description with changelog",
			description: `This is a test field.

add.version: 1.0.0
update.version: 1.2.3

- Some list item
- Another item`,
			expectedDesc:      "This is a test field.\n\n- Some list item\n- Another item",
			expectedChangelog: "\n.Changelog\n* add: 1.0.0\n* update: 1.2.3\n",
		},
		{
			name:              "description without changelog",
			description:       "Just a simple description with no versions.",
			expectedDesc:      "Just a simple description with no versions.",
			expectedChangelog: "",
		},
		{
			name: "description with @deprecated",
			description: `This field is deprecated.

@deprecated(reason: "Use newField instead")

add.version: 1.0.0`,
			expectedDesc:      "This field is deprecated.\n\n@deprecated(reason: \"Use newField instead\")\n",
			expectedChangelog: "\n.Changelog\n* add: 1.0.0\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			desc, changelog := ProcessWithChangelog(tc.description, mockProcessor)
			if desc != tc.expectedDesc {
				t.Errorf("ProcessWithChangelog() description = %q; expected %q", desc, tc.expectedDesc)
			}
			if changelog != tc.expectedChangelog {
				t.Errorf("ProcessWithChangelog() changelog = %q; expected %q", changelog, tc.expectedChangelog)
			}
		})
	}
}
package parser

import "testing"

// A markdown fence is a literal region: a `#` inside it is source code (a Python
// or shell comment), not a heading. Converting it to `==` corrupts the sample and
// injects a spurious section into the generated document.
func TestConvertMarkdownHeadersToAsciiDocSkipsCodeBlocks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "heading outside a fence is converted",
			input: "# Overview\n\nSome text.",
			want:  "== Overview\n\nSome text.",
		},
		{
			name:  "comment inside a fence is left alone",
			input: "```python\ndef f():\n    # Validate input\n    return 1\n```",
			want:  "```python\ndef f():\n    # Validate input\n    return 1\n```",
		},
		{
			name:  "headings resume after the fence closes",
			input: "```sh\n# not a heading\n```\n\n# Real Heading",
			want:  "```sh\n# not a heading\n```\n\n== Real Heading",
		},
		{
			name:  "language-less fence is still a fence",
			input: "```\n# still code\n```",
			want:  "```\n# still code\n```",
		},
		{
			name:  "nested hash levels outside a fence keep working",
			input: "## Sub\n### Deeper",
			want:  "=== Sub\n==== Deeper",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertMarkdownHeadersToAsciiDoc(tt.input); got != tt.want {
				t.Errorf("ConvertMarkdownHeadersToAsciiDoc()\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// A callout legend written in hash-comment style (`# 1 - text`) sits outside the
// code block it annotates. It is a callout, not a heading — test/schema.graphql
// pairs it with the (1), // 1 and /* 1 */ styles for the same list.
func TestConvertMarkdownHeadersToAsciiDocKeepsCalloutLegends(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "dash separator",
			input: "# 1 - Always validate user input first",
			want:  "<1> Always validate user input first",
		},
		{
			name:  "no separator",
			input: "# 2 Check the character limit",
			want:  "<2> Check the character limit",
		},
		{
			name:  "multi-digit",
			input: "# 12 - Twelfth note",
			want:  "<12> Twelfth note",
		},
		{
			name:  "a heading that merely starts with a word is untouched",
			input: "# Overview of 1 thing",
			want:  "== Overview of 1 thing",
		},
		{
			name:  "a bare number heading stays a heading",
			input: "# 1",
			want:  "== 1",
		},
		{
			name:  "paren style",
			input: "(3) Use the domain model to create the tweet",
			want:  "<3> Use the domain model to create the tweet",
		},
		{
			name:  "block comment style",
			input: "/* 4 */ Get the authenticated user context",
			want:  "<4> Get the authenticated user context",
		},
		{
			name:  "a parenthesised number mid-sentence is not a legend",
			input: "Returns the first (1) matching record.",
			want:  "Returns the first (1) matching record.",
		},
		{
			name:  "legends inside a fence are left to the code block processor",
			input: "```python\n(3) not a legend\n```",
			want:  "```python\n(3) not a legend\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertMarkdownHeadersToAsciiDoc(tt.input); got != tt.want {
				t.Errorf("ConvertMarkdownHeadersToAsciiDoc()\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

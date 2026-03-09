package parser

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Pre-compiled regex patterns for utilities
var (
	// Description and code block patterns
	reDescriptionBlock   = regexp.MustCompile(`(?s)"""(.*?)"""`)
	reAsciiDocCodeBlock  = regexp.MustCompile(`(?s)\[source[^\]]*\][^\n]*\n\s*----[^\n]*\n.*?\n\s*----`)
	reMarkdownCodeInline = regexp.MustCompile("(?s)```[^`]*```")
)

// NormalizeIndentation removes common leading whitespace from all lines in a description
// This handles GraphQL triple-quoted strings that often have indentation
func NormalizeIndentation(description string) string {
	if description == "" {
		return description
	}

	lines := strings.Split(description, "\n")
	if len(lines) == 0 {
		return description
	}

	// Find the minimum indentation (excluding empty lines)
	minIndent := -1
	for _, line := range lines {
		if len(strings.TrimSpace(line)) == 0 {
			continue // Skip empty lines
		}

		// Count leading spaces/tabs
		indent := 0
		for _, char := range line {
			if char == ' ' || char == '\t' {
				indent++
			} else {
				break
			}
		}

		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	// If no indentation found, return as-is
	if minIndent <= 0 {
		return description
	}

	// Remove the common indentation from all lines
	var result []string
	for _, line := range lines {
		if len(strings.TrimSpace(line)) == 0 {
			result = append(result, "") // Preserve empty lines
		} else if len(line) > minIndent {
			result = append(result, line[minIndent:])
		} else {
			result = append(result, strings.TrimSpace(line))
		}
	}

	// Also trim leading and trailing empty lines
	// Trim leading empty lines
	for len(result) > 0 && strings.TrimSpace(result[0]) == "" {
		result = result[1:]
	}

	// Trim trailing empty lines
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	return strings.Join(result, "\n")
}

// CamelToSnake converts CamelCase to snake_case
func CamelToSnake(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result = append(result, '_')
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}

// CleanDescription removes lines starting with skipCharacter, except for AsciiDoc code block delimiters
func CleanDescription(text string, skipCharacter string) string {
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Special case: preserve AsciiDoc code block delimiters
		if trimmed == "----" {
			result = append(result, line)
			continue
		}

		// Skip lines starting with the skip character (both dash and asterisk)
		if !strings.HasPrefix(trimmed, skipCharacter) && !strings.HasPrefix(trimmed, "* ") {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n") + "\n"
}

// StripCodeBlocksFromDescriptions removes code blocks from GraphQL triple-quoted descriptions
// to prevent the parser from treating example code as actual schema definitions.
// It replaces code blocks with a placeholder to maintain structure.
func StripCodeBlocksFromDescriptions(schemaContent string) string {
	return reDescriptionBlock.ReplaceAllStringFunc(schemaContent, func(match string) string {
		// Extract the content between triple quotes
		content := match[3 : len(match)-3]

		// Replace code blocks with placeholder text that won't be parsed as GraphQL
		cleaned := reAsciiDocCodeBlock.ReplaceAllString(content, "[CODE_BLOCK_REMOVED]")
		cleaned = reMarkdownCodeInline.ReplaceAllString(cleaned, "[CODE_BLOCK_REMOVED]")

		// Return the description with code blocks replaced
		return `"""` + cleaned + `"""`
	})
}

// ConvertDescriptionToRefNumbers converts dash and asterisk list items to numbered references
// Only converts items that look like parameter descriptions (start with backtick or _RETURNS_)
func ConvertDescriptionToRefNumbers(text string, skipNonDash bool) string {
	lines := strings.Split(text, "\n")
	var result []string
	counter := 1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for list markers and convert to numbered references
		var content string
		var isListItem bool

		if strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "-- ") {
			content = strings.TrimPrefix(trimmed, "- ")
			isListItem = true
		} else if strings.HasPrefix(trimmed, "* ") {
			content = strings.TrimPrefix(trimmed, "* ")
			isListItem = true
		}

		if isListItem {
			// Only convert to callout if it looks like a parameter description
			// (starts with backtick for param name) or is a RETURNS line
			if strings.HasPrefix(content, "`") || strings.HasPrefix(content, "_RETURNS_") {
				result = append(result, fmt.Sprintf("<%d> %s", counter, content))
				counter++
			} else if !skipNonDash {
				// Preserve regular bullet points
				result = append(result, line)
			}
		} else if !skipNonDash {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n") + "\n"
}

// RemoveFragments removes fragment definitions from schema content
// Fragments are client-side query constructs and don't belong in schema files
func RemoveFragments(schemaContent string) string {
	lines := strings.Split(schemaContent, "\n")
	var cleanedLines []string
	inFragment := false
	braceCount := 0

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check if this line starts a fragment definition
		if strings.HasPrefix(trimmedLine, "fragment ") && !inFragment {
			inFragment = true
			// Count opening braces on the same line
			braceCount = strings.Count(line, "{") - strings.Count(line, "}")
			continue
		}

		// If we're inside a fragment, skip lines until fragment is complete
		if inFragment {
			braceCount += strings.Count(line, "{") - strings.Count(line, "}")

			// Check if fragment is complete
			if braceCount == 0 {
				inFragment = false
			}
			continue
		}

		// Not in a fragment, keep the line
		cleanedLines = append(cleanedLines, line)
	}

	return strings.Join(cleanedLines, "\n")
}

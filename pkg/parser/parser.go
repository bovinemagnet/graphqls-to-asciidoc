package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// Pre-compiled regex patterns for list item processing
var (
	reAsteriskList = regexp.MustCompile(`(^|\s)\*\s`)
	reHyphenList   = regexp.MustCompile(`(^|\s)-\s`)
)

// ProcessDescription processes GraphQL description text for AsciiDoc output
// This is the main entry point that supports both structured and unstructured descriptions
func ProcessDescription(description string) string {
	// First normalize indentation - GraphQL descriptions often have leading whitespace
	description = NormalizeIndentation(description)

	// Try to parse as structured description first
	parser := NewDescriptionParser()
	parsed := parser.ParseDescription(description)

	// If it's a structured description, process it specially
	if parsed.Structured != nil && parsed.Structured.IsStructured {
		return processStructuredDescription(parsed.Structured)
	}

	// Fall back to original processing for unstructured descriptions
	return processUnstructuredDescription(description)
}

// processUnstructuredDescription handles traditional non-structured descriptions
func processUnstructuredDescription(description string) string {
	// Convert markdown headers to AsciiDoc format FIRST
	processed := ConvertMarkdownHeadersToAsciiDoc(description)

	// Then convert markdown code blocks to AsciiDoc format
	processed = ConvertMarkdownCodeBlocks(processed)

	// Process anchors and labels before table processing to avoid conflicts with pipe characters
	processed = ProcessAnchorsAndLabels(processed)

	// Process tables (both markdown and AsciiDoc pass-through)
	processed = ProcessTables(processed)

	// Convert admonition patterns to AsciiDoc admonition blocks
	processed = ConvertAdmonitionBlocks(processed)

	// Format @deprecated directives with backticks if not already enclosed
	processed = FormatDeprecatedDirectives(processed)

	// Convert Arguments patterns to AsciiDoc format
	processed = ConvertArgumentsPatterns(processed)

	// Replace * and - with newline followed by the character, but only when they start list items
	// Use pre-compiled regex to replace asterisks only when they start list items
	processed = reAsteriskList.ReplaceAllString(processed, "${1}* ")

	// Use pre-compiled regex to replace hyphens only when they start list items
	// Match: start of line OR whitespace, followed by hyphen, followed by space
	processed = reHyphenList.ReplaceAllString(processed, "${1}* ")

	// Remove newline at start if present
	return strings.TrimPrefix(processed, "\n")
}

// processStructuredDescription handles structured descriptions with sections
func processStructuredDescription(structured *DescriptionStructure) string {
	var parts []string

	// Add overview if present
	if structured.Overview != "" {
		processed := processUnstructuredDescription(structured.Overview)
		parts = append(parts, processed)
	}

	// Add parameters section if present
	if len(structured.Parameters) > 0 {
		parts = append(parts, formatParametersSection(structured.Parameters))
	}

	// Add returns section if present
	if structured.Returns != "" {
		parts = append(parts, ".Returns", processUnstructuredDescription(structured.Returns))
	}

	// Add errors section if present
	if len(structured.Errors) > 0 {
		parts = append(parts, formatErrorsSection(structured.Errors))
	}

	// Add examples section if present
	if len(structured.Examples) > 0 {
		parts = append(parts, formatExamplesSection(structured.Examples))
	}

	// Add custom sections
	for sectionName, sectionContent := range structured.Sections {
		// Skip sections we've already processed
		if isProcessedSection(sectionName) {
			continue
		}
		// Use === for subsection headings in AsciiDoc
		parts = append(parts, fmt.Sprintf("=== %s", sectionName))
		parts = append(parts, processUnstructuredDescription(sectionContent))
	}

	// Add changelog if present
	if len(structured.Changelog) > 0 {
		parts = append(parts, formatChangelogSection(structured.Changelog))
	}

	// Add metadata annotations if present
	if len(structured.Metadata) > 0 {
		parts = append(parts, formatMetadataSection(structured.Metadata))
	}

	return strings.Join(parts, "\n\n")
}

// formatParametersSection formats parameter documentation
func formatParametersSection(params []ParameterDoc) string {
	if len(params) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, ".Parameters")

	for i := range params {
		param := &params[i]
		// Format main parameter
		paramLine := fmt.Sprintf("<%d> `%s`", i+1, param.Name)
		if param.Type != "" {
			paramLine += fmt.Sprintf(" (%s)", param.Type)
		}
		if param.Description != "" {
			paramLine += fmt.Sprintf(" - %s", param.Description)
		}
		if param.Default != "" {
			paramLine += fmt.Sprintf(" (default: %s)", param.Default)
		}
		lines = append(lines, paramLine)

		// Format sub-parameters if present
		for j := range param.SubParams {
			subParam := &param.SubParams[j]
			subLine := fmt.Sprintf("  * `%s`", subParam.Name)
			if subParam.Description != "" {
				subLine += fmt.Sprintf(" - %s", subParam.Description)
			}
			lines = append(lines, subLine)
		}
	}

	return strings.Join(lines, "\n")
}

// formatErrorsSection formats error documentation
func formatErrorsSection(errors []ErrorDoc) string {
	if len(errors) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, ".Errors")

	for _, err := range errors {
		errLine := fmt.Sprintf("* `%s`", err.Code)
		if err.Description != "" {
			errLine += fmt.Sprintf(" - %s", err.Description)
		}
		if err.When != "" {
			errLine += fmt.Sprintf(" (when: %s)", err.When)
		}
		lines = append(lines, errLine)
	}

	return strings.Join(lines, "\n")
}

// formatExamplesSection formats code examples
func formatExamplesSection(examples []Example) string {
	if len(examples) == 0 {
		return ""
	}

	var lines []string

	for _, example := range examples {
		if example.Title != "" && example.Title != "Example" {
			lines = append(lines, fmt.Sprintf(".%s", example.Title))
		} else {
			lines = append(lines, ".Example")
		}

		if example.Description != "" {
			lines = append(lines, example.Description)
		}

		// Format code block
		lang := example.Language
		if lang == "" {
			lang = "graphql"
		}
		lines = append(lines, fmt.Sprintf("[source,%s]", lang), "----", ProcessCallouts(example.Code), "----")
	}

	return strings.Join(lines, "\n\n")
}

// formatChangelogSection formats version history
func formatChangelogSection(changelog []ChangelogEntry) string {
	if len(changelog) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, ".Version History")

	for _, entry := range changelog {
		line := fmt.Sprintf("* %s %s", entry.Type, entry.Version)
		if entry.Description != "" {
			line += fmt.Sprintf(" - %s", entry.Description)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// formatMetadataSection formats metadata annotations
func formatMetadataSection(metadata map[string]string) string {
	var lines []string

	if since, ok := metadata["since"]; ok {
		lines = append(lines, fmt.Sprintf("_Since: %s_", since))
	}

	if deprecated, ok := metadata["deprecated"]; ok {
		lines = append(lines, fmt.Sprintf("_Deprecated: %s_", deprecated))
	}

	if _, ok := metadata["beta"]; ok {
		lines = append(lines, "_Beta: This feature is in beta and may change_")
	}

	if _, ok := metadata["experimental"]; ok {
		lines = append(lines, "_Experimental: This feature is experimental and subject to change_")
	}

	if _, ok := metadata["internal"]; ok {
		lines = append(lines, "_Internal: This is an internal API_")
	}

	return strings.Join(lines, "\n")
}

// isProcessedSection checks if a section name has already been processed
func isProcessedSection(name string) bool {
	processed := []string{
		"overview", "parameters", "params", "returns", "return",
		"errors", "throws", "exceptions", "examples", "example",
	}

	lowerName := strings.ToLower(name)
	for _, p := range processed {
		if lowerName == p {
			return true
		}
	}
	return false
}

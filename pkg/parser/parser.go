package parser

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/vektah/gqlparser/v2/ast"
)

// Pre-compiled regex patterns for performance optimization
var (
	// List item patterns
	reAsteriskList = regexp.MustCompile(`(^|\s)\*\s`)
	reHyphenList   = regexp.MustCompile(`(^|\s)-\s`)

	// Deprecated directive pattern
	reDeprecated = regexp.MustCompile(`@deprecated(?:\([^)]*\))?`)

	// Markdown code block pattern
	reMarkdownCodeBlock = regexp.MustCompile("(?s)```(\\w*)\n(.*?)\n```")

	// Anchor and reference patterns
	reParenNumber     = regexp.MustCompile(`\((\d+)\)`)
	reCommentNumber   = regexp.MustCompile(`(?m)//\s*(\d+)\s*$`)
	reHashNumber      = regexp.MustCompile(`(?m)#\s*(\d+)\s*$`)
	reBlockComment    = regexp.MustCompile(`/\*\s*(\d+)\s*\*/`)
	reAnchorBracket   = regexp.MustCompile(`\[#([a-zA-Z0-9_-]+)\]`)
	reAnchorLine      = regexp.MustCompile(`(?m)^\[([a-zA-Z0-9_-]+)\]\s*$`)
	reRefPattern      = regexp.MustCompile(`\{ref:([a-zA-Z0-9_-]+)\}`)
	reLinkPattern     = regexp.MustCompile(`\{link:([a-zA-Z0-9_-]+)\|([^}]+)\}`)

	// Table separator pattern
	reTableSeparator = regexp.MustCompile(`^\s*\|[\s\-|:]+\|\s*$`)

	// Admonition patterns (pre-compiled for each type)
	reAdmonitionBold  = make(map[string]*regexp.Regexp)
	reAdmonitionPlain = make(map[string]*regexp.Regexp)

	// Description and code block patterns
	reDescriptionBlock   = regexp.MustCompile(`(?s)"""(.*?)"""`)
	reAsciiDocCodeBlock  = regexp.MustCompile(`(?s)\[source[^\]]*\][^\n]*\n\s*----[^\n]*\n.*?\n\s*----`)
	reMarkdownCodeInline = regexp.MustCompile("(?s)```[^`]*```")

	// Arguments patterns
	reArgumentsColon = regexp.MustCompile(`(?m)^\.Arguments:\s*$`)
	reArgumentsBold  = regexp.MustCompile(`(?m)^\*\*Arguments:\*\*\s*$`)
)

func init() {
	// Pre-compile admonition patterns for each type
	admonitionTypes := []string{"NOTE", "TIP", "IMPORTANT", "WARNING", "CAUTION"}
	for _, admonType := range admonitionTypes {
		reAdmonitionBold[admonType] = regexp.MustCompile(fmt.Sprintf(`\*\*%s\*\*:\s*(.+)`, admonType))
		reAdmonitionPlain[admonType] = regexp.MustCompile(fmt.Sprintf(`(?m)^%s:\s*(.+)$`, admonType))
	}
}

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
		parts = append(parts, ".Returns")
		parts = append(parts, processUnstructuredDescription(structured.Returns))
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

	for i, param := range params {
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
		for _, subParam := range param.SubParams {
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
		lines = append(lines, fmt.Sprintf("[source,%s]", lang))
		lines = append(lines, "----")
		lines = append(lines, ProcessCallouts(example.Code))
		lines = append(lines, "----")
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

// ConvertMarkdownHeadersToAsciiDoc converts markdown headers to AsciiDoc format
// # -> =, ## -> ==, ### -> ===, etc.
func ConvertMarkdownHeadersToAsciiDoc(description string) string {
	lines := strings.Split(description, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this line is a markdown header
		if strings.HasPrefix(trimmed, "#") {
			// Count the number of # symbols
			level := 0
			for _, char := range trimmed {
				if char == '#' {
					level++
				} else {
					break
				}
			}

			// Extract the header text
			headerText := strings.TrimSpace(strings.TrimPrefix(trimmed, strings.Repeat("#", level)))

			// Convert to AsciiDoc format
			// Note: In AsciiDoc, = is for document title, == is for level 1, === is for level 2, etc.
			// So we need to add one more = than the number of #
			asciidocLevel := strings.Repeat("=", level+1)
			result = append(result, fmt.Sprintf("%s %s", asciidocLevel, headerText))
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// FormatDeprecatedDirectives wraps @deprecated directives in backticks if not already enclosed
func FormatDeprecatedDirectives(description string) string {
	return reDeprecated.ReplaceAllStringFunc(description, func(match string) string {
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

// ConvertMarkdownCodeBlocks converts markdown code blocks (```lang) to AsciiDoc format ([source,lang] ----)
func ConvertMarkdownCodeBlocks(description string) string {
	return reMarkdownCodeBlock.ReplaceAllStringFunc(description, func(match string) string {
		// Extract language and content from the match
		submatches := reMarkdownCodeBlock.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match // Return original if parsing fails
		}

		language := submatches[1]
		content := submatches[2]

		// Default to generic source block if no language specified
		if language == "" {
			language = "text"
		}

		// Use kotlin syntax highlighting for GraphQL as it provides better colors in AsciiDoc
		if language == "graphql" || language == "gql" {
			language = "kotlin"
		}

		// Process callouts in the content
		processedContent := ProcessCallouts(content)

		// Convert to AsciiDoc format
		return fmt.Sprintf("[source,%s]\n----\n%s\n----", language, processedContent)
	})
}

// ProcessCallouts converts various callout patterns to AsciiDoc callout syntax
func ProcessCallouts(content string) string {
	// Pattern 1: (1), (2), etc. -> <1>, <2>, etc.
	content = reParenNumber.ReplaceAllString(content, "<$1>")

	// Pattern 2: // 1, // 2, etc. -> <1>, <2>, etc. (comment-style callouts)
	content = reCommentNumber.ReplaceAllString(content, "<$1>")

	// Pattern 3: # 1, # 2, etc. -> <1>, <2>, etc. (hash comment-style callouts)
	content = reHashNumber.ReplaceAllString(content, "<$1>")

	// Pattern 4: /* 1 */, /* 2 */, etc. -> <1>, <2>, etc. (block comment-style callouts)
	content = reBlockComment.ReplaceAllString(content, "<$1>")

	return content
}

// ProcessAnchorsAndLabels converts anchor and label patterns to AsciiDoc format
func ProcessAnchorsAndLabels(content string) string {
	// Pattern 1: [#id] format -> [[id]]
	content = reAnchorBracket.ReplaceAllString(content, "[[$1]]")

	// Pattern 2: [[anchor]] format is already AsciiDoc, so preserve it
	// No processing needed for this pattern

	// Pattern 3: <<reference>> and <<reference,text>> for cross-references
	// These are already AsciiDoc format, so preserve them

	// Pattern 4: [label] format -> [[label]] (simple label to anchor conversion)
	// Only convert if it's not already a cross-reference or other AsciiDoc construct
	// Exclude admonition blocks (NOTE, TIP, IMPORTANT, WARNING, CAUTION)
	content = reAnchorLine.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the label content
		submatches := reAnchorLine.FindStringSubmatch(match)
		if len(submatches) > 1 {
			label := submatches[1]
			// Check if this is an admonition block
			admonitions := []string{"NOTE", "TIP", "IMPORTANT", "WARNING", "CAUTION"}
			for _, admon := range admonitions {
				if label == admon {
					// Don't convert admonition blocks
					return match
				}
			}
			// Convert to anchor
			return "[[" + label + "]]"
		}
		return match
	})

	// Pattern 5: Convert reference patterns like {ref:anchor} to <<anchor>>
	content = reRefPattern.ReplaceAllString(content, "<<$1>>")

	// Pattern 6: Convert reference patterns like {link:anchor|text} to <<anchor,text>>
	content = reLinkPattern.ReplaceAllString(content, "<<$1,$2>>")

	return content
}

// ProcessTables converts markdown tables to AsciiDoc format and preserves existing AsciiDoc tables
func ProcessTables(content string) string {
	// Convert markdown tables to AsciiDoc format
	content = ConvertMarkdownTables(content)

	// AsciiDoc tables (|===....|===) are already in correct format, so no conversion needed
	// They will pass through unchanged

	return content
}

// ConvertMarkdownTables converts markdown-style tables to AsciiDoc format
func ConvertMarkdownTables(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	var inTable bool
	var inAsciiDocTable bool
	var columnCount int

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if we're entering or exiting an AsciiDoc table
		if strings.Contains(trimmed, "|===") {
			inAsciiDocTable = !inAsciiDocTable
			result = append(result, line)
			continue
		}

		// If we're inside an AsciiDoc table, preserve the line as-is
		if inAsciiDocTable {
			result = append(result, line)
			continue
		}

		// Check if this line looks like a markdown table row
		if strings.Contains(trimmed, "|") && !strings.HasPrefix(trimmed, "[") {
			// Check if this is a separator line (|---|---|)
			if reTableSeparator.MatchString(trimmed) {
				// Skip separator lines in markdown tables
				continue
			}

			// This looks like a table row
			if !inTable {
				// Start new table
				result = append(result, "[options=\"header\"]")
				result = append(result, "|===")
				inTable = true
			}

			// Process the row
			cells := parseTableRow(trimmed)
			if len(cells) > 0 {
				if columnCount == 0 {
					columnCount = len(cells)
				}

				// Add the row - put all cells on one line for AsciiDoc
				rowLine := "| " + strings.Join(cells, " | ")
				result = append(result, rowLine)
			}
		} else {
			// Not a table row
			if inTable {
				// End the table
				result = append(result, "|===")
				inTable = false
				columnCount = 0

				// Add empty line after table only if the next line isn't empty
				if trimmed != "" {
					result = append(result, "")
				}
			}
			result = append(result, line)
		}
	}

	// Close table if we ended while still in one
	if inTable {
		result = append(result, "|===")
	}

	return strings.Join(result, "\n")
}

// parseTableRow extracts cell content from a markdown table row
func parseTableRow(row string) []string {
	// Remove leading and trailing pipes and whitespace
	row = strings.Trim(row, " \t|")

	if row == "" {
		return []string{}
	}

	// Split by pipe and clean up each cell
	parts := strings.Split(row, "|")
	var cells []string

	for _, part := range parts {
		cell := strings.TrimSpace(part)
		if cell != "" {
			cells = append(cells, cell)
		}
	}

	return cells
}

// ConvertAdmonitionBlocks converts admonition patterns to AsciiDoc admonition blocks
func ConvertAdmonitionBlocks(description string) string {
	// Define supported admonition types
	admonitionTypes := []string{"NOTE", "TIP", "IMPORTANT", "WARNING", "CAUTION"}

	for _, admonType := range admonitionTypes {
		// Pattern 1: **ADMONITION**: content (single line)
		patternBold := reAdmonitionBold[admonType]
		description = patternBold.ReplaceAllStringFunc(description, func(match string) string {
			submatches := patternBold.FindStringSubmatch(match)
			if len(submatches) < 2 {
				return match
			}
			content := strings.TrimSpace(submatches[1])
			return fmt.Sprintf("[%s]\n====\n%s\n====", admonType, content)
		})

		// Pattern 2: ADMONITION: content (without asterisks, single line)
		patternPlain := reAdmonitionPlain[admonType]
		description = patternPlain.ReplaceAllStringFunc(description, func(match string) string {
			submatches := patternPlain.FindStringSubmatch(match)
			if len(submatches) < 2 {
				return match
			}
			content := strings.TrimSpace(submatches[1])
			return fmt.Sprintf("[%s]\n====\n%s\n====", admonType, content)
		})
	}

	// Handle multi-line admonitions with a simpler approach
	// Process **ADMONITION** on its own line followed by content
	lines := strings.Split(description, "\n")
	var result []string
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Check if this line is an admonition marker
		var admonType string
		for _, aType := range admonitionTypes {
			if line == "**"+aType+"**" {
				admonType = aType
				break
			}
		}

		if admonType != "" {
			// Found an admonition marker, collect content until next empty line or end
			result = append(result, fmt.Sprintf("[%s]", admonType))
			result = append(result, "====")
			i++ // Move to next line

			// Collect content lines
			for i < len(lines) {
				contentLine := lines[i]
				trimmedContent := strings.TrimSpace(contentLine)

				// Stop if we hit an empty line or another admonition
				if trimmedContent == "" {
					break
				}

				// Check if this is another admonition marker
				isNextAdmonition := false
				for _, aType := range admonitionTypes {
					if trimmedContent == "**"+aType+"**" || strings.HasPrefix(trimmedContent, "**"+aType+"**:") {
						isNextAdmonition = true
						break
					}
				}

				if isNextAdmonition {
					break
				}

				result = append(result, contentLine)
				i++
			}

			result = append(result, "====")
		} else {
			result = append(result, lines[i])
			i++
		}
	}

	return strings.Join(result, "\n")
}

// ProcessTypeName converts GraphQL types to AsciiDoc cross-references
func ProcessTypeName(typeName string, definitionsMap map[string]*ast.Definition) string {
	// Handle complex type structures

	// Check for list types [Type] or [Type!]
	if strings.HasPrefix(typeName, "[") && strings.Contains(typeName, "]") {
		// Extract the inner type
		innerStart := strings.Index(typeName, "[") + 1
		innerEnd := strings.LastIndex(typeName, "]")
		innerType := typeName[innerStart:innerEnd]

		// Process the inner type recursively
		processedInner := ProcessTypeName(innerType, definitionsMap)

		// Reconstruct with processed inner type
		result := "[" + processedInner + "]"

		// Add trailing ! if present
		if strings.HasSuffix(typeName, "!") {
			result += "!"
		}

		return result
	}

	// Handle simple required types Type!
	isRequired := strings.HasSuffix(typeName, "!")
	baseTypeName := strings.TrimSuffix(typeName, "!")

	// Check if this is a custom type (exists in definitions)
	if _, exists := definitionsMap[baseTypeName]; exists {
		// Create cross-reference
		result := fmt.Sprintf("<<%s,`%s`>>", baseTypeName, baseTypeName)
		if isRequired {
			result += "!"
		}
		return result
	}

	// For built-in types, just wrap in backticks
	return "`" + typeName + "`"
}

// ProcessTypeNameForSignature converts GraphQL types to plain type names for method signatures
// This function does not create cross-references, just returns the plain type name
func ProcessTypeNameForSignature(typeName string, definitionsMap map[string]*ast.Definition) string {
	// Handle complex type structures

	// Check for list types [Type] or [Type!]
	if strings.HasPrefix(typeName, "[") && strings.Contains(typeName, "]") {
		// Extract the inner type
		innerStart := strings.Index(typeName, "[") + 1
		innerEnd := strings.LastIndex(typeName, "]")
		innerType := typeName[innerStart:innerEnd]

		// Process the inner type recursively
		processedInner := ProcessTypeNameForSignature(innerType, definitionsMap)

		// Reconstruct with processed inner type
		result := "[" + processedInner + "]"

		// Add trailing ! if present
		if strings.HasSuffix(typeName, "!") {
			result += "!"
		}

		return result
	}

	// Handle simple required types Type!
	isRequired := strings.HasSuffix(typeName, "!")
	baseTypeName := strings.TrimSuffix(typeName, "!")

	// For method signatures, just return the plain type name without cross-references
	// Check if this is a custom type (exists in definitions)
	if _, exists := definitionsMap[baseTypeName]; exists {
		// Return plain type name without cross-reference
		result := baseTypeName
		if isRequired {
			result += "!"
		}
		return result
	}

	// For built-in types, just return the plain type name
	return typeName
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

// ConvertArgumentsPatterns converts .Arguments: and **Arguments:** patterns to AsciiDoc format
func ConvertArgumentsPatterns(description string) string {
	// Convert .Arguments: to .Arguments
	description = reArgumentsColon.ReplaceAllString(description, ".Arguments")

	// Convert **Arguments:** to .Arguments
	description = reArgumentsBold.ReplaceAllString(description, ".Arguments")

	return description
}

// ConvertDashToAsterisk converts dash list items to asterisk format for main description
func ConvertDashToAsterisk(text string) string {
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Convert dash list items to asterisk format
		if strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "-- ") {
			content := strings.TrimPrefix(trimmed, "- ")
			result = append(result, "* "+content)
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
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

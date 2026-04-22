package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// Pre-compiled regex patterns for conversions
var (
	// Deprecated directive pattern
	reDeprecated = regexp.MustCompile(`@deprecated(?:\([^)]*\))?`)

	// Markdown code block pattern
	reMarkdownCodeBlock = regexp.MustCompile("(?s)```(\\w*)\n(.*?)\n```")

	// Table separator pattern
	reTableSeparator = regexp.MustCompile(`^\s*\|[\s\-|:]+\|\s*$`)

	// Admonition patterns (pre-compiled for each type)
	reAdmonitionBold  = make(map[string]*regexp.Regexp)
	reAdmonitionPlain = make(map[string]*regexp.Regexp)

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
		if len(submatches) < 3 { //nolint:mnd // regex group count
			return match // Return original if parsing fails
		}

		language := submatches[1]
		content := submatches[2]

		// Default to generic source block if no language specified
		if language == "" {
			language = "text"
		}

		// Use kotlin syntax highlighting for GraphQL as it provides better colours in AsciiDoc
		if language == langGraphQL || language == "gql" {
			language = "kotlin"
		}

		// Process callouts in the content
		processedContent := ProcessCallouts(content)

		// Convert to AsciiDoc format
		return fmt.Sprintf("[source,%s]\n----\n%s\n----", language, processedContent)
	})
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
				result = append(result, "[options=\"header\"]", "|===")
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
			if len(submatches) < 2 { //nolint:mnd // regex group count
				return match
			}
			content := strings.TrimSpace(submatches[1])
			return fmt.Sprintf("[%s]\n====\n%s\n====", admonType, content)
		})

		// Pattern 2: ADMONITION: content (without asterisks, single line)
		patternPlain := reAdmonitionPlain[admonType]
		description = patternPlain.ReplaceAllStringFunc(description, func(match string) string {
			submatches := patternPlain.FindStringSubmatch(match)
			if len(submatches) < 2 { //nolint:mnd // regex group count
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
			result = append(result, fmt.Sprintf("[%s]", admonType), "====")
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

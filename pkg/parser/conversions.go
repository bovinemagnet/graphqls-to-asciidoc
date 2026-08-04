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

	// AsciiDoc include tag written as a hash comment inside a description:
	// "# tag::NAME[]" / "# end::NAME[]".
	reHashIncludeTag = regexp.MustCompile(`^#\s*((?:tag|end)::\S*\[\])\s*$`)

	// Callout legend lines: the entries listed under a code block that explain
	// each callout, written in comment style. An optional separator between the
	// number and the text is consumed. Text after the number is required, so a
	// bare "# 1" stays a header and a "(1)" mid-sentence is left alone.
	reCalloutLegends = []*regexp.Regexp{
		regexp.MustCompile(`^#\s*(\d+)\s*[-*.):]?\s+(\S.*)$`),        // # 1 - text
		regexp.MustCompile(`^\((\d+)\)\s*[-*.:]?\s+(\S.*)$`),         // (1) text
		regexp.MustCompile(`^/\*\s*(\d+)\s*\*/\s*[-*.:]?\s+(\S.*)$`), // /* 1 */ text
	}

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
	admonitionTypes := admonitionNames()
	for _, admonType := range admonitionTypes {
		reAdmonitionBold[admonType] = regexp.MustCompile(fmt.Sprintf(`\*\*%s\*\*:\s*(.+)`, admonType))
		reAdmonitionPlain[admonType] = regexp.MustCompile(fmt.Sprintf(`(?m)^%s:\s*(.+)$`, admonType))
	}
}

// calloutLegend renders line as an AsciiDoc callout legend if it is one of the
// supported comment-style legend forms.
func calloutLegend(line string) (string, bool) {
	for _, re := range reCalloutLegends {
		if m := re.FindStringSubmatch(line); m != nil {
			return fmt.Sprintf("<%s> %s", m[1], m[2]), true
		}
	}
	return "", false
}

// ConvertMarkdownHeadersToAsciiDoc converts markdown headers to AsciiDoc format
// # -> =, ## -> ==, ### -> ===, etc.
//
// Lines inside a fenced code block are left alone: a leading # there is source
// code (a Python or shell comment), not a header. A hash-style callout legend
// (# 1 - text) is converted to an AsciiDoc callout rather than a header, matching
// the (1), // 1 and /* 1 */ styles handled by ProcessCallouts.
func ConvertMarkdownHeadersToAsciiDoc(description string) string {
	lines := strings.Split(description, "\n")
	var result []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			result = append(result, line)
			continue
		}

		if inCodeBlock {
			result = append(result, line)
			continue
		}

		if tag := reHashIncludeTag.FindStringSubmatch(trimmed); tag != nil {
			result = append(result, "// "+tag[1])
			continue
		}

		if legend, ok := calloutLegend(trimmed); ok {
			result = append(result, legend)
			continue
		}

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
	description = convertInlineAdmonitions(description)
	return convertBlockAdmonitions(description)
}

// convertInlineAdmonitions rewrites single-line "**NOTE**: text" and
// "NOTE: text" forms as AsciiDoc admonition blocks.
func convertInlineAdmonitions(description string) string {
	for _, admonType := range admonitionNames() {
		for _, pattern := range []*regexp.Regexp{reAdmonitionBold[admonType], reAdmonitionPlain[admonType]} {
			description = pattern.ReplaceAllStringFunc(description, func(match string) string {
				submatches := pattern.FindStringSubmatch(match)
				if len(submatches) < 2 { //nolint:mnd // regex group count
					return match
				}
				return fmt.Sprintf("[%s]\n====\n%s\n====", admonType, strings.TrimSpace(submatches[1]))
			})
		}
	}
	return description
}

// convertBlockAdmonitions rewrites a "**NOTE**" marker on its own line, with the
// following lines as its content, as an AsciiDoc admonition block. The block
// ends at a blank line or the next marker.
func convertBlockAdmonitions(description string) string {
	lines := strings.Split(description, "\n")
	var result []string

	for i := 0; i < len(lines); {
		admonType := admonitionMarker(strings.TrimSpace(lines[i]))
		if admonType == "" {
			result = append(result, lines[i])
			i++
			continue
		}

		result = append(result, fmt.Sprintf("[%s]", admonType), "====")
		i++
		for i < len(lines) {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" || startsAdmonition(trimmed) {
				break
			}
			result = append(result, lines[i])
			i++
		}
		result = append(result, "====")
	}

	return strings.Join(result, "\n")
}

// admonitionMarker returns the admonition type when line is exactly a bold
// marker such as "**NOTE**", or "" when it is not one.
func admonitionMarker(line string) string {
	for _, aType := range admonitionNames() {
		if line == "**"+aType+"**" {
			return aType
		}
	}
	return ""
}

// startsAdmonition reports whether line opens a new admonition, either as a
// bare marker or as "**NOTE**: text".
func startsAdmonition(line string) bool {
	for _, aType := range admonitionNames() {
		if line == "**"+aType+"**" || strings.HasPrefix(line, "**"+aType+"**:") {
			return true
		}
	}
	return false
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

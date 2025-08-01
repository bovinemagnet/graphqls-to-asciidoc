package parser

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/vektah/gqlparser/v2/ast"
)

// ProcessDescription processes GraphQL description text for AsciiDoc output
func ProcessDescription(description string) string {
	// First convert markdown code blocks to AsciiDoc format
	processed := ConvertMarkdownCodeBlocks(description)

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
	// Use regex to replace asterisks only when they start list items
	reAsterisk := regexp.MustCompile(`(^|\s)\*\s`)
	processed = reAsterisk.ReplaceAllString(processed, "${1}* ")

	// Use regex to replace hyphens only when they start list items
	// Match: start of line OR whitespace, followed by hyphen, followed by space
	reHyphen := regexp.MustCompile(`(^|\s)-\s`)
	processed = reHyphen.ReplaceAllString(processed, "${1}* ")

	// Remove newline at start if present
	return strings.TrimPrefix(processed, "\n")
}

// FormatDeprecatedDirectives wraps @deprecated directives in backticks if not already enclosed
func FormatDeprecatedDirectives(description string) string {
	// Regex to match @deprecated directives with optional arguments
	re := regexp.MustCompile(`@deprecated(?:\([^)]*\))?`)

	return re.ReplaceAllStringFunc(description, func(match string) string {
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
	// Regex to match markdown code blocks: ```language\ncontent\n```
	// Supports optional language specification
	re := regexp.MustCompile("(?s)```(\\w*)\n(.*?)\n```")

	return re.ReplaceAllStringFunc(description, func(match string) string {
		// Extract language and content from the match
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match // Return original if parsing fails
		}

		language := submatches[1]
		content := submatches[2]

		// Default to generic source block if no language specified
		if language == "" {
			language = "text"
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
	pattern1 := regexp.MustCompile(`\((\d+)\)`)
	content = pattern1.ReplaceAllString(content, "<$1>")

	// Pattern 2: // 1, // 2, etc. -> <1>, <2>, etc. (comment-style callouts)
	pattern2 := regexp.MustCompile(`(?m)//\s*(\d+)\s*$`)
	content = pattern2.ReplaceAllString(content, "<$1>")

	// Pattern 3: # 1, # 2, etc. -> <1>, <2>, etc. (hash comment-style callouts)
	pattern3 := regexp.MustCompile(`(?m)#\s*(\d+)\s*$`)
	content = pattern3.ReplaceAllString(content, "<$1>")

	// Pattern 4: /* 1 */, /* 2 */, etc. -> <1>, <2>, etc. (block comment-style callouts)
	pattern4 := regexp.MustCompile(`/\*\s*(\d+)\s*\*/`)
	content = pattern4.ReplaceAllString(content, "<$1>")

	return content
}

// ProcessAnchorsAndLabels converts anchor and label patterns to AsciiDoc format
func ProcessAnchorsAndLabels(content string) string {
	// Pattern 1: [#id] format -> [[id]]
	pattern1 := regexp.MustCompile(`\[#([a-zA-Z0-9_-]+)\]`)
	content = pattern1.ReplaceAllString(content, "[[$1]]")

	// Pattern 2: [[anchor]] format is already AsciiDoc, so preserve it
	// No processing needed for this pattern

	// Pattern 3: <<reference>> and <<reference,text>> for cross-references
	// These are already AsciiDoc format, so preserve them

	// Pattern 4: [label] format -> [[label]] (simple label to anchor conversion)
	// Only convert if it's not already a cross-reference or other AsciiDoc construct
	pattern4 := regexp.MustCompile(`(?m)^\[([a-zA-Z0-9_-]+)\]\s*$`)
	content = pattern4.ReplaceAllString(content, "[[$1]]")

	// Pattern 5: Convert reference patterns like {ref:anchor} to <<anchor>>
	pattern5 := regexp.MustCompile(`\{ref:([a-zA-Z0-9_-]+)\}`)
	content = pattern5.ReplaceAllString(content, "<<$1>>")

	// Pattern 6: Convert reference patterns like {link:anchor|text} to <<anchor,text>>
	pattern6 := regexp.MustCompile(`\{link:([a-zA-Z0-9_-]+)\|([^}]+)\}`)
	content = pattern6.ReplaceAllString(content, "<<$1,$2>>")

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
			if regexp.MustCompile(`^\s*\|[\s\-|:]+\|\s*$`).MatchString(trimmed) {
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
		pattern1 := regexp.MustCompile(fmt.Sprintf(`\*\*%s\*\*:\s*(.+)`, admonType))
		description = pattern1.ReplaceAllStringFunc(description, func(match string) string {
			submatches := pattern1.FindStringSubmatch(match)
			if len(submatches) < 2 {
				return match
			}
			content := strings.TrimSpace(submatches[1])
			return fmt.Sprintf("[%s]\n====\n%s\n====", admonType, content)
		})

		// Pattern 2: ADMONITION: content (without asterisks, single line)
		pattern2 := regexp.MustCompile(fmt.Sprintf(`(?m)^%s:\s*(.+)$`, admonType))
		description = pattern2.ReplaceAllStringFunc(description, func(match string) string {
			submatches := pattern2.FindStringSubmatch(match)
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

// ConvertDescriptionToRefNumbers converts dash and asterisk list items to numbered references
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
			result = append(result, fmt.Sprintf("<%d> %s", counter, content))
			counter++
		} else if !skipNonDash {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n") + "\n"
}

// ConvertArgumentsPatterns converts .Arguments: and **Arguments:** patterns to AsciiDoc format
func ConvertArgumentsPatterns(description string) string {
	// Convert .Arguments: to .Arguments
	re := regexp.MustCompile(`(?m)^\.Arguments:\s*$`)
	description = re.ReplaceAllString(description, ".Arguments")

	// Convert **Arguments:** to .Arguments
	re = regexp.MustCompile(`(?m)^\*\*Arguments:\*\*\s*$`)
	description = re.ReplaceAllString(description, ".Arguments")

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

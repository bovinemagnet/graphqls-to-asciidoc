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

	// Convert admonition patterns to AsciiDoc admonition blocks
	processed = ConvertAdmonitionBlocks(processed)

	// Format @deprecated directives with backticks if not already enclosed
	processed = FormatDeprecatedDirectives(processed)

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

		// Convert to AsciiDoc format
		return fmt.Sprintf("[source,%s]\n----\n%s\n----", language, content)
	})
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
		
		// Skip lines starting with the skip character
		if !strings.HasPrefix(trimmed, skipCharacter) {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n") + "\n"
}

// ConvertDescriptionToRefNumbers converts dash list items to numbered references
func ConvertDescriptionToRefNumbers(text string, skipNonDash bool) string {
	lines := strings.Split(text, "\n")
	var result []string
	counter := 1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Check for single dash at start (not double dash)
		if strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "-- ") {
			// Convert to numbered reference
			content := strings.TrimPrefix(trimmed, "- ")
			result = append(result, fmt.Sprintf("<%d> %s", counter, content))
			counter++
		} else if !skipNonDash {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n") + "\n"
}
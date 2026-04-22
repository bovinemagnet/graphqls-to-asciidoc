package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

// Pre-compiled regex patterns for formatting
var (
	// Anchor and reference patterns
	reParenNumber   = regexp.MustCompile(`\((\d+)\)`)
	reCommentNumber = regexp.MustCompile(`(?m)//\s*(\d+)\s*$`)
	reHashNumber    = regexp.MustCompile(`(?m)#\s*(\d+)\s*$`)
	reBlockComment  = regexp.MustCompile(`/\*\s*(\d+)\s*\*/`)
	reAnchorBracket = regexp.MustCompile(`\[#([a-zA-Z0-9_-]+)\]`)
	reAnchorLine    = regexp.MustCompile(`(?m)^\[([a-zA-Z0-9_-]+)\]\s*$`)
	reRefPattern    = regexp.MustCompile(`\{ref:([a-zA-Z0-9_-]+)\}`)
	reLinkPattern   = regexp.MustCompile(`\{link:([a-zA-Z0-9_-]+)\|([^}]+)\}`)
)

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

// CrossReferenceTypeNames scans text for type names present in definitionsMap and
// wraps them in AsciiDoc cross-references <<Type,`Type`>>.
// Handles both backtick-wrapped (`Type`) and bare type names (whole-word match only).
// Skips names already inside <<...>> cross-references.
func CrossReferenceTypeNames(text string, definitionsMap map[string]*ast.Definition) string {
	if text == "" || len(definitionsMap) == 0 {
		return text
	}

	result := text

	for typeName := range definitionsMap {
		xref := fmt.Sprintf("<<%s,`%s`>>", typeName, typeName)

		// First pass: replace backtick-wrapped type names: `Type` → <<Type,`Type`>>
		// But skip if already inside a cross-reference (preceded by comma)
		backtickPattern := regexp.MustCompile("(?:^|[^,])`" + regexp.QuoteMeta(typeName) + "`")
		result = backtickPattern.ReplaceAllStringFunc(result, func(match string) string {
			// Preserve any leading character that isn't the backtick; the
			// rest of `match` (the backticked type name) is wholly replaced
			// by `xref`, so we don't need to reference it again.
			prefix := ""
			if !strings.HasPrefix(match, "`") {
				prefix = match[:1]
			}
			return prefix + xref
		})

		// Second pass: replace bare type names as whole words
		// Skip if already inside <<...>> cross-references or backticks
		barePattern := regexp.MustCompile(`(?:^|[^<` + "`" + `a-zA-Z])` + regexp.QuoteMeta(typeName) + `(?:[^>` + "`" + `a-zA-Z]|$)`)
		result = barePattern.ReplaceAllStringFunc(result, func(match string) string {
			// Extract prefix and suffix characters around the type name
			idx := strings.Index(match, typeName)
			prefix := match[:idx]
			suffix := match[idx+len(typeName):]
			return prefix + xref + suffix
		})
	}

	return result
}

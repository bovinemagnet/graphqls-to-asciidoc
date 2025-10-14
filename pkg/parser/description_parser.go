package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// DescriptionParser handles parsing of structured and unstructured descriptions
type DescriptionParser struct {
	enableStructured bool
	enableMetrics    bool
}

// NewDescriptionParser creates a new description parser
func NewDescriptionParser() *DescriptionParser {
	return &DescriptionParser{
		enableStructured: true,
		enableMetrics:    true,
	}
}

// ParseDescription parses a description string into structured components
func (dp *DescriptionParser) ParseDescription(description string) *ParsedDescription {
	if description == "" {
		return &ParsedDescription{
			Unstructured: "",
			Metrics:      dp.calculateMetrics(nil, ""),
		}
	}

	// Check if description appears to be structured
	if dp.isStructuredDescription(description) && dp.enableStructured {
		structured := dp.parseStructuredDescription(description)
		metrics := dp.calculateMetrics(structured, description)
		return &ParsedDescription{
			Structured: structured,
			Metrics:    metrics,
		}
	}

	// Fallback to unstructured
	return &ParsedDescription{
		Unstructured: description,
		Metrics:      dp.calculateMetrics(nil, description),
	}
}

// isStructuredDescription checks if a description uses structured format
func (dp *DescriptionParser) isStructuredDescription(description string) bool {
	// Check for common structured patterns
	patterns := []string{
		`(?m)^##\s+\w+`,          // Markdown headers
		`@param\s+\w+`,           // JSDoc parameters
		`@returns?\s+`,           // JSDoc returns
		`@throws?\s+`,            // JSDoc throws
		`@example\s*`,            // JSDoc examples
		`(?m)^###?\s+Overview`,   // Overview section
		`(?m)^###?\s+Parameters`, // Parameters section
		`(?m)^###?\s+Examples?`,  // Examples section
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, description); matched {
			return true
		}
	}
	return false
}

// parseStructuredDescription parses a structured description into components
func (dp *DescriptionParser) parseStructuredDescription(description string) *DescriptionStructure {
	structure := &DescriptionStructure{
		RawDescription: description,
		Parameters:     []ParameterDoc{},
		Errors:         []ErrorDoc{},
		Examples:       []Example{},
		Changelog:      []ChangelogEntry{},
		Metadata:       make(map[string]string),
		Sections:       make(map[string]string),
		IsStructured:   true,
	}

	// Parse sections
	dp.parseSections(description, structure)

	// Parse JSDoc annotations
	dp.parseJSDocAnnotations(description, structure)

	// Parse changelog entries
	dp.parseChangelog(description, structure)

	// Parse examples
	dp.parseExamples(description, structure)

	// Extract metadata
	dp.parseMetadata(description, structure)

	return structure
}

// parseSections extracts markdown-style sections from the description
func (dp *DescriptionParser) parseSections(description string, structure *DescriptionStructure) {
	// Split description into lines for line-by-line processing
	lines := strings.Split(description, "\n")
	currentSection := ""
	currentContent := []string{}
	inSection := false
	overviewContent := []string{}

	for i, line := range lines {
		// Check if this is a section header (## or ###)
		if matched, _ := regexp.MatchString(`^###?\s+.+`, line); matched {
			// Save previous section if any
			if currentSection != "" {
				sectionContent := strings.TrimSpace(strings.Join(currentContent, "\n"))
				switch strings.ToLower(currentSection) {
				case "overview":
					structure.Overview = sectionContent
				case "returns", "return":
					structure.Returns = sectionContent
				default:
					// Store all sections including Parameters, Errors, Examples etc
					// They'll be parsed separately later but we keep them here too
					structure.Sections[currentSection] = sectionContent
				}
			}

			// Start new section - handle both ## and ### headers
			sectionName := strings.TrimSpace(line)
			sectionName = strings.TrimPrefix(sectionName, "###")
			sectionName = strings.TrimPrefix(sectionName, "##")
			sectionName = strings.TrimSpace(sectionName)
			currentSection = sectionName
			currentContent = []string{}
			inSection = true
		} else if inSection {
			// Add line to current section
			currentContent = append(currentContent, line)
		} else {
			// Before any section - this is overview content
			overviewContent = append(overviewContent, line)
		}

		// Check if we're at the end
		if i == len(lines)-1 && currentSection != "" {
			// Save the last section
			sectionContent := strings.TrimSpace(strings.Join(currentContent, "\n"))
			switch strings.ToLower(currentSection) {
			case "overview":
				structure.Overview = sectionContent
			case "returns", "return":
				structure.Returns = sectionContent
			default:
				structure.Sections[currentSection] = sectionContent
			}
		}
	}

	// If no overview section was found, use content before first section
	if structure.Overview == "" && len(overviewContent) > 0 {
		overview := strings.TrimSpace(strings.Join(overviewContent, "\n"))
		// Remove JSDoc annotations from overview
		overview = regexp.MustCompile(`(?m)^@\w+.*$`).ReplaceAllString(overview, "")
		structure.Overview = strings.TrimSpace(overview)
	}
}

// parseJSDocAnnotations extracts JSDoc-style annotations
func (dp *DescriptionParser) parseJSDocAnnotations(description string, structure *DescriptionStructure) {
	// Parse @param annotations - handle both dot notation and without
	// First pattern: @param name.subname - description
	// Second pattern: @param name - description
	lines := strings.Split(description, "\n")
	paramMap := make(map[string]*ParameterDoc)
	paramOrder := []string{} // Keep track of order

	for _, line := range lines {
		// Check for @param annotations
		if strings.Contains(line, "@param") {
			// Try to match nested parameter first (with dot notation)
			nestedPattern := regexp.MustCompile(`@param\s+(\S+)\.(\S+)\s*-?\s*(.*)`)
			if match := nestedPattern.FindStringSubmatch(line); len(match) >= 4 {
				paramName := match[1]
				subParam := match[2]
				paramDesc := strings.TrimSpace(match[3])

				// Ensure parent exists
				if _, exists := paramMap[paramName]; !exists {
					paramMap[paramName] = &ParameterDoc{
						Name:        paramName,
						Description: "",
						SubParams:   []ParameterDoc{},
					}
					paramOrder = append(paramOrder, paramName)
				}

				// Add sub-parameter
				paramMap[paramName].SubParams = append(paramMap[paramName].SubParams, ParameterDoc{
					Name:        subParam,
					Description: paramDesc,
				})
			} else {
				// Try simple parameter pattern
				simplePattern := regexp.MustCompile(`@param\s+(\S+)\s*-?\s*(.*)`)
				if match := simplePattern.FindStringSubmatch(line); len(match) >= 3 {
					paramName := match[1]
					paramDesc := strings.TrimSpace(match[2])

					if existing, exists := paramMap[paramName]; exists {
						// Update description if empty
						if existing.Description == "" {
							existing.Description = paramDesc
						}
					} else {
						paramMap[paramName] = &ParameterDoc{
							Name:        paramName,
							Description: paramDesc,
							SubParams:   []ParameterDoc{},
						}
						paramOrder = append(paramOrder, paramName)
					}
				}
			}
		}
	}

	// Convert map to slice maintaining order
	for _, paramName := range paramOrder {
		if param, exists := paramMap[paramName]; exists {
			structure.Parameters = append(structure.Parameters, *param)
		}
	}

	// Parse @returns annotation
	returnsPattern := regexp.MustCompile(`@returns?\s+(.*)`)
	returnsMatch := returnsPattern.FindStringSubmatch(description)
	if len(returnsMatch) > 1 {
		structure.Returns = strings.TrimSpace(returnsMatch[1])
	}

	// Parse @throws annotations
	throwsPattern := regexp.MustCompile(`@throws?\s+(\S+)\s*-?\s*(.*)`)
	throwsMatches := throwsPattern.FindAllStringSubmatch(description, -1)

	for _, match := range throwsMatches {
		if len(match) >= 3 {
			structure.Errors = append(structure.Errors, ErrorDoc{
				Code:        match[1],
				Description: strings.TrimSpace(match[2]),
			})
		}
	}
}

// parseChangelog extracts version annotations
func (dp *DescriptionParser) parseChangelog(description string, structure *DescriptionStructure) {
	// Pattern for version annotations - handle multiple line formats
	// Match both single line and multi-line descriptions
	lines := strings.Split(description, "\n")
	currentVersion := ""
	currentType := ""
	currentDesc := ""

	for _, line := range lines {
		// Check if this is a version annotation
		versionPattern := regexp.MustCompile(`@version\s+(add|update|deprecate|remove)\.(\S+)(?:\s+(.*))?`)
		if match := versionPattern.FindStringSubmatch(line); len(match) >= 3 {
			// Save previous version if exists
			if currentVersion != "" {
				structure.Changelog = append(structure.Changelog, ChangelogEntry{
					Type:        currentType,
					Version:     currentVersion,
					Description: strings.TrimSpace(currentDesc),
				})
			}

			// Start new version
			currentType = match[1]
			currentVersion = match[2]
			if len(match) > 3 {
				currentDesc = match[3]
			} else {
				currentDesc = ""
			}
		} else if currentVersion != "" && strings.HasPrefix(strings.TrimSpace(line), "-") {
			// This could be a continuation of the description
			currentDesc += " " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		}
	}

	// Don't forget the last one
	if currentVersion != "" {
		structure.Changelog = append(structure.Changelog, ChangelogEntry{
			Type:        currentType,
			Version:     currentVersion,
			Description: strings.TrimSpace(currentDesc),
		})
	}
}

// parseExamples extracts code examples from the description
func (dp *DescriptionParser) parseExamples(description string, structure *DescriptionStructure) {
	// Pattern for code blocks with optional title - handle various formats
	// First try to match titled examples with markdown code blocks
	codeBlockPattern := regexp.MustCompile("(?s)(###?\\s*[^\\n]*Example[^\\n]*)\\n```(\\w*)\\n(.*?)\\n```")
	matches := codeBlockPattern.FindAllStringSubmatch(description, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			example := Example{
				Title:    strings.TrimSpace(match[1]),
				Language: match[2],
				Code:     match[3],
			}
			if example.Language == "" {
				example.Language = "graphql"
			}
			// Clean up title - remove leading ###
			example.Title = strings.TrimPrefix(example.Title, "###")
			example.Title = strings.TrimPrefix(example.Title, "##")
			example.Title = strings.TrimSpace(example.Title)
			structure.Examples = append(structure.Examples, example)
		}
	}

	// Also try to match code blocks without Example in title but in Examples section
	if section, exists := structure.Sections["Examples"]; exists {
		// Look for code blocks in the Examples section
		sectionCodePattern := regexp.MustCompile("(?s)```(\\w*)\\n(.*?)\\n```")
		sectionMatches := sectionCodePattern.FindAllStringSubmatch(section, -1)

		for i, match := range sectionMatches {
			if len(match) >= 3 {
				// Check if we haven't already added this example
				alreadyAdded := false
				for _, ex := range structure.Examples {
					if ex.Code == match[2] {
						alreadyAdded = true
						break
					}
				}

				if !alreadyAdded {
					lang := match[1]
					if lang == "" {
						lang = "graphql"
					}
					structure.Examples = append(structure.Examples, Example{
						Title:    fmt.Sprintf("Example %d", i+1),
						Language: lang,
						Code:     match[2],
					})
				}
			}
		}
	}

	// Also parse @example annotations
	examplePattern := regexp.MustCompile(`@example\s*\n?(.*)`)
	exampleMatches := examplePattern.FindAllStringSubmatch(description, -1)

	for _, match := range exampleMatches {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			structure.Examples = append(structure.Examples, Example{
				Title:    "Example",
				Code:     strings.TrimSpace(match[1]),
				Language: "graphql",
			})
		}
	}
}

// parseMetadata extracts metadata annotations
func (dp *DescriptionParser) parseMetadata(description string, structure *DescriptionStructure) {
	// Parse @since
	if match := regexp.MustCompile(`@since\s+(\S+)`).FindStringSubmatch(description); len(match) > 1 {
		structure.Metadata["since"] = match[1]
	}

	// Parse @deprecated
	if match := regexp.MustCompile(`@deprecated\s+(.+)`).FindStringSubmatch(description); len(match) > 1 {
		structure.Metadata["deprecated"] = match[1]
	}

	// Parse @beta
	if regexp.MustCompile(`@beta\b`).MatchString(description) {
		structure.Metadata["beta"] = "true"
	}

	// Parse @experimental
	if regexp.MustCompile(`@experimental\b`).MatchString(description) {
		structure.Metadata["experimental"] = "true"
	}

	// Parse @internal
	if regexp.MustCompile(`@internal\b`).MatchString(description) {
		structure.Metadata["internal"] = "true"
	}
}

// calculateMetrics calculates quality metrics for the description
func (dp *DescriptionParser) calculateMetrics(structure *DescriptionStructure, rawDescription string) *DescriptionMetrics {
	if !dp.enableMetrics {
		return nil
	}

	metrics := &DescriptionMetrics{}

	if structure != nil {
		// Calculate from structured description
		metrics.HasOverview = structure.Overview != ""
		metrics.HasChangelog = len(structure.Changelog) > 0
		metrics.HasExamples = len(structure.Examples) > 0
		metrics.HasParameters = len(structure.Parameters) > 0
		metrics.HasReturns = structure.Returns != ""
		metrics.HasErrors = len(structure.Errors) > 0

		metrics.Sections = len(structure.Sections)
		metrics.Examples = len(structure.Examples)
		metrics.Parameters = len(structure.Parameters)

		// Calculate word count
		allText := structure.Overview + structure.Returns
		for _, param := range structure.Parameters {
			allText += " " + param.Description
		}
		for _, err := range structure.Errors {
			allText += " " + err.Description
		}
		metrics.WordCount = len(strings.Fields(allText))
	} else if rawDescription != "" {
		// Calculate from raw description
		metrics.WordCount = len(strings.Fields(rawDescription))
	}

	// Calculate completeness score
	completeness := 0.0
	factors := 0.0

	if metrics.HasOverview {
		completeness += 0.3
	}
	factors += 0.3

	if metrics.HasParameters {
		completeness += 0.2
	}
	factors += 0.2

	if metrics.HasReturns {
		completeness += 0.2
	}
	factors += 0.2

	if metrics.HasExamples {
		completeness += 0.15
	}
	factors += 0.15

	if metrics.HasErrors {
		completeness += 0.15
	}
	factors += 0.15

	if factors > 0 {
		metrics.Completeness = completeness / factors
	}

	// Determine complexity
	if metrics.WordCount < 50 {
		metrics.Complexity = "simple"
	} else if metrics.WordCount < 200 {
		metrics.Complexity = "moderate"
	} else {
		metrics.Complexity = "complex"
	}

	return metrics
}

// ExtractParameterType attempts to extract type information from parameter description
func (dp *DescriptionParser) ExtractParameterType(description string) (paramType string, cleanDesc string) {
	// Pattern to match type annotations like (String), [String], {String}, <String>
	typePattern := regexp.MustCompile(`^\s*[\(\[\{<]([^\)\]\}>]+)[\)\]\}>]\s*(.*)`)
	if match := typePattern.FindStringSubmatch(description); len(match) > 2 {
		return match[1], strings.TrimSpace(match[2])
	}
	return "", description
}

// ExtractDefault extracts default value from parameter description
func (dp *DescriptionParser) ExtractDefault(description string) (defaultValue string, cleanDesc string) {
	// Pattern to match default value annotations - more specific patterns
	// Handle various formats: (default: X), default: X, (default X)
	patterns := []string{
		`\(default:\s*([^)]+)\)`, // (default: value)
		`\(default\s+([^)]+)\)`,  // (default value)
		`\bdefault:\s*(\S+)`,     // default: value
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		if match := re.FindStringSubmatch(description); len(match) > 1 {
			// Extract the default value
			defaultValue = strings.TrimSpace(match[1])

			// Remove the matched part from the description
			cleanDesc = re.ReplaceAllString(description, "")
			cleanDesc = strings.TrimSpace(cleanDesc)

			// Clean up any extra spaces
			cleanDesc = regexp.MustCompile(`\s+`).ReplaceAllString(cleanDesc, " ")

			return defaultValue, cleanDesc
		}
	}

	return "", description
}

// ExtractFirstSentence extracts the first sentence from a description up to the first full stop.
// It cleans the description first by removing special markers and extra whitespace.
func ExtractFirstSentence(description string) string {
	if description == "" {
		return ""
	}

	// Remove common markers like INTERNAL, JDR internal, etc.
	cleaned := regexp.MustCompile(`(?i)^\s*(\*\*INTERNAL\*\*|INTERNAL|JDR\s+internal)\s*:?\s*`).ReplaceAllString(description, "")

	// Remove asciidoc anchor markers like [#anchor-name] or [anchor-name]
	cleaned = regexp.MustCompile(`(?m)^\s*\[[#]?[^\]]+\]\s*\n?`).ReplaceAllString(cleaned, "")

	// Remove leading/trailing whitespace
	cleaned = strings.TrimSpace(cleaned)

	// Get only the first line/paragraph (split by double newlines or single newlines)
	// This prevents getting multi-paragraph content
	lines := strings.Split(cleaned, "\n")
	firstNonEmptyLine := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			firstNonEmptyLine = trimmed
			break
		}
	}

	if firstNonEmptyLine == "" {
		return ""
	}

	// Find the first full stop followed by a space, newline, or end of string
	// This pattern looks for a period followed by whitespace or end of text
	// and captures everything before it
	pattern := regexp.MustCompile(`^([^.]+\.)\s`)
	if match := pattern.FindStringSubmatch(firstNonEmptyLine); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	// If no period with space after it, check for period at end of string
	if strings.Contains(firstNonEmptyLine, ".") {
		parts := strings.SplitN(firstNonEmptyLine, ".", 2)
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			return strings.TrimSpace(parts[0]) + "."
		}
	}

	// If no period found at all, return the whole cleaned description
	// but limit it to a reasonable length (first 100 chars)
	if len(firstNonEmptyLine) > 100 {
		return firstNonEmptyLine[:100] + "..."
	}
	return firstNonEmptyLine
}

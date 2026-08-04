package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// reJSDocLine matches a JSDoc annotation line such as "@param x".
// Example code-block patterns: one for a heading mentioning "Example" followed
// by a fenced block, one for a bare fenced block.
var (
	reTitledExample = regexp.MustCompile("(?s)(###?\\s*[^\\n]*Example[^\\n]*)\\n```(\\w*)\\n(.*?)\\n```")
	reCodeBlock     = regexp.MustCompile("(?s)```(\\w*)\\n(.*?)\\n```")
)

// JSDoc @param forms: "@param parent.child - text" and "@param name - text".
var (
	reNestedParam = regexp.MustCompile(`@param\s+(\S+)\.(\S+)\s*-?\s*(.*)`)
	reSimpleParam = regexp.MustCompile(`@param\s+(\S+)\s*-?\s*(.*)`)
)

var reJSDocLine = regexp.MustCompile(`(?m)^@\w+.*$`)

const (
	sectionOverview   = "overview"
	sectionReturn     = "return"
	sectionReturns    = "returns"
	langGraphQL       = "graphql"
	complexitySimple  = "simple"
	metadataValueTrue = "true"

	// Weights used to score how complete a description is.
	weightOverview   = 0.3
	weightParameters = 0.2
	weightReturns    = 0.2
	weightExamples   = 0.15
	weightErrors     = 0.15

	// Description-complexity word-count thresholds.
	simpleWordLimit   = 50  // < this ⇒ "simple"
	moderateWordLimit = 200 // < this ⇒ "moderate"; otherwise "complex"

	// Maximum first-line preview length before truncating with ellipsis.
	firstLinePreviewMax = 100

	// splitOnFirstPeriod is the N for strings.SplitN when we only want the
	// first sentence plus remainder.
	splitOnFirstPeriod = 2
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
	lines := strings.Split(description, "\n")
	currentSection := ""
	currentContent := []string{}
	inSection := false
	overviewContent := []string{}

	for i, line := range lines {
		// Detect header levels: ## starts a new section, ### within an existing section is content
		isH2 := strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "### ")
		isH3 := strings.HasPrefix(line, "### ")

		switch {
		case isH2 || (isH3 && !inSection):
			storeSection(structure, currentSection, currentContent)
			currentSection = sectionHeading(line)
			currentContent = []string{}
			inSection = true
		case inSection:
			// Content of the current section, including any ### subheading.
			currentContent = append(currentContent, line)
		default:
			// Before any section - this is overview content
			overviewContent = append(overviewContent, line)
		}

		if i == len(lines)-1 {
			storeSection(structure, currentSection, currentContent)
		}
	}

	// If no overview section was found, use content before first section
	if structure.Overview == "" && len(overviewContent) > 0 {
		overview := strings.TrimSpace(strings.Join(overviewContent, "\n"))
		// Remove JSDoc annotations from overview
		overview = reJSDocLine.ReplaceAllString(overview, "")
		structure.Overview = strings.TrimSpace(overview)
	}
}

// storeSection files a completed section's content under the right field of
// structure. A section with no name has not started yet and is ignored.
func storeSection(structure *DescriptionStructure, name string, content []string) {
	if name == "" {
		return
	}

	sectionContent := strings.TrimSpace(strings.Join(content, "\n"))
	switch strings.ToLower(name) {
	case sectionOverview:
		structure.Overview = sectionContent
	case sectionReturns, sectionReturn:
		structure.Returns = sectionContent
	default:
		// Store all sections including Parameters, Errors, Examples etc.
		// They are parsed separately later but kept here too.
		structure.Sections[name] = sectionContent
	}
}

// sectionHeading strips the markdown heading markers from a section title.
func sectionHeading(line string) string {
	name := strings.TrimSpace(line)
	name = strings.TrimPrefix(name, "###")
	name = strings.TrimPrefix(name, "##")
	return strings.TrimSpace(name)
}

// parseJSDocAnnotations extracts JSDoc-style annotations
// parseParamAnnotations collects @param annotations in declaration order,
// attaching "@param parent.child" entries to their parent parameter.
func parseParamAnnotations(description string) []ParameterDoc {
	paramMap := make(map[string]*ParameterDoc)
	paramOrder := []string{}

	ensureParent := func(name, desc string) *ParameterDoc {
		if existing, exists := paramMap[name]; exists {
			return existing
		}
		paramMap[name] = &ParameterDoc{Name: name, Description: desc, SubParams: []ParameterDoc{}}
		paramOrder = append(paramOrder, name)
		return paramMap[name]
	}

	for _, line := range strings.Split(description, "\n") {
		if !strings.Contains(line, "@param") {
			continue
		}

		// Nested parameters (dot notation) take precedence over the simple form.
		if match := reNestedParam.FindStringSubmatch(line); len(match) >= 4 { //nolint:mnd // regex group count
			parent := ensureParent(match[1], "")
			parent.SubParams = append(parent.SubParams, ParameterDoc{
				Name:        match[2],
				Description: strings.TrimSpace(match[3]),
			})
			continue
		}

		if match := reSimpleParam.FindStringSubmatch(line); len(match) >= 3 { //nolint:mnd // regex group count
			desc := strings.TrimSpace(match[2])
			if param := ensureParent(match[1], desc); param.Description == "" {
				param.Description = desc
			}
		}
	}

	params := make([]ParameterDoc, 0, len(paramOrder))
	for _, name := range paramOrder {
		if param, exists := paramMap[name]; exists {
			params = append(params, *param)
		}
	}
	return params
}

func (dp *DescriptionParser) parseJSDocAnnotations(description string, structure *DescriptionStructure) {
	structure.Parameters = append(structure.Parameters, parseParamAnnotations(description)...)

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
		if len(match) >= 3 { //nolint:mnd // regex group count
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
		if match := versionPattern.FindStringSubmatch(line); len(match) >= 3 { //nolint:mnd // regex group count (optional)
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
			if len(match) > 3 { //nolint:mnd // optional 3rd capture
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
// titledExamples collects code blocks introduced by a heading containing the
// word "Example".
func titledExamples(description string) []Example {
	var examples []Example
	for _, match := range reTitledExample.FindAllStringSubmatch(description, -1) {
		if len(match) < 4 { //nolint:mnd // regex group count
			continue
		}

		title := strings.TrimSpace(match[1])
		title = strings.TrimPrefix(title, "###")
		title = strings.TrimPrefix(title, "##")

		language := match[2]
		if language == "" {
			language = langGraphQL
		}

		examples = append(examples, Example{
			Title:    strings.TrimSpace(title),
			Language: language,
			Code:     match[3],
		})
	}
	return examples
}

// examplesSectionBlocks collects any code block inside an "Examples" section
// that titledExamples did not already pick up.
func examplesSectionBlocks(structure *DescriptionStructure) []Example {
	section, exists := structure.Sections["Examples"]
	if !exists {
		return nil
	}

	var examples []Example
	for i, match := range reCodeBlock.FindAllStringSubmatch(section, -1) {
		if len(match) < 3 { //nolint:mnd // regex group count
			continue
		}
		if containsExampleCode(structure.Examples, match[2]) {
			continue
		}

		language := match[1]
		if language == "" {
			language = langGraphQL
		}
		examples = append(examples, Example{
			Title:    fmt.Sprintf("Example %d", i+1),
			Language: language,
			Code:     match[2],
		})
	}
	return examples
}

// containsExampleCode reports whether code has already been collected.
func containsExampleCode(examples []Example, code string) bool {
	for _, ex := range examples {
		if ex.Code == code {
			return true
		}
	}
	return false
}

func (dp *DescriptionParser) parseExamples(description string, structure *DescriptionStructure) {
	structure.Examples = append(structure.Examples, titledExamples(description)...)
	structure.Examples = append(structure.Examples, examplesSectionBlocks(structure)...)

	// Also parse @example annotations
	examplePattern := regexp.MustCompile(`@example\s*\n?(.*)`)
	exampleMatches := examplePattern.FindAllStringSubmatch(description, -1)

	for _, match := range exampleMatches {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			structure.Examples = append(structure.Examples, Example{
				Title:    defaultExampleTitle,
				Code:     strings.TrimSpace(match[1]),
				Language: langGraphQL,
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
		structure.Metadata["beta"] = metadataValueTrue
	}

	// Parse @experimental
	if regexp.MustCompile(`@experimental\b`).MatchString(description) {
		structure.Metadata["experimental"] = metadataValueTrue
	}

	// Parse @internal
	if regexp.MustCompile(`@internal\b`).MatchString(description) {
		structure.Metadata["internal"] = metadataValueTrue
	}
}

// calculateMetrics calculates quality metrics for the description
func (dp *DescriptionParser) calculateMetrics(
	structure *DescriptionStructure,
	rawDescription string,
) *DescriptionMetrics {
	if !dp.enableMetrics {
		return nil
	}

	metrics := &DescriptionMetrics{}
	switch {
	case structure != nil:
		populateStructureMetrics(metrics, structure)
	case rawDescription != "":
		metrics.WordCount = len(strings.Fields(rawDescription))
	}

	metrics.Completeness = completenessScore(metrics)
	metrics.Complexity = complexityFor(metrics.WordCount)

	return metrics
}

// populateStructureMetrics fills in the counts and presence flags derived from
// a parsed description.
func populateStructureMetrics(metrics *DescriptionMetrics, structure *DescriptionStructure) {
	metrics.HasOverview = structure.Overview != ""
	metrics.HasChangelog = len(structure.Changelog) > 0
	metrics.HasExamples = len(structure.Examples) > 0
	metrics.HasParameters = len(structure.Parameters) > 0
	metrics.HasReturns = structure.Returns != ""
	metrics.HasErrors = len(structure.Errors) > 0

	metrics.Sections = len(structure.Sections)
	metrics.Examples = len(structure.Examples)
	metrics.Parameters = len(structure.Parameters)

	allText := structure.Overview + structure.Returns
	for i := range structure.Parameters {
		allText += " " + structure.Parameters[i].Description
	}
	for _, err := range structure.Errors {
		allText += " " + err.Description
	}
	metrics.WordCount = len(strings.Fields(allText))
}

// completenessScore weights the documentation elements that are present against
// the total available weight.
func completenessScore(metrics *DescriptionMetrics) float64 {
	elements := []struct {
		present bool
		weight  float64
	}{
		{metrics.HasOverview, weightOverview},
		{metrics.HasParameters, weightParameters},
		{metrics.HasReturns, weightReturns},
		{metrics.HasExamples, weightExamples},
		{metrics.HasErrors, weightErrors},
	}

	completeness, factors := 0.0, 0.0
	for _, e := range elements {
		if e.present {
			completeness += e.weight
		}
		factors += e.weight
	}

	if factors == 0 {
		return 0
	}
	return completeness / factors
}

// complexityFor classifies a description by its word count.
func complexityFor(wordCount int) string {
	switch {
	case wordCount < simpleWordLimit:
		return complexitySimple
	case wordCount < moderateWordLimit:
		return "moderate"
	default:
		return "complex"
	}
}

// ExtractParameterType attempts to extract type information from parameter description
func (dp *DescriptionParser) ExtractParameterType(description string) (paramType, cleanDesc string) {
	// Pattern to match type annotations like (String), [String], {String}, <String>
	typePattern := regexp.MustCompile(`^\s*[\(\[\{<]([^\)\]\}>]+)[\)\]\}>]\s*(.*)`)
	if match := typePattern.FindStringSubmatch(description); len(match) > 2 { //nolint:mnd // regex group count
		return match[1], strings.TrimSpace(match[2])
	}
	return "", description
}

// ExtractDefault extracts default value from parameter description
func (dp *DescriptionParser) ExtractDefault(description string) (defaultValue, cleanDesc string) {
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
	cleaned := regexp.MustCompile(`(?i)^\s*(\*\*INTERNAL\*\*|INTERNAL|JDR\s+internal)\s*:?\s*`).ReplaceAllString(description, "") //nolint:lll // inline regex literal

	// Remove asciidoc anchor markers like [#anchor-name] or [anchor-name]
	cleaned = regexp.MustCompile(`(?m)^\s*\[#?[^\]]+\]\s*\n?`).ReplaceAllString(cleaned, "")

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
		parts := strings.SplitN(firstNonEmptyLine, ".", splitOnFirstPeriod)
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			return strings.TrimSpace(parts[0]) + "."
		}
	}

	// If no period found at all, return the whole cleaned description
	// but limit it to a reasonable length.
	if len(firstNonEmptyLine) > firstLinePreviewMax {
		return firstNonEmptyLine[:firstLinePreviewMax] + "..."
	}
	return firstNonEmptyLine
}

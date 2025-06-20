package changelog

import (
	"fmt"
	"regexp"
	"strings"
)

// Extract extracts version annotations and formats them as AsciiDoc changelog
func Extract(description string) string {
	// Regex to match version annotations: action.version: version_number
	re := regexp.MustCompile(`(?m)^\s*(add|update|deprecated|removed)\.version:\s*(.+)$`)
	matches := re.FindAllStringSubmatch(description, -1)

	if len(matches) == 0 {
		return ""
	}

	// Group versions by action type
	changelog := map[string][]string{
		"add":        {},
		"update":     {},
		"deprecated": {},
		"removed":    {},
	}

	for _, match := range matches {
		if len(match) >= 3 {
			action := match[1]
			version := strings.TrimSpace(match[2])
			if _, exists := changelog[action]; exists {
				changelog[action] = append(changelog[action], version)
			}
		}
	}

	// Build AsciiDoc changelog
	var changelogBuilder strings.Builder
	changelogBuilder.WriteString("\n.Changelog\n")

	// Order: add, update, deprecated, removed
	actions := []string{"add", "update", "deprecated", "removed"}
	for _, action := range actions {
		versions := changelog[action]
		if len(versions) > 0 {
			if len(versions) == 1 {
				changelogBuilder.WriteString(fmt.Sprintf("* %s: %s\n", action, versions[0]))
			} else {
				changelogBuilder.WriteString(fmt.Sprintf("* %s: %s\n", action, strings.Join(versions, ", ")))
			}
		}
	}

	return changelogBuilder.String()
}

// ProcessWithChangelog processes description and extracts changelog separately
func ProcessWithChangelog(description string, processor func(string) string) (processedDesc, changelog string) {
	// Extract changelog first
	changelog = Extract(description)

	// Remove version annotations from description for regular processing
	versionRe := regexp.MustCompile(`(?m)^\s*(add|update|deprecated|removed)\.version:\s*.+$\n?`)
	cleanedDesc := versionRe.ReplaceAllString(description, "")

	// Process the cleaned description normally
	processedDesc = processor(cleanedDesc)

	return processedDesc, changelog
}
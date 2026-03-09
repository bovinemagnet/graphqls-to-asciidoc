package generator

import (
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

// isBuiltInScalar checks if a type name is a built-in GraphQL scalar
func isBuiltInScalar(typeName string) bool {
	builtInScalars := map[string]bool{
		"String":  true,
		"Int":     true,
		"Float":   true,
		"Boolean": true,
		"ID":      true,
	}
	return builtInScalars[typeName]
}

// isInternal checks if a field is internal based on name or description.
// A field is considered internal if:
//   - Its name starts with "internal" (case-insensitive)
//   - Its description contains "INTERNAL" (case-insensitive)
func isInternal(name string, description string) bool {
	if strings.HasPrefix(strings.ToLower(name), "internal") {
		return true
	}
	if strings.Contains(strings.ToUpper(description), "INTERNAL") {
		return true
	}
	return false
}

// isDeprecated checks if a field is deprecated.
// A field is considered deprecated if:
//   - It has a @deprecated directive (checked via directives parameter)
//   - Its description contains "@deprecated" or "deprecated" (case-insensitive)
func isDeprecated(description string, directives ast.DirectiveList) bool {
	for _, d := range directives {
		if strings.ToLower(d.Name) == "deprecated" {
			return true
		}
	}
	descUpper := strings.ToUpper(description)
	if strings.Contains(descUpper, "@DEPRECATED") || strings.Contains(descUpper, "DEPRECATED") {
		return true
	}
	return false
}

// isPreview checks if a field is in preview status.
// A field is considered preview if its description contains "PREVIEW" markers.
func isPreview(description string) bool {
	if strings.Contains(strings.ToUpper(description), "PREVIEW") {
		return true
	}
	return false
}

// isLegacy checks if a field is legacy.
// A field is considered legacy if its description contains "LEGACY" markers.
func isLegacy(description string) bool {
	if strings.Contains(strings.ToUpper(description), "LEGACY") {
		return true
	}
	return false
}

// isZeroVersion checks if a field has version 0.0.0 or 0.0.0.0.
// A field is considered zero version if its description contains version patterns
// like "@version: 0.0.0", "add.version: 0.0.0", etc.
func isZeroVersion(description string) bool {
	if strings.Contains(description, "@version: 0.0.0") || strings.Contains(description, "@version: 0.0.0.0") {
		return true
	}

	versionPatterns := []string{
		"add.version: 0.0.0",
		"update.version: 0.0.0",
		"delete.version: 0.0.0",
		"save.version: 0.0.0",
		"remove.version: 0.0.0",
		"create.version: 0.0.0",
		"deprecated.version: 0.0.0",
		"add.version: 0.0.0.0",
		"update.version: 0.0.0.0",
		"delete.version: 0.0.0.0",
		"save.version: 0.0.0.0",
		"remove.version: 0.0.0.0",
		"create.version: 0.0.0.0",
		"deprecated.version: 0.0.0.0",
	}

	for _, pattern := range versionPatterns {
		if strings.Contains(description, pattern) {
			return true
		}
	}

	return false
}

// shouldIncludeField checks if a field should be included based on the configuration settings.
// This consolidates the filtering logic for queries, mutations, and subscriptions.
func (g *Generator) shouldIncludeField(name, description string, directives ast.DirectiveList) bool {
	if !g.config.IncludeInternal && isInternal(name, description) {
		return false
	}
	if !g.config.IncludeDeprecated && isDeprecated(description, directives) {
		return false
	}
	if !g.config.IncludePreview && isPreview(description) {
		return false
	}
	if !g.config.IncludeLegacy && isLegacy(description) {
		return false
	}
	if !g.config.IncludeZeroVersion && isZeroVersion(description) {
		return false
	}
	return true
}

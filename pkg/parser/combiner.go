package parser

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// CombineSchemaFiles reads and combines multiple GraphQL schema files into a single schema string
func CombineSchemaFiles(files []string) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no files provided to combine")
	}
	
	var combined strings.Builder
	var allContent []string
	
	// Track definitions to detect conflicts
	definedTypes := make(map[string]string) // type name -> source file
	
	// Read all files
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("failed to read schema file '%s': %v", file, err)
		}
		
		contentStr := string(content)
		
		// Check for duplicate type definitions
		if err := checkForConflicts(contentStr, file, definedTypes); err != nil {
			return "", err
		}
		
		allContent = append(allContent, contentStr)
	}
	
	// Combine all content with appropriate separators
	for i, content := range allContent {
		if i > 0 {
			combined.WriteString("\n\n") // Add separator between files
		}
		
		// Add a comment to indicate source file for debugging
		if len(files) > 1 {
			combined.WriteString(fmt.Sprintf("# Source: %s\n", files[i]))
		}
		
		combined.WriteString(strings.TrimSpace(content))
	}
	
	return combined.String(), nil
}

// checkForConflicts detects duplicate type definitions across files
func checkForConflicts(content, filename string, definedTypes map[string]string) error {
	// Regular expressions to match GraphQL type definitions
	typePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s*type\s+(\w+)`),
		regexp.MustCompile(`(?m)^\s*input\s+(\w+)`),
		regexp.MustCompile(`(?m)^\s*enum\s+(\w+)`),
		regexp.MustCompile(`(?m)^\s*scalar\s+(\w+)`),
		regexp.MustCompile(`(?m)^\s*interface\s+(\w+)`),
		regexp.MustCompile(`(?m)^\s*union\s+(\w+)`),
		regexp.MustCompile(`(?m)^\s*directive\s+@(\w+)`),
	}
	
	for _, pattern := range typePatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			
			typeName := match[1]
			
			// Skip built-in GraphQL types
			if isBuiltInType(typeName) {
				continue
			}
			
			if existingFile, exists := definedTypes[typeName]; exists {
				return fmt.Errorf("duplicate definition of '%s' found in '%s' (previously defined in '%s')", 
					typeName, filename, existingFile)
			}
			
			definedTypes[typeName] = filename
		}
	}
	
	return nil
}

// isBuiltInType checks if a type name is a built-in GraphQL type
func isBuiltInType(typeName string) bool {
	builtInTypes := map[string]bool{
		"String":  true,
		"Int":     true,
		"Float":   true,
		"Boolean": true,
		"ID":      true,
		"Query":   false, // Allow Query to be redefined across files
		"Mutation": false, // Allow Mutation to be redefined across files
		"Subscription": false, // Allow Subscription to be redefined across files
	}
	
	allowed, exists := builtInTypes[typeName]
	return exists && allowed
}

// CombineSchemaContent is a simpler version that just concatenates content without conflict checking
// This can be used when conflict checking is not needed or performed elsewhere
func CombineSchemaContent(contents []string) string {
	var combined strings.Builder
	
	for i, content := range contents {
		if i > 0 {
			combined.WriteString("\n\n")
		}
		combined.WriteString(strings.TrimSpace(content))
	}
	
	return combined.String()
}
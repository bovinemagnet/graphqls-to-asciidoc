package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FindSchemaFiles finds all GraphQL schema files matching the given pattern
func FindSchemaFiles(pattern string) ([]string, error) {
	var files []string

	// Handle different patterns
	if strings.Contains(pattern, "**") {
		// Use filepath.WalkDir for recursive patterns
		// Extract the root directory from the pattern
		rootDir := extractRootDir(pattern)
		err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			// Check if file matches the pattern
			matched, err := matchesPattern(path, pattern)
			if err != nil {
				return err
			}

			if matched {
				files = append(files, path)
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error walking directory tree: %v", err)
		}
	} else {
		// Handle brace expansion patterns like *.{graphql,graphqls}
		if strings.Contains(pattern, "{") && strings.Contains(pattern, "}") {
			expandedPatterns := expandBraces(pattern)
			for _, expandedPattern := range expandedPatterns {
				matches, err := filepath.Glob(expandedPattern)
				if err != nil {
					continue // Skip invalid patterns
				}

				files = append(files, filterFiles(matches)...)
			}
		} else {
			// Use filepath.Glob for simple patterns
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern '%s': %v", pattern, err)
			}

			files = append(files, filterFiles(matches)...)
		}
	}

	// Validate that we found at least one file
	if len(files) == 0 {
		return nil, fmt.Errorf("no GraphQL schema files found matching pattern '%s'", pattern)
	}

	// Sort files for deterministic processing order
	sort.Strings(files)

	return files, nil
}

// matchesPattern checks if a file path matches a pattern with ** support
func matchesPattern(path, pattern string) (bool, error) {
	// Convert both to absolute paths for consistent comparison
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}

	// If pattern doesn't contain **, treat it as a simple glob
	if !strings.Contains(pattern, "**") {
		return filepath.Match(pattern, absPath)
	}

	// Extract the filename pattern (part after the last /)
	patternParts := strings.Split(pattern, "/")
	if len(patternParts) == 0 {
		return false, nil
	}

	filenameMatched, err := matchesFilename(patternParts[len(patternParts)-1], filepath.Base(absPath))
	if err != nil || !filenameMatched {
		return false, err
	}

	return matchesPrefixDir(pattern, absPath)
}

// matchesFilename matches a base name against the filename portion of a
// pattern, expanding any {a,b} alternatives first.
func matchesFilename(filenamePattern, base string) (bool, error) {
	if !strings.Contains(filenamePattern, "{") || !strings.Contains(filenamePattern, "}") {
		return filepath.Match(filenamePattern, base)
	}

	for _, expanded := range expandBraces(filenamePattern) {
		matched, err := filepath.Match(expanded, base)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

// matchesPrefixDir checks that absPath sits under the directory preceding the
// ** in pattern. A pattern with no prefix matches any directory.
func matchesPrefixDir(pattern, absPath string) (bool, error) {
	doubleStar := strings.Index(pattern, "**")
	if doubleStar == -1 {
		return true, nil
	}

	prefix := pattern[:doubleStar]
	prefix = strings.TrimSuffix(prefix, "/")
	prefix = strings.TrimSuffix(prefix, "\\")
	if prefix == "" {
		return true, nil
	}

	absPrefix := prefix
	if !filepath.IsAbs(prefix) {
		abs, err := filepath.Abs(prefix)
		if err != nil {
			return false, err
		}
		absPrefix = abs
	}

	return strings.HasPrefix(filepath.Dir(absPath), absPrefix), nil
}

// ValidateSchemaFiles checks that all files are readable and have appropriate extensions
func ValidateSchemaFiles(files []string) error {
	validExtensions := map[string]bool{
		".graphql":  true,
		".graphqls": true,
		".gql":      true,
	}

	for _, file := range files {
		// Check if file is readable
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("schema file '%s' does not exist", file)
		}

		// Check file extension
		ext := strings.ToLower(filepath.Ext(file))
		if !validExtensions[ext] {
			return fmt.Errorf("file '%s' does not have a valid GraphQL extension (.graphql, .graphqls, .gql)", file)
		}
	}

	return nil
}

// expandBraces expands brace patterns like *.{graphql,graphqls} into multiple patterns
func expandBraces(pattern string) []string {
	start := strings.Index(pattern, "{")
	end := strings.Index(pattern, "}")

	if start == -1 || end == -1 || start >= end {
		return []string{pattern}
	}

	prefix := pattern[:start]
	suffix := pattern[end+1:]
	options := strings.Split(pattern[start+1:end], ",")

	var expanded []string
	for _, option := range options {
		expanded = append(expanded, prefix+strings.TrimSpace(option)+suffix)
	}

	return expanded
}

// extractRootDir extracts the root directory from a pattern containing **
func extractRootDir(pattern string) string {
	// Find the position of **
	doubleStar := strings.Index(pattern, "**")
	if doubleStar == -1 {
		// No **, use the directory part of the pattern
		return filepath.Dir(pattern)
	}

	// Get everything before **
	beforeDoubleStar := pattern[:doubleStar]

	// Remove trailing slash if present
	beforeDoubleStar = strings.TrimSuffix(beforeDoubleStar, "/")
	beforeDoubleStar = strings.TrimSuffix(beforeDoubleStar, "\\")

	// If empty or just a slash, use current directory
	if beforeDoubleStar == "" || beforeDoubleStar == "/" {
		return "."
	}

	return beforeDoubleStar
}

// filterFiles filters out directories from a list of file paths
func filterFiles(matches []string) []string {
	var files []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue // Skip files that can't be accessed
		}

		if !info.IsDir() {
			files = append(files, match)
		}
	}
	return files
}

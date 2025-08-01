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
				
				// Filter out directories
				for _, match := range matches {
					info, err := os.Stat(match)
					if err != nil {
						continue
					}
					
					if !info.IsDir() {
						files = append(files, match)
					}
				}
			}
		} else {
			// Use filepath.Glob for simple patterns
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern '%s': %v", pattern, err)
			}
			
			// Filter out directories
			for _, match := range matches {
				info, err := os.Stat(match)
				if err != nil {
					continue // Skip files that can't be accessed
				}
				
				if !info.IsDir() {
					files = append(files, match)
				}
			}
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
	
	// Handle ** patterns
	// Extract the filename pattern (part after the last /)
	patternParts := strings.Split(pattern, "/")
	if len(patternParts) == 0 {
		return false, nil
	}
	
	filenamePattern := patternParts[len(patternParts)-1]
	
	// Check if filename matches the pattern
	var filenameMatched bool
	if strings.Contains(filenamePattern, "{") && strings.Contains(filenamePattern, "}") {
		// Handle brace expansion
		expandedPatterns := expandBraces(filenamePattern)
		for _, expandedPattern := range expandedPatterns {
			matched, err := filepath.Match(expandedPattern, filepath.Base(absPath))
			if err != nil {
				return false, err
			}
			if matched {
				filenameMatched = true
				break
			}
		}
	} else {
		filenameMatched, err = filepath.Match(filenamePattern, filepath.Base(absPath))
		if err != nil {
			return false, err
		}
	}
	
	if !filenameMatched {
		return false, nil
	}
	
	// Check directory structure
	doubleStar := strings.Index(pattern, "**")
	if doubleStar == -1 {
		return filenameMatched, nil
	}
	
	// Get the prefix before **
	prefix := pattern[:doubleStar]
	prefix = strings.TrimSuffix(prefix, "/")
	prefix = strings.TrimSuffix(prefix, "\\")
	
	// If there's no prefix, any path matches
	if prefix == "" {
		return filenameMatched, nil
	}
	
	// Convert prefix to absolute path for comparison
	var absPrefix string
	if filepath.IsAbs(prefix) {
		absPrefix = prefix
	} else {
		absPrefix, err = filepath.Abs(prefix)
		if err != nil {
			return false, err
		}
	}
	
	// Check if the path is under the prefix directory
	pathDir := filepath.Dir(absPath)
	return strings.HasPrefix(pathDir, absPrefix), nil
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
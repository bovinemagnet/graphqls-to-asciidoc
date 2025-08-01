package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindSchemaFiles(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir := t.TempDir()
	
	// Create test files
	testFiles := []string{
		"schema.graphql",
		"types.graphqls",
		"mutations.gql",
		"subdirectory/nested.graphql",
		"subdirectory/deep/another.graphqls",
		"ignored.txt", // Should be ignored
	}
	
	for _, file := range testFiles {
		fullPath := filepath.Join(tempDir, file)
		dir := filepath.Dir(fullPath)
		
		// Create directory if it doesn't exist
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		
		// Create file
		if err := os.WriteFile(fullPath, []byte("type Test { id: ID! }"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", fullPath, err)
		}
	}
	
	// Change to temp directory for testing
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)
	
	tests := []struct {
		name     string
		pattern  string
		expected []string
		wantErr  bool
	}{
		{
			name:     "simple glob pattern",
			pattern:  "*.graphql",
			expected: []string{"schema.graphql"},
			wantErr:  false,
		},
		{
			name:     "multiple extensions",
			pattern:  "*.{graphql,graphqls}",
			expected: []string{"schema.graphql", "types.graphqls"},
			wantErr:  false,
		},
		{
			name:     "recursive pattern",
			pattern:  "**/*.graphql",
			expected: []string{"schema.graphql", "subdirectory/nested.graphql"},
			wantErr:  false,
		},
		{
			name:     "all graphql files recursively",
			pattern:  "**/*.{graphql,graphqls,gql}",
			expected: []string{"mutations.gql", "schema.graphql", "subdirectory/deep/another.graphqls", "subdirectory/nested.graphql", "types.graphqls"},
			wantErr:  false,
		},
		{
			name:     "no matches",
			pattern:  "*.nonexistent",
			expected: nil,
			wantErr:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := FindSchemaFiles(tt.pattern)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("FindSchemaFiles() expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("FindSchemaFiles() unexpected error: %v", err)
				return
			}
			
			if len(files) != len(tt.expected) {
				t.Errorf("FindSchemaFiles() found %d files, expected %d", len(files), len(tt.expected))
				t.Errorf("Found: %v", files)
				t.Errorf("Expected: %v", tt.expected)
				return
			}
			
			for i, expected := range tt.expected {
				if files[i] != expected {
					t.Errorf("FindSchemaFiles() file[%d] = %s, expected %s", i, files[i], expected)
				}
			}
		})
	}
}

func TestValidateSchemaFiles(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create test files
	validFile := filepath.Join(tempDir, "valid.graphql")
	invalidExtFile := filepath.Join(tempDir, "invalid.txt")
	
	os.WriteFile(validFile, []byte("type Test { id: ID! }"), 0644)
	os.WriteFile(invalidExtFile, []byte("not graphql"), 0644)
	
	tests := []struct {
		name    string
		files   []string
		wantErr bool
	}{
		{
			name:    "valid files",
			files:   []string{validFile},
			wantErr: false,
		},
		{
			name:    "invalid extension",
			files:   []string{invalidExtFile},
			wantErr: true,
		},
		{
			name:    "nonexistent file",
			files:   []string{filepath.Join(tempDir, "nonexistent.graphql")},
			wantErr: true,
		},
		{
			name:    "mixed valid and invalid",
			files:   []string{validFile, invalidExtFile},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSchemaFiles(tt.files)
			
			if tt.wantErr && err == nil {
				t.Errorf("ValidateSchemaFiles() expected error but got none")
			}
			
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateSchemaFiles() unexpected error: %v", err)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	// Create a temp directory for testing with actual files
	tempDir := t.TempDir()
	
	// Create test directory structure
	os.MkdirAll(filepath.Join(tempDir, "subdirectory"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "a", "b", "c"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "schemas"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "other"), 0755)
	
	// Create test files
	testFiles := map[string]string{
		"schema.graphql":                filepath.Join(tempDir, "schema.graphql"),
		"schema.txt":                    filepath.Join(tempDir, "schema.txt"),
		"subdirectory/schema.graphql":   filepath.Join(tempDir, "subdirectory", "schema.graphql"),
		"a/b/c/schema.graphql":          filepath.Join(tempDir, "a", "b", "c", "schema.graphql"),
		"schemas/types.graphql":         filepath.Join(tempDir, "schemas", "types.graphql"),
		"other/types.graphql":           filepath.Join(tempDir, "other", "types.graphql"),
	}
	
	// Create the actual files
	for _, filePath := range testFiles {
		os.WriteFile(filePath, []byte("type Test { id: ID! }"), 0644)
	}
	
	tests := []struct {
		name     string
		path     string
		pattern  string
		expected bool
		wantErr  bool
	}{
		{
			name:     "simple match",
			path:     testFiles["schema.graphql"],
			pattern:  filepath.Join(tempDir, "*.graphql"),
			expected: true,
			wantErr:  false,
		},
		{
			name:     "no match",
			path:     testFiles["schema.txt"],
			pattern:  filepath.Join(tempDir, "*.graphql"),
			expected: false,
			wantErr:  false,
		},
		{
			name:     "recursive match",
			path:     testFiles["subdirectory/schema.graphql"],
			pattern:  filepath.Join(tempDir, "**/*.graphql"),
			expected: true,
			wantErr:  false,
		},
		{
			name:     "deep recursive match",
			path:     testFiles["a/b/c/schema.graphql"],
			pattern:  filepath.Join(tempDir, "**/*.graphql"),
			expected: true,
			wantErr:  false,
		},
		{
			name:     "prefix with recursive",
			path:     testFiles["schemas/types.graphql"],
			pattern:  filepath.Join(tempDir, "schemas/**/*.graphql"),
			expected: true,
			wantErr:  false,
		},
		{
			name:     "prefix mismatch",
			path:     testFiles["other/types.graphql"],
			pattern:  filepath.Join(tempDir, "schemas/**/*.graphql"),
			expected: false,
			wantErr:  false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, err := matchesPattern(tt.path, tt.pattern)
			
			if tt.wantErr && err == nil {
				t.Errorf("matchesPattern() expected error but got none")
				return
			}
			
			if !tt.wantErr && err != nil {
				t.Errorf("matchesPattern() unexpected error: %v", err)
				return
			}
			
			if matched != tt.expected {
				t.Errorf("matchesPattern() = %v, expected %v", matched, tt.expected)
			}
		})
	}
}
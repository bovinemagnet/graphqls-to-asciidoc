package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCombineSchemaFiles(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create test schema files
	file1Content := `type User {
  id: ID!
  name: String
}`

	file2Content := `type Post {
  id: ID!
  title: String
  author: User
}`

	file1 := filepath.Join(tempDir, "user.graphql")
	file2 := filepath.Join(tempDir, "post.graphql")
	
	os.WriteFile(file1, []byte(file1Content), 0644)
	os.WriteFile(file2, []byte(file2Content), 0644)
	
	tests := []struct {
		name     string
		files    []string
		wantErr  bool
		contains []string
	}{
		{
			name:     "combine two files",
			files:    []string{file1, file2},
			wantErr:  false,
			contains: []string{"type User", "type Post", "# Source:"},
		},
		{
			name:     "single file",
			files:    []string{file1},
			wantErr:  false,
			contains: []string{"type User"},
		},
		{
			name:    "no files",
			files:   []string{},
			wantErr: true,
		},
		{
			name:    "nonexistent file",
			files:   []string{filepath.Join(tempDir, "nonexistent.graphql")},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CombineSchemaFiles(tt.files)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("CombineSchemaFiles() expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("CombineSchemaFiles() unexpected error: %v", err)
				return
			}
			
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("CombineSchemaFiles() result missing expected content: %s", expected)
					t.Errorf("Result:\n%s", result)
				}
			}
		})
	}
}

func TestCombineSchemaFilesConflicts(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create conflicting schema files
	file1Content := `type User {
  id: ID!
  name: String
}`

	file2Content := `type User {
  id: ID!
  email: String
}`

	file1 := filepath.Join(tempDir, "user1.graphql")
	file2 := filepath.Join(tempDir, "user2.graphql")
	
	os.WriteFile(file1, []byte(file1Content), 0644)
	os.WriteFile(file2, []byte(file2Content), 0644)
	
	_, err := CombineSchemaFiles([]string{file1, file2})
	if err == nil {
		t.Errorf("CombineSchemaFiles() expected conflict error but got none")
	}
	
	if !strings.Contains(err.Error(), "duplicate definition") {
		t.Errorf("CombineSchemaFiles() error should mention duplicate definition, got: %v", err)
	}
}

func TestCheckForConflicts(t *testing.T) {
	definedTypes := make(map[string]string)
	
	tests := []struct {
		name     string
		content  string
		filename string
		setup    func()
		wantErr  bool
	}{
		{
			name:     "new type definition",
			content:  "type User { id: ID! }",
			filename: "user.graphql",
			setup:    func() {},
			wantErr:  false,
		},
		{
			name:     "duplicate type definition",
			content:  "type User { email: String }",
			filename: "user2.graphql",
			setup: func() {
				definedTypes["User"] = "user.graphql"
			},
			wantErr: true,
		},
		{
			name:     "input type",
			content:  "input UserInput { name: String }",
			filename: "input.graphql",
			setup:    func() {},
			wantErr:  false,
		},
		{
			name:     "enum type",
			content:  "enum Status { ACTIVE INACTIVE }",
			filename: "enum.graphql",
			setup:    func() {},
			wantErr:  false,
		},
		{
			name:     "scalar type",
			content:  "scalar DateTime",
			filename: "scalar.graphql",
			setup:    func() {},
			wantErr:  false,
		},
		{
			name:     "directive definition",
			content:  "directive @auth on FIELD_DEFINITION",
			filename: "directive.graphql",
			setup:    func() {},
			wantErr:  false,
		},
		{
			name:     "built-in type (allowed)",
			content:  "scalar String",
			filename: "builtin.graphql",
			setup:    func() {},
			wantErr:  false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset map for each test
			definedTypes = make(map[string]string)
			tt.setup()
			
			err := checkForConflicts(tt.content, tt.filename, definedTypes)
			
			if tt.wantErr && err == nil {
				t.Errorf("checkForConflicts() expected error but got none")
			}
			
			if !tt.wantErr && err != nil {
				t.Errorf("checkForConflicts() unexpected error: %v", err)
			}
		})
	}
}

func TestIsBuiltInType(t *testing.T) {
	tests := []struct {
		typeName string
		expected bool
	}{
		{"String", true},
		{"Int", true},
		{"Float", true},
		{"Boolean", true},
		{"ID", true},
		{"Query", false},    // Allow redefinition
		{"Mutation", false}, // Allow redefinition
		{"Subscription", false}, // Allow redefinition
		{"User", false},     // Custom type
		{"DateTime", false}, // Custom scalar
	}
	
	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			result := isBuiltInType(tt.typeName)
			if result != tt.expected {
				t.Errorf("isBuiltInType(%s) = %v, expected %v", tt.typeName, result, tt.expected)
			}
		})
	}
}

func TestCombineSchemaContent(t *testing.T) {
	contents := []string{
		"type User { id: ID! }",
		"type Post { title: String }",
		"enum Status { ACTIVE INACTIVE }",
	}
	
	result := CombineSchemaContent(contents)
	
	expectedParts := []string{
		"type User { id: ID! }",
		"type Post { title: String }",
		"enum Status { ACTIVE INACTIVE }",
	}
	
	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("CombineSchemaContent() missing expected part: %s", part)
		}
	}
	
	// Check that parts are separated
	parts := strings.Split(result, "\n\n")
	if len(parts) != 3 {
		t.Errorf("CombineSchemaContent() expected 3 parts separated by double newlines, got %d", len(parts))
	}
}
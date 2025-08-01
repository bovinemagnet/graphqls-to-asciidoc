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
	testCases := []struct {
		name     string
		content  string
		filename string
		setup    func(map[string]string)
		wantErr  bool
	}{
		{"new type definition", "type User { id: ID! }", "user.graphql", nil, false},
		{"duplicate type definition", "type User { email: String }", "user2.graphql", 
			func(dt map[string]string) { dt["User"] = "user.graphql" }, true},
		{"input type", "input UserInput { name: String }", "input.graphql", nil, false},
		{"enum type", "enum Status { ACTIVE INACTIVE }", "enum.graphql", nil, false},
		{"scalar type", "scalar DateTime", "scalar.graphql", nil, false},
		{"directive definition", "directive @auth on FIELD_DEFINITION", "directive.graphql", nil, false},
		{"built-in type (allowed)", "scalar String", "builtin.graphql", nil, false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			definedTypes := make(map[string]string)
			if tc.setup != nil {
				tc.setup(definedTypes)
			}
			
			err := checkForConflicts(tc.content, tc.filename, definedTypes)
			
			if (err != nil) != tc.wantErr {
				t.Errorf("checkForConflicts() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestIsBuiltInType(t *testing.T) {
	testCases := map[string]bool{
		"String": true, "Int": true, "Float": true, "Boolean": true, "ID": true,
		"Query": false, "Mutation": false, "Subscription": false, // Allow redefinition
		"User": false, "DateTime": false, // Custom types
	}
	
	for typeName, expected := range testCases {
		t.Run(typeName, func(t *testing.T) {
			if result := isBuiltInType(typeName); result != expected {
				t.Errorf("isBuiltInType(%s) = %v, expected %v", typeName, result, expected)
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
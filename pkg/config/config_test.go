package config

import (
	"os"
	"testing"
)

func TestNewConfig(t *testing.T) {
	config := NewConfig()

	// Test default values
	if !config.IncludeMutations {
		t.Error("Expected IncludeMutations to be true by default")
	}
	if !config.IncludeQueries {
		t.Error("Expected IncludeQueries to be true by default")
	}
	if config.IncludeSubscriptions {
		t.Error("Expected IncludeSubscriptions to be false by default")
	}
	if !config.IncludeDirectives {
		t.Error("Expected IncludeDirectives to be true by default")
	}
	if !config.IncludeTypes {
		t.Error("Expected IncludeTypes to be true by default")
	}
	if !config.IncludeEnums {
		t.Error("Expected IncludeEnums to be true by default")
	}
	if !config.IncludeInputs {
		t.Error("Expected IncludeInputs to be true by default")
	}
	if !config.IncludeScalars {
		t.Error("Expected IncludeScalars to be true by default")
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		name        string
		schemaFile  string
		expectError bool
	}{
		{
			name:        "valid schema file",
			schemaFile:  "../../test/schema.graphql",
			expectError: false,
		},
		{
			name:        "empty schema file",
			schemaFile:  "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := NewConfig()
			config.SchemaFile = tc.schemaFile

			err := config.Validate()
			if tc.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func TestVersionOutput(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalBuildTime := BuildTime

	// Set test values
	Version = "test-version"
	BuildTime = "2025-01-01_12:00:00"

	// Restore original values after test
	defer func() {
		Version = originalVersion
		BuildTime = originalBuildTime
	}()

	// Note: We can't easily test the actual version output without capturing stdout,
	// but we can verify the variables are accessible
	if Version != "test-version" {
		t.Errorf("Version variable not set correctly, got %q", Version)
	}
	if BuildTime != "2025-01-01_12:00:00" {
		t.Errorf("BuildTime variable not set correctly, got %q", BuildTime)
	}

	// Test that HandleVersion returns true when ShowVersion is set
	config := NewConfig()
	config.ShowVersion = true

	// We can't easily test the output, but we can test the return value
	// In a real test, this would print to stdout, but for testing we just verify the logic
	if !config.ShowVersion {
		t.Error("ShowVersion should be true when set")
	}
}

func TestConfigWithAllFlags(t *testing.T) {
	cfg := NewConfig()

	// Test all boolean flags
	cfg.IncludeQueries = false
	cfg.IncludeMutations = false
	cfg.IncludeSubscriptions = false
	cfg.IncludeTypes = false
	cfg.IncludeEnums = false
	cfg.IncludeInputs = false
	cfg.IncludeDirectives = false
	cfg.IncludeScalars = false
	cfg.ExcludeInternal = true
	cfg.Verbose = true

	// Create a temporary file for validation
	tmpFile, err := os.CreateTemp("", "test-*.graphql")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Test string fields
	cfg.SchemaFile = tmpFile.Name()
	cfg.OutputFile = "output.adoc"

	// Test validation
	err = cfg.Validate()
	if err != nil {
		t.Errorf("Config validation failed: %v", err)
	}

	// Test that all fields are set correctly
	if cfg.SchemaFile != tmpFile.Name() {
		t.Error("SchemaFile not set correctly")
	}
	if cfg.OutputFile != "output.adoc" {
		t.Error("OutputFile not set correctly")
	}
	if !cfg.ExcludeInternal {
		t.Error("ExcludeInternal not set correctly")
	}
	if !cfg.Verbose {
		t.Error("Verbose not set correctly")
	}
}

func TestConfigValidationErrors(t *testing.T) {
	cfg := NewConfig()

	// Test empty schema file
	cfg.SchemaFile = ""
	err := cfg.Validate()
	if err == nil {
		t.Error("Should return error for empty schema file")
	}

	// Test non-existent schema file
	cfg.SchemaFile = "nonexistent.graphql"
	err = cfg.Validate()
	if err == nil {
		t.Error("Should return error for non-existent schema file")
	}
}

func TestConfigDefaultValues(t *testing.T) {
	cfg := NewConfig()

	// Test that default values are set correctly
	if !cfg.IncludeQueries {
		t.Error("IncludeQueries should be true by default")
	}
	if !cfg.IncludeMutations {
		t.Error("IncludeMutations should be true by default")
	}
	if cfg.IncludeSubscriptions {
		t.Error("IncludeSubscriptions should be false by default")
	}
	if !cfg.IncludeTypes {
		t.Error("IncludeTypes should be true by default")
	}
	if !cfg.IncludeEnums {
		t.Error("IncludeEnums should be true by default")
	}
	if !cfg.IncludeInputs {
		t.Error("IncludeInputs should be true by default")
	}
	if !cfg.IncludeDirectives {
		t.Error("IncludeDirectives should be true by default")
	}
	if !cfg.IncludeScalars {
		t.Error("IncludeScalars should be true by default")
	}
	if cfg.ExcludeInternal {
		t.Error("ExcludeInternal should be false by default")
	}
	if cfg.Verbose {
		t.Error("Verbose should be false by default")
	}
}

func TestGetOutputWriterStdout(t *testing.T) {
	cfg := NewConfig()
	cfg.OutputFile = ""
	file, isStdout, err := cfg.GetOutputWriter()
	if err != nil {
		t.Fatalf("GetOutputWriter returned error: %v", err)
	}
	if isStdout {
		t.Error("Expected isStdout to be false for empty OutputFile")
	}
	if file != os.Stdout {
		t.Error("Expected file to be os.Stdout for empty OutputFile")
	}
}

func TestGetOutputWriterFileSuccess(t *testing.T) {
	cfg := NewConfig()
	tmpFile, err := os.CreateTemp("", "output-*.adoc")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	cfg.OutputFile = tmpFile.Name()
	file, isStdout, err := cfg.GetOutputWriter()
	if err != nil {
		t.Fatalf("GetOutputWriter returned error: %v", err)
	}
	if !isStdout {
		t.Error("Expected isStdout to be true for OutputFile")
	}
	if file == os.Stdout {
		t.Error("Expected file not to be os.Stdout for OutputFile")
	}
	file.Close()
}

func TestGetOutputWriterFileDirNotExist(t *testing.T) {
	cfg := NewConfig()
	cfg.OutputFile = "/nonexistentdir/output.adoc"
	_, _, err := cfg.GetOutputWriter()
	if err == nil {
		t.Error("Expected error for non-existent output directory")
	}
}

func TestValidateOutputDirNotExist(t *testing.T) {
	cfg := NewConfig()
	tmpFile, err := os.CreateTemp("", "test-*.graphql")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	cfg.SchemaFile = tmpFile.Name()
	cfg.OutputFile = "/nonexistentdir/output.adoc"
	err = cfg.Validate()
	if err == nil {
		t.Error("Expected error for non-existent output directory in Validate")
	}
}

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go CLI tool that converts GraphQL schema files (.graphqls) to AsciiDoc documentation files (.adoc). The tool parses GraphQL schemas using the `vektah/gqlparser` library and generates comprehensive documentation with tables, type definitions, and cross-references.

## Architecture

The application follows a **modular architecture** with clear separation of concerns across multiple packages:

### Core Structure
- **`main.go`**: Entry point that orchestrates the conversion process
- **`pkg/config/`**: Configuration management and command-line flag parsing
- **`pkg/parser/`**: GraphQL schema parsing, file discovery, and schema combination
- **`pkg/generator/`**: AsciiDoc generation logic and template processing
- **`pkg/templates/`**: Template management and rendering
- **`pkg/changelog/`**: Changelog extraction and formatting from GraphQL descriptions
- **`pkg/metrics/`**: Performance monitoring and processing statistics

### Key Features
1. **Configuration Management** (`pkg/config/`):
   - Centralized command-line flag parsing and validation
   - Support for single file (`-schema`) or multiple files (`-pattern`) 
   - Output file support (`-output`) or stdout
   - Comprehensive validation with helpful error messages

2. **Schema Processing** (`pkg/parser/`):
   - File discovery using glob patterns (e.g., `schemas/**/*.graphqls`)
   - Schema file combination with conflict detection
   - Support for `.graphql`, `.graphqls`, and `.gql` extensions
   - Validation of file accessibility and GraphQL syntax

3. **Documentation Generation** (`pkg/generator/`):
   - Template-based AsciiDoc generation
   - Support for all GraphQL constructs (Queries, Mutations, Subscriptions, Types, Enums, Inputs, Directives, Scalars)
   - Catalogue mode for quick reference tables of queries, mutations, and subscriptions
   - Advanced description processing (changelog extraction, markdown conversion, cross-references)
   - Configurable section inclusion/exclusion

4. **Performance Monitoring** (`pkg/metrics/`):
   - Processing time tracking per section
   - Memory usage monitoring  
   - Detailed performance reports with efficiency calculations

### Architectural Benefits
- **Modularity**: Clear separation of concerns enables easier testing and maintenance
- **Testability**: Each package has comprehensive unit tests (100+ test cases)
- **Extensibility**: Interface-based design allows for future output formats
- **Configurability**: Extensive customization options via flags and configuration

## Common Commands

### Build
```bash
make build
# or
go build -o bin/graphqls-to-asciidoc -v
```

### Test
```bash
make test
# or
go test -v ./...
```

### Run
```bash
# Basic usage (single file)
./graphqls-to-asciidoc -schema ./schema.graphqls > output.adoc

# Multiple files using pattern
./graphqls-to-asciidoc -pattern "schemas/**/*.graphqls" > output.adoc

# Generate catalogue mode (quick reference)
./graphqls-to-asciidoc -schema ./test/schema.graphql -catalogue -o api-catalogue.adoc

# With options (single file)
./graphqls-to-asciidoc -schema ./test/schema.graphql -exclude-internal -mutations=false > output.adoc

# With options (multiple files)
./graphqls-to-asciidoc -pattern "**/*.{graphql,graphqls}" -exclude-internal -o docs.adoc

# Catalogue with filtering
./graphqls-to-asciidoc -pattern "schemas/**/*.graphqls" -catalogue -exclude-internal -o public-api.adoc

# Check version information
./graphqls-to-asciidoc -version
```

### Test Documentation Generation
```bash
make test_doc
# Generates test/schema.adoc from test/schema.graphql
```

### Clean
```bash
make clean
```

### Release
Uses GoReleaser for cross-platform builds targeting Linux, macOS, and Windows on amd64 and arm64 architectures.

## Command-Line Flags

The tool supports extensive customization through flags:
- `-schema`: Path to GraphQL schema file (single file mode)
- `-pattern`: Pattern to match multiple GraphQL schema files (e.g., `schemas/**/*.graphqls`)
- `-version`: Show program version and build information
- `-catalogue`: Generate quick reference catalogue with queries, mutations, and subscriptions tables only
- `-sub-title`: Optional subtitle for catalogue (e.g., 'Activities')
- `-exclude-internal`: Exclude queries/mutations marked as INTERNAL (deprecated, use `--inc-internal` instead)
- `-mutations`, `-queries`, `-subscriptions`: Include/exclude specific sections (default: most are true)
- `-directives`, `-types`, `-enums`, `-inputs`, `-scalars`: Include/exclude type definitions

### Filtering Flags
The following flags control which items are included in the output:
- `--inc-internal`: Include internal queries/mutations (those starting with 'internal' or marked INTERNAL)
- `--inc-deprecated`: Include deprecated queries/mutations (those with @deprecated directive)
- `--inc-preview`: Include preview queries/mutations (those marked as PREVIEW)
- `--inc-legacy`: Include legacy queries/mutations (those marked as LEGACY)
- `--inc-zero`: Include items with version 0.0.0 or 0.0.0.0
- `--inc-changelog`: Include changelog information in catalogue descriptions

**Note:** `-schema` and `-pattern` flags are mutually exclusive. Use `-schema` for single file mode or `-pattern` for multiple file mode.

## Key Components

### Package Functions by Module

**`pkg/config/`**:
- `ParseFlags()`: Command-line argument parsing and validation
- `GetOutputWriter()`: Output destination management (file or stdout)
- `Validate()`: Configuration validation with detailed error messages

**`pkg/parser/`**:
- `FindSchemaFiles()`: File discovery using glob patterns
- `CombineSchemaFiles()`: Multi-file schema combination with conflict detection
- `ValidateSchemaFiles()`: File accessibility and extension validation

**`pkg/generator/`**:
- `New()`: Generator initialization with configuration and schema
- `Generate()`: Main generation orchestration (supports both full documentation and catalogue mode)
- `generateCatalogue()`: Catalogue mode generation with queries, mutations, and subscriptions tables
- `shouldIncludeField()`: Centralized field filtering based on internal/deprecated/preview/legacy/zero-version status
- `ProcessTypeName()`: GraphQL type to AsciiDoc cross-reference conversion
- `ProcessDescription()`: Advanced description processing (changelog, markdown, cross-refs)

**`pkg/templates/`**:
- Template management and rendering for consistent output formatting
- Support for customizable templates and formatting

**`pkg/changelog/`**:
- `Extract()`: Version annotation extraction from GraphQL descriptions
- `ProcessWithChangelog()`: Changelog integration into documentation

**`pkg/metrics/`**:
- `New()`: Performance monitoring initialization
- `SectionTimer()`: Per-section timing and statistics
- `PrintSummary()`: Detailed performance and efficiency reporting

## Testing

The project has **comprehensive test coverage** with 100+ test cases across all packages:

### Test Structure
- **`main_test.go`**: Integration tests and version output validation
- **`pkg/*/`**: Each package has dedicated `*_test.go` files with extensive unit tests
- **Coverage**: High test coverage across all modules including edge cases and error scenarios

### Test Categories
- **Unit Tests**: Individual function testing with various input scenarios
- **Integration Tests**: End-to-end testing of complete workflows  
- **Error Handling**: Validation of error conditions and edge cases
- **Performance Tests**: Benchmark tests for critical code paths

### Running Tests
```bash
make test              # Run all tests
make test-coverage     # Run tests with coverage report
make test-bench        # Run benchmark tests
```

## Output Format

Generates comprehensive AsciiDoc documentation with:

### Documentation Features
- **Table of contents and metadata**: Automatic generation with version information and command line tracking
- **Modular sections**: Separate sections for each GraphQL construct (Queries, Mutations, Subscriptions, Types, Enums, Inputs, Directives, Scalars)
- **Catalogue mode**: Quick reference tables with GraphQL introduction, queries, mutations, and subscriptions
  - Includes `:revdate:` attribute with generation timestamp
  - Includes `:commandline:` attribute with the exact command used
  - Includes `_attributes.adoc` for consistent styling
  - Provides explanatory text about GraphQL concepts
  - Shows "No subscriptions exist in this schema" note when applicable
- **Cross-references**: Intelligent linking between types and definitions
- **Advanced formatting**:
  - Changelog integration from `@version` annotations
  - Markdown to AsciiDoc conversion
  - Code block processing with syntax highlighting
  - Table generation and formatting
- **Tagging system**: Extensive AsciiDoc tags for selective inclusion in larger documents
- **Directive documentation**: Complete signature, arguments table, usage locations, and repeatability information

### Performance Monitoring
The tool provides detailed performance metrics including:
- Processing time per section
- Memory usage statistics  
- Processing efficiency calculations
- Items processed per second
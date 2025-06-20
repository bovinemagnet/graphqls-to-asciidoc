# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go CLI tool that converts GraphQL schema files (.graphqls) to AsciiDoc documentation files (.adoc). The tool parses GraphQL schemas using the `vektah/gqlparser` library and generates comprehensive documentation with tables, type definitions, and cross-references.

## Architecture

The application is a single-file Go program (`main.go`) that:

1. **Parses command-line flags** for customizing output (exclude internal queries, include/exclude specific sections like mutations, types, enums, etc.)
2. **Reads and parses GraphQL schema** using `github.com/vektah/gqlparser/v2`
3. **Generates AsciiDoc output** using Go templates and string builders
4. **Supports multiple output sections**: Queries, Mutations, Subscriptions, Types, Enums, Inputs, Directives, and Scalars

Key architectural components:
- **Template system**: Uses Go's `text/template` for consistent formatting
- **Type processing**: Converts GraphQL types to AsciiDoc cross-references
- **Sorting**: All definitions are sorted alphabetically for consistent output
- **Tagging**: Extensive use of AsciiDoc tags for selective inclusion in larger documents

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
# Basic usage
./graphqls-to-asciidoc -schema ./schema.graphqls > output.adoc

# With options
./graphqls-to-asciidoc -schema ./test/schema.graphql -exclude-internal -mutations=false > output.adoc
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
- `-schema`: Path to GraphQL schema file (required)
- `-exclude-internal`: Exclude queries/mutations marked as INTERNAL
- `-mutations`, `-queries`, `-subscriptions`: Include/exclude specific sections (default: most are true)
- `-directives`, `-types`, `-enums`, `-inputs`, `-scalars`: Include/exclude type definitions

## Key Functions

- `main()`: Entry point, orchestrates the entire conversion process
- `printXxxTmpl()` functions: Template-based rendering for each section type
- `processTypeName()`: Converts GraphQL types to AsciiDoc cross-references
- `camelToSnake()`: Converts names for anchor generation
- `getArgsString()`: Formats GraphQL arguments for display
- `printAsciiDocTagsTmpl()`: Processes embedded AsciiDoc tags in descriptions

## Testing

The project includes basic unit tests (`main_test.go`) focusing on utility functions like `camelToSnake()`.

## Output Format

Generates comprehensive AsciiDoc documentation with:
- Table of contents and metadata
- Separate sections for each GraphQL construct
- Cross-references between types
- Extensive tagging for selective inclusion
- Consistent table formatting and styling
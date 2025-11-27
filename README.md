# GraphQLS to AsciiDoc

A powerful CLI tool that converts GraphQL schema files (.graphqls) to comprehensive AsciiDoc documentation with rich formatting, cross-references, and professional styling.

## Features

### 🚀 Core Functionality
- **Complete GraphQL Support**: Queries, Mutations, Subscriptions, Types, Enums, Inputs, Directives, and Scalars
- **Multi-file Support**: Process single files or combine multiple schema files using glob patterns
- **Rich Documentation**: Converts GraphQL descriptions to formatted AsciiDoc with tables, cross-references, and metadata
- **Catalogue Mode**: Generate quick reference tables of queries, mutations, and subscriptions with introductory text
- **Flexible Output**: Configurable sections with command-line flags to include/exclude specific parts

### 📝 Advanced Markup Support
- **Admonition Blocks**: Convert `**NOTE**:`, `**WARNING**:`, etc. to AsciiDoc admonition blocks
- **Code Callouts**: Automatic conversion of code annotations `(1)`, `// 2`, `# 3`, `/* 4 */` to AsciiDoc callouts `<1>`, `<2>`, etc.
- **Anchors & Cross-References**: Support for `[#anchor]`, `{ref:target}`, and `{link:target|text}` patterns
- **Code Blocks**: Markdown-style ````lang` blocks converted to AsciiDoc `[source,lang]` format
- **Table Conversion**: Markdown tables automatically converted to AsciiDoc format with proper headers
- **AsciiDoc Pass-through**: Existing AsciiDoc tables preserved unchanged for maximum flexibility
- **Deprecated Directives**: Automatic formatting of `@deprecated` annotations

### 🔧 Configuration Options
- **Selective Generation**: Include/exclude sections (mutations, queries, types, etc.)
- **Internal Filtering**: Exclude queries/mutations marked as `INTERNAL`
- **Changelog Integration**: Automatic extraction and formatting of version annotations
- **Cross-Platform**: Supports Linux, macOS, and Windows (amd64 and arm64)

### 📁 Multi-File Schema Support
- **Glob Patterns**: Use patterns like `schemas/**/*.graphqls` to process multiple files
- **Absolute & Relative Paths**: Full support for both absolute (`/home/user/schemas/**/*.graphqls`) and relative (`schemas/**/*.graphqls`) path patterns
- **Mixed Extensions**: Support for `.graphql`, `.graphqls`, and `.gql` files
- **Brace Expansion**: Patterns like `*.{graphql,graphqls}` for multiple extensions
- **Conflict Detection**: Automatic detection of duplicate type definitions across files
- **Deterministic Processing**: Files processed in alphabetical order for consistent output
- **Source Tracking**: Combined schema includes source file comments for debugging

## Installation

### Homebrew (macOS and Linux)
```bash
# Add the tap and install
brew tap bovinemagnet/tap
brew install graphqls-to-asciidoc

# Or install directly
brew install bovinemagnet/tap/graphqls-to-asciidoc
```

### Pre-built Binaries
Download the latest release from the [Releases page](https://github.com/bovinemagnet/graphqls-to-asciidoc/releases).

### Build from Source
```bash
git clone https://github.com/bovinemagnet/graphqls-to-asciidoc.git
cd graphqls-to-asciidoc
make build
```

## Usage

### Basic Usage
```bash
# Generate documentation from single file to stdout
graphqls-to-asciidoc -s schema.graphql

# Generate documentation from single file to a file
graphqls-to-asciidoc -s schema.graphql -o documentation.adoc

# Generate documentation from multiple files using patterns
graphqls-to-asciidoc -p "schemas/**/*.graphqls" -o documentation.adoc

# Multiple file extensions
graphqls-to-asciidoc -p "**/*.{graphql,graphqls,gql}" -o full-schema.adoc
```

### Catalogue Mode
Generate a quick reference catalogue of all queries, mutations, and subscriptions:

```bash
# Generate a catalogue from a schema file
graphqls-to-asciidoc -s schema.graphql --catalogue -o api-catalogue.adoc

# Generate catalogue from multiple files
graphqls-to-asciidoc -p "schemas/**/*.graphqls" --catalogue -o api-catalogue.adoc

# Catalogue with filtering
graphqls-to-asciidoc -s schema.graphql --catalogue --exclude-internal -o public-api.adoc
```

The catalogue output includes:
- **Introduction**: Brief overview of GraphQL and its key concepts
- **Queries table**: Quick reference to all available queries with descriptions
- **Mutations table**: Summary of all mutations for data modification
- **Subscriptions section**: Real-time subscription endpoints (or note if none exist)
- **Metadata**: Generation timestamp (`:revdate:`) and command line used (`:commandline:`)
- **Attributes**: Includes `_attributes.adoc` for consistent styling

### Advanced Usage
```bash
# Single file: Exclude internal queries and disable mutations section
graphqls-to-asciidoc -s schema.graphql -o api-docs.adoc -x -m=false

# Multiple files: Generate only types and enums using short flags
graphqls-to-asciidoc -p "types/*.graphql" -o types-only.adoc -q=false -m=false

# Multiple files: Generate comprehensive documentation with all sections
graphqls-to-asciidoc -p "schemas/**/*.graphqls" -o full-docs.adoc --subscriptions

# Pattern with verbose logging to see which files were combined
graphqls-to-asciidoc -p "**/*.graphql" -o docs.adoc --verbose

# Check version and help
graphqls-to-asciidoc -v
graphqls-to-asciidoc -h
```

### Pattern Examples

The `--pattern` flag supports various glob patterns for flexible file matching:

```bash
# All .graphql files in current directory
graphqls-to-asciidoc -p "*.graphql"

# All GraphQL files recursively
graphqls-to-asciidoc -p "**/*.graphqls"

# Multiple extensions using brace expansion
graphqls-to-asciidoc -p "**/*.{graphql,graphqls,gql}"

# Specific subdirectory
graphqls-to-asciidoc -p "schemas/types/*.graphql"

# Complex pattern with prefix
graphqls-to-asciidoc -p "api/v1/**/*.{graphql,graphqls}"

# Absolute paths are fully supported
graphqls-to-asciidoc -p "/home/user/project/src/**/*.graphqls"

# Mixed relative and absolute patterns work
graphqls-to-asciidoc -p "/absolute/path/**/*.graphql"
```

**Important Notes:**
- **Absolute and Relative Paths**: Both absolute paths (e.g., `/home/user/schemas/**/*.graphqls`) and relative paths (e.g., `schemas/**/*.graphqls`) are fully supported
- **GraphQL Ordering**: GraphQL doesn't require specific ordering of type definitions - the parser handles dependencies automatically
- **Deterministic Processing**: Files are processed in alphabetical order for consistent output across runs
- **Conflict Detection**: Duplicate type definitions across files will trigger a clear error message
- **Debugging**: Use `--verbose` to see exactly which files are being combined and their processing order

### Command-Line Options

#### Core Options
| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--schema` | `-s` | Path to GraphQL schema file (single file mode) | - |
| `--pattern` | `-p` | Pattern to match multiple GraphQL schema files | - |
| `--output` | `-o` | Output file path | stdout |
| `--help` | `-h` | Show detailed help information | - |
| `--version` | `-v` | Show version information | - |

**Note:** Either `--schema` or `--pattern` is required, but not both.

#### Control Options
| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--catalogue` | - | Generate quick reference catalogue (queries, mutations, subscriptions tables only) | false |
| `--sub-title` | - | Optional subtitle for catalogue (e.g., 'Activities') | - |
| `--exclude-internal` | `-x` | Exclude queries/mutations marked as INTERNAL (deprecated, use `--inc-internal` instead) | false |
| `--verbose` | - | Enable verbose logging with processing metrics | false |

#### Filtering Options
| Flag | Description | Default |
|------|-------------|---------|
| `--inc-internal` | Include internal queries/mutations (those starting with 'internal' or marked INTERNAL) | false |
| `--inc-deprecated` | Include deprecated queries/mutations (those with @deprecated directive or marked deprecated) | false |
| `--inc-preview` | Include preview queries/mutations (those marked as PREVIEW) | false |
| `--inc-legacy` | Include legacy queries/mutations (those marked as LEGACY) | false |
| `--inc-zero` | Include items with version 0.0.0 or 0.0.0.0 | false |
| `--inc-changelog` | Include changelog information in catalogue descriptions | false |

#### Section Control
| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--queries` | `-q` | Include queries section | true |
| `--mutations` | `-m` | Include mutations section | true |
| `--subscriptions` | - | Include subscriptions section | false |
| `--types` | `-t` | Include types section | true |
| `--enums` | `-e` | Include enums section | true |
| `--inputs` | `-i` | Include inputs section | true |
| `--directives` | `-d` | Include directives section | true |
| `--scalars` | - | Include scalars section | true |

## GraphQL Schema Enhancements

The tool supports rich markup within GraphQL descriptions:

### Admonition Blocks
```graphql
"""
**NOTE**: This query requires authentication.

**WARNING**: Rate limiting applies to this endpoint.

TIP: Use pagination for better performance.
"""
```

### Code Callouts
```graphql
"""
Example usage:

```javascript
const client = new GraphQLClient(endpoint); (1)
const result = await client.request(query); // 2
console.log(result); # 3
```

(1) Initialize the client
(2) Execute the query  
(3) Display results
"""
```

### Anchors and Cross-References
```graphql
"""
[#user-management]
User management operations.

See {ref:authentication} for auth details.
For permissions, check {link:permissions|the permissions guide}.
"""
```

### Tables
```graphql
"""
Search parameters documentation with automatic table conversion:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| query | String | Yes | Search term to match |
| limit | Int | No | Maximum results (default: 20) |
| offset | Int | No | Starting position (default: 0) |

You can also use existing AsciiDoc table syntax:

[options="header"]
|===
| Setting | Default | Description
| timeout | 30s | Request timeout
| retries | 3 | Max retry attempts
|===
"""
```

### Changelog Annotations
```graphql
"""
User profile data.

add.version: 1.0.0
update.version: 1.2.3
deprecated.version: 2.0.0
"""
```

## Output Format

The generated AsciiDoc includes:

- **Document metadata** with generation timestamp and source file
- **Table of contents** with navigation
- **Comprehensive sections** for each GraphQL construct
- **Cross-referenced types** with clickable links  
- **Formatted tables** for fields, arguments, and parameters
- **AsciiDoc tags** for selective inclusion in larger documents
- **Professional styling** with consistent formatting

## Examples

The [test](test/) directory contains comprehensive examples:

### Single File Examples
- **Input**: [schema.graphql](test/schema.graphql) - Feature-rich GraphQL schema
- **Output**: [schema.adoc](test/schema.adoc) - Generated AsciiDoc documentation

### Multi-File Examples
- **Input**: [test/multi-schema/](test/multi-schema/) - Multiple schema files split by domain:
  - `schema.graphql` - Root Query, Mutation, and Subscription definitions
  - `users.graphql` - User-related types and inputs
  - `posts.graphqls` - Post-related types, enums, and inputs  
  - `scalars.gql` - Custom scalar definitions

```bash
# Generate documentation from the multi-file example
graphqls-to-asciidoc -p "test/multi-schema/*.{graphql,graphqls,gql}" -o multi-schema-docs.adoc --verbose

# Example with absolute path (useful for CI/CD pipelines)
graphqls-to-asciidoc -p "/absolute/path/to/schemas/**/*.graphqls" -o docs.adoc --verbose
```

## Development

### Requirements
- Go 1.19+ 
- Make

### Commands
```bash
# Build the application
make build

# Run tests
make test

# Generate test documentation
make test_doc

# Clean build artifacts  
make clean
```

### Testing
The project includes comprehensive unit tests covering:
- Description processing and markup conversion
- Table conversion (markdown to AsciiDoc) and preservation
- Admonition blocks, code callouts, and anchor processing
- Type name processing and cross-references
- Template rendering and output generation
- Command-line flag handling and validation
- Multi-file pattern matching and schema combining

## Troubleshooting

### Common Pattern Issues

**"No GraphQL schema files found matching pattern"**
- Verify the directory path exists: `ls -la /your/path/to/schemas/`
- Check file extensions are correct (`.graphql`, `.graphqls`, or `.gql`)
- Use `--verbose` to see the search process
- Test with a simpler pattern first: `graphqls-to-asciidoc -p "*.graphql"`

**"Duplicate definition error"**
- This occurs when the same type is defined in multiple files
- Use `--verbose` to see which files contain the conflict
- Consider splitting conflicting types into separate files or removing duplicates

**Pattern syntax examples:**
```bash
# ✅ Correct patterns
graphqls-to-asciidoc -p "schemas/**/*.graphqls"           # Recursive
graphqls-to-asciidoc -p "/home/user/api/**/*.{graphql,graphqls}"  # Absolute + brace expansion  
graphqls-to-asciidoc -p "src/graphql/*.graphql"          # Single directory

# ❌ Common mistakes
graphqls-to-asciidoc -p schemas/**/*.graphqls             # Missing quotes
graphqls-to-asciidoc -p "schemas/*/*.graphqls"           # Single * won't recurse
```

## Schema Requirements

Your GraphQL schema must be valid as the tool relies on [vektah/gqlparser](https://github.com/vektah/gqlparser) for parsing. The tool supports:

- Standard GraphQL syntax (SDL)
- Comments and descriptions
- Custom directives
- Complex type definitions
- Schema extensions

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality  
4. Ensure all tests pass with `make test`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [vektah/gqlparser](https://github.com/vektah/gqlparser) for GraphQL parsing
- Inspired by the need for professional GraphQL API documentation
- Uses Go's powerful template system for flexible output generation

# GraphQLS to AsciiDoc

A powerful CLI tool that converts GraphQL schema files (.graphqls) to comprehensive AsciiDoc documentation with rich formatting, cross-references, and professional styling.

## Features

### 🚀 Core Functionality
- **Complete GraphQL Support**: Queries, Mutations, Subscriptions, Types, Enums, Inputs, Directives, and Scalars
- **Rich Documentation**: Converts GraphQL descriptions to formatted AsciiDoc with tables, cross-references, and metadata
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
./graphqls-to-asciidoc -schema ./schema.graphqls > documentation.adoc
```

### Advanced Usage
```bash
# Exclude internal queries and disable mutations section
./graphqls-to-asciidoc -schema ./schema.graphql -exclude-internal -mutations=false > api-docs.adoc

# Generate only types and enums
./graphqls-to-asciidoc -schema ./schema.graphql -queries=false -mutations=false -subscriptions=false > types-only.adoc

# Check version
./graphqls-to-asciidoc -version
```

### Command-Line Options
| Flag | Description | Default |
|------|-------------|---------|
| `-schema` | Path to GraphQL schema file | **Required** |
| `-version` | Show version information | false |
| `-exclude-internal` | Exclude queries/mutations marked as INTERNAL | false |
| `-queries` | Include queries section | true |
| `-mutations` | Include mutations section | true |
| `-subscriptions` | Include subscriptions section | true |
| `-types` | Include types section | true |
| `-enums` | Include enums section | true |
| `-inputs` | Include inputs section | true |
| `-directives` | Include directives section | true |
| `-scalars` | Include scalars section | true |

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
- **Input**: [schema.graphql](test/schema.graphql) - Feature-rich GraphQL schema
- **Output**: [schema.adoc](test/schema.adoc) - Generated AsciiDoc documentation

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

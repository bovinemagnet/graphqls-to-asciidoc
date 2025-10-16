package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

var (
	Version   = "development"
	BuildTime = "unknown"
)

// Config holds all configuration options for the application
type Config struct {
	SchemaFile           string
	SchemaPattern        string
	OutputFile           string
	ExcludeInternal      bool // Deprecated: use IncludeInternal instead
	IncludeInternal      bool
	IncludeDeprecated    bool
	IncludePreview       bool
	IncludeLegacy        bool
	IncludeZeroVersion   bool
	IncludeMutations     bool
	IncludeQueries       bool
	IncludeSubscriptions bool
	IncludeDirectives    bool
	IncludeTypes         bool
	IncludeEnums         bool
	IncludeInputs        bool
	IncludeScalars       bool
	ShowVersion          bool
	ShowHelp             bool
	Verbose              bool
	Catalogue            bool
	SubTitle             string
	IncludeChangelog     bool
}

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		IncludeMutations:     true,
		IncludeQueries:       true,
		IncludeSubscriptions: false,
		IncludeDirectives:    true,
		IncludeTypes:         true,
		IncludeEnums:         true,
		IncludeInputs:        true,
		IncludeScalars:       true,
	}
}

// ParseFlags parses command-line flags and returns a Config
func ParseFlags() *Config {
	config := NewConfig()

	// Core flags with short aliases
	flag.StringVar(&config.SchemaFile, "schema", "", "Path to the GraphQL schema file")
	flag.StringVar(&config.SchemaFile, "s", "", "Path to the GraphQL schema file (shorthand)")
	flag.StringVar(&config.SchemaPattern, "pattern", "", "Pattern to match multiple GraphQL schema files (e.g., 'schemas/**/*.graphqls')")
	flag.StringVar(&config.SchemaPattern, "p", "", "Pattern to match multiple GraphQL schema files (shorthand)")
	flag.StringVar(&config.OutputFile, "output", "", "Output file path (default: stdout)")
	flag.StringVar(&config.OutputFile, "o", "", "Output file path (shorthand)")

	// Control flags
	flag.BoolVar(&config.ExcludeInternal, "exclude-internal", false, "Exclude internal queries from output (deprecated: use --inc-internal)")
	flag.BoolVar(&config.ExcludeInternal, "x", false, "Exclude internal queries from output (deprecated, shorthand)")
	flag.BoolVar(&config.IncludeInternal, "inc-internal", false, "Include internal queries/mutations (those starting with 'internal' or marked INTERNAL)")
	flag.BoolVar(&config.IncludeDeprecated, "inc-deprecated", false, "Include deprecated queries/mutations (those with @deprecated directive or marked deprecated)")
	flag.BoolVar(&config.IncludePreview, "inc-preview", false, "Include preview queries/mutations (those marked as PREVIEW or preview)")
	flag.BoolVar(&config.IncludeLegacy, "inc-legacy", false, "Include legacy queries/mutations (those marked as LEGACY or legacy)")
	flag.BoolVar(&config.IncludeZeroVersion, "inc-zero", false, "Include items with version 0.0.0 or 0.0.0.0 (by default, items marked with @version: 0.0.0 or @version: 0.0.0.0 are excluded)")
	flag.BoolVar(&config.IncludeChangelog, "inc-changelog", false, "Include changelog information in catalogue descriptions (version annotations)")
	flag.BoolVar(&config.ShowVersion, "version", false, "Show program version and build information")
	flag.BoolVar(&config.ShowVersion, "v", false, "Show program version and build information (shorthand)")
	flag.BoolVar(&config.ShowHelp, "help", false, "Show detailed help information")
	flag.BoolVar(&config.ShowHelp, "h", false, "Show detailed help information (shorthand)")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging with metrics")
	flag.BoolVar(&config.Catalogue, "catalogue", false, "Generate a catalogue table with query/mutation names and first sentence descriptions")
	flag.StringVar(&config.SubTitle, "sub-title", "", "Optional subtitle for catalogue (e.g., 'Activities')")

	// Section inclusion flags
	flag.BoolVar(&config.IncludeQueries, "queries", true, "Include queries in the output")
	flag.BoolVar(&config.IncludeQueries, "q", true, "Include queries in the output (shorthand)")
	flag.BoolVar(&config.IncludeMutations, "mutations", true, "Include mutations in the output")
	flag.BoolVar(&config.IncludeMutations, "m", true, "Include mutations in the output (shorthand)")
	flag.BoolVar(&config.IncludeSubscriptions, "subscriptions", false, "Include subscriptions in the output")
	flag.BoolVar(&config.IncludeTypes, "types", true, "Include types in the output")
	flag.BoolVar(&config.IncludeTypes, "t", true, "Include types in the output (shorthand)")
	flag.BoolVar(&config.IncludeEnums, "enums", true, "Include enums in the output")
	flag.BoolVar(&config.IncludeEnums, "e", true, "Include enums in the output (shorthand)")
	flag.BoolVar(&config.IncludeInputs, "inputs", true, "Include inputs in the output")
	flag.BoolVar(&config.IncludeInputs, "i", true, "Include inputs in the output (shorthand)")
	flag.BoolVar(&config.IncludeDirectives, "directives", true, "Include directives in the output")
	flag.BoolVar(&config.IncludeDirectives, "d", true, "Include directives in the output (shorthand)")
	flag.BoolVar(&config.IncludeScalars, "scalars", true, "Include scalars in the output")

	// Custom usage function
	flag.Usage = PrintUsage

	flag.Parse()

	return config
}

// HandleVersion handles the version flag display
func (c *Config) HandleVersion() bool {
	if c.ShowVersion {
		fmt.Printf("graphqls-to-asciidoc\n")
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Built with: %s\n", runtime.Version())
		return true
	}
	return false
}

// HandleHelp handles the help flag display
func (c *Config) HandleHelp() bool {
	if c.ShowHelp {
		PrintUsage()
		return true
	}
	return false
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Require either schema file or pattern, but not both
	if c.SchemaFile == "" && c.SchemaPattern == "" {
		return fmt.Errorf("either -schema or -pattern flag is required")
	}

	if c.SchemaFile != "" && c.SchemaPattern != "" {
		return fmt.Errorf("-schema and -pattern flags are mutually exclusive")
	}

	// Check if schema file exists (single file mode)
	if c.SchemaFile != "" {
		if _, err := os.Stat(c.SchemaFile); os.IsNotExist(err) {
			return fmt.Errorf("schema file '%s' does not exist", c.SchemaFile)
		}
	}

	// Validate output file directory if specified
	if c.OutputFile != "" {
		dir := c.OutputFile[:len(c.OutputFile)-len(filepath.Base(c.OutputFile))]
		if dir != "" && dir != "." {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return fmt.Errorf("output directory '%s' does not exist", dir)
			}
		}
	}

	return nil
}

// PrintUsage prints detailed usage information
func PrintUsage() {
	fmt.Printf(`graphqls-to-asciidoc - Convert GraphQL schema files to comprehensive AsciiDoc documentation

USAGE:
    graphqls-to-asciidoc [OPTIONS]

REQUIRED (choose one):
    -s, --schema PATH       Path to the GraphQL schema file
    -p, --pattern PATTERN   Pattern to match multiple GraphQL schema files

OPTIONS:
    -o, --output PATH       Output file path (default: stdout)
    -h, --help              Show this help information
    -v, --version           Show program version and build information
        --inc-internal      Include internal queries/mutations (by default, items starting with
                            'internal' or marked INTERNAL in description are excluded)
        --inc-deprecated    Include deprecated queries/mutations (by default, items with
                            @deprecated directive or marked deprecated are excluded)
        --inc-preview       Include preview queries/mutations (by default, items marked as
                            PREVIEW or preview are excluded)
        --inc-legacy        Include legacy queries/mutations (by default, items marked as
                            LEGACY or legacy are excluded)
        --inc-zero          Include items with version 0.0.0 or 0.0.0.0 (by default, items
                            marked with @version: 0.0.0 or @version: 0.0.0.0 are excluded)
        --inc-changelog     Include changelog information in catalogue descriptions
                            (extracts version annotations like add.version: 1.0.0)
        --verbose           Enable verbose logging with processing metrics
        --catalogue         Generate a catalogue table with query/mutation names and descriptions
        --sub-title TEXT    Optional subtitle for catalogue (e.g., 'Activities')

SECTION CONTROL:
    -q, --queries           Include queries in the output (default: true)
    -m, --mutations         Include mutations in the output (default: true)
        --subscriptions     Include subscriptions in the output (default: false)
    -t, --types             Include types in the output (default: true)
    -e, --enums             Include enums in the output (default: true)
    -i, --inputs            Include inputs in the output (default: true)
    -d, --directives        Include directives in the output (default: true)
        --scalars           Include scalars in the output (default: true)

EXAMPLES:
    # Generate documentation from single file to stdout
    graphqls-to-asciidoc -s schema.graphql

    # Generate documentation from multiple files
    graphqls-to-asciidoc -p "schemas/**/*.graphqls" -o docs.adoc

    # Generate from pattern with specific extensions
    graphqls-to-asciidoc -p "src/graphql/*.{graphql,graphqls}" -o api-docs.adoc

    # Generate only types and enums from single file
    graphqls-to-asciidoc -s schema.graphql -o types.adoc -q=false -m=false

    # Include internal queries (by default they are excluded)
    graphqls-to-asciidoc -p "**/*.graphqls" -o api-docs.adoc --inc-internal

    # Include deprecated, preview, and legacy items
    graphqls-to-asciidoc -s schema.graphql --inc-deprecated --inc-preview --inc-legacy

    # Generate comprehensive documentation with all sections
    graphqls-to-asciidoc -s schema.graphql -o full-docs.adoc --subscriptions

    # Generate with verbose logging and metrics
    graphqls-to-asciidoc -p "schemas/*.graphql" -o docs.adoc --verbose

    # Generate a catalogue table of queries and mutations
    graphqls-to-asciidoc -s schema.graphql --catalogue -o catalogue.adoc

    # Generate a catalogue with a subtitle
    graphqls-to-asciidoc -s schema.graphql --catalogue --sub-title "Activities" -o catalogue.adoc

FEATURES:
    ✓ Admonition blocks (NOTE, WARNING, TIP, etc.)
    ✓ Code callouts with automatic conversion
    ✓ Anchors and cross-references
    ✓ Table conversion (markdown to AsciiDoc)
    ✓ Professional AsciiDoc formatting
    ✓ Comprehensive type documentation

For more information, visit: https://github.com/bovinemagnet/graphqls-to-asciidoc
`)
}

// PrintError prints usage information for errors and exits
func PrintError(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n\n", msg)
	fmt.Fprintf(os.Stderr, "Use -h or --help for detailed usage information.\n")
	fmt.Fprintf(os.Stderr, "Quick start: graphqls-to-asciidoc -s schema.graphql\n")
	fmt.Fprintf(os.Stderr, "Or with pattern: graphqls-to-asciidoc -p \"**/*.graphqls\"\n")
	os.Exit(1)
}

// GetOutputWriter returns either stdout or a file writer based on configuration
func (c *Config) GetOutputWriter() (*os.File, bool, error) {
	if c.OutputFile == "" {
		// Return stdout, not a file to close
		return os.Stdout, false, nil
	}

	file, err := os.Create(c.OutputFile)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create output file '%s': %v", c.OutputFile, err)
	}

	// Return file, needs to be closed
	return file, true, nil
}

package config

import (
	"flag"
	"fmt"
	"os"
	"runtime"
)

var (
	Version   = "development"
	BuildTime = "unknown"
)

// Config holds all configuration options for the application
type Config struct {
	SchemaFile          string
	ExcludeInternal     bool
	IncludeMutations    bool
	IncludeQueries      bool
	IncludeSubscriptions bool
	IncludeDirectives   bool
	IncludeTypes        bool
	IncludeEnums        bool
	IncludeInputs       bool
	IncludeScalars      bool
	ShowVersion         bool
}

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		IncludeMutations:    true,
		IncludeQueries:      true,
		IncludeSubscriptions: false,
		IncludeDirectives:   true,
		IncludeTypes:        true,
		IncludeEnums:        true,
		IncludeInputs:       true,
		IncludeScalars:      true,
	}
}

// ParseFlags parses command-line flags and returns a Config
func ParseFlags() *Config {
	config := NewConfig()

	flag.StringVar(&config.SchemaFile, "schema", "", "Path to the GraphQL schema file")
	flag.BoolVar(&config.ExcludeInternal, "exclude-internal", false, "Exclude internal queries from output")
	flag.BoolVar(&config.IncludeMutations, "mutations", true, "Include mutations in the output")
	flag.BoolVar(&config.IncludeQueries, "queries", true, "Include queries in the output")
	flag.BoolVar(&config.IncludeSubscriptions, "subscriptions", false, "Include subscriptions in the output")
	flag.BoolVar(&config.IncludeDirectives, "directives", true, "Include directives in the output")
	flag.BoolVar(&config.IncludeTypes, "types", true, "Include types in the output")
	flag.BoolVar(&config.IncludeEnums, "enums", true, "Include enums in the output")
	flag.BoolVar(&config.IncludeInputs, "inputs", true, "Include inputs in the output")
	flag.BoolVar(&config.IncludeScalars, "scalars", true, "Include scalars in the output")
	flag.BoolVar(&config.ShowVersion, "version", false, "Show program version and build information")

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

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.SchemaFile == "" {
		return fmt.Errorf("error: -schema flag is required")
	}
	return nil
}

// PrintUsage prints usage information and exits
func PrintUsage() {
	fmt.Fprintf(os.Stderr, "Error: -schema flag is required\n")
	flag.Usage()
	os.Exit(1)
}
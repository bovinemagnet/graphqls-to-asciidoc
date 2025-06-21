package main

import (
	"log"
	"os"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/generator"
)

var (
	Version   = "development"
	BuildTime = "unknown"
)

func init() {
	// Set version variables in config package
	config.Version = Version
	config.BuildTime = BuildTime
}

func main() {
	// Parse configuration
	cfg := config.ParseFlags()

	// Handle version flag
	if cfg.HandleVersion() {
		os.Exit(0)
	}

	// Handle help flag
	if cfg.HandleHelp() {
		os.Exit(0)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		config.PrintError(err.Error())
	}

	// Read schema file
	schemaBytes, err := os.ReadFile(cfg.SchemaFile)
	if err != nil {
		log.Fatalf("Failed to read schema file %s: %v", cfg.SchemaFile, err)
	}

	// Parse GraphQL schema
	source := &ast.Source{
		Name:  "GraphQL schema",
		Input: string(schemaBytes),
	}

	doc, gqlErr := parser.ParseSchema(source)
	if gqlErr != nil {
		log.Fatalf("Failed to parse GraphQL schema: %v", gqlErr)
	}

	// Convert document to schema-like structure for generator
	// For now, let's create a simple schema from the doc
	schema := &ast.Schema{
		Types: make(map[string]*ast.Definition),
	}
	
	for _, def := range doc.Definitions {
		schema.Types[def.Name] = def
		
		// Identify special root types
		switch def.Name {
		case "Query":
			schema.Query = def
		case "Mutation":
			schema.Mutation = def
		case "Subscription":
			schema.Subscription = def
		}
	}

	// Get output writer
	outputWriter, shouldClose, err := cfg.GetOutputWriter()
	if err != nil {
		log.Fatalf("Failed to setup output: %v", err)
	}
	if shouldClose {
		defer outputWriter.Close()
	}

	// Generate AsciiDoc documentation
	gen := generator.New(cfg, schema, outputWriter)
	if err := gen.Generate(); err != nil {
		log.Fatalf("Failed to generate documentation: %v", err)
	}
}
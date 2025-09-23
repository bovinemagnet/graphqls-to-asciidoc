package main

import (
	"log"
	"os"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/generator"
	schemaParser "github.com/bovinemagnet/graphqls-to-asciidoc/pkg/parser"
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

	// Read schema content (single file or multiple files)
	var schemaContent string
	if cfg.SchemaPattern != "" {
		// Multi-file mode using pattern
		files, err := schemaParser.FindSchemaFiles(cfg.SchemaPattern)
		if err != nil {
			log.Fatalf("Failed to find schema files with pattern '%s': %v", cfg.SchemaPattern, err)
		}
		
		// Validate files are accessible and have correct extensions
		if err := schemaParser.ValidateSchemaFiles(files); err != nil {
			log.Fatalf("Schema file validation failed: %v", err)
		}
		
		// Combine multiple schema files
		schemaContent, err = schemaParser.CombineSchemaFiles(files)
		if err != nil {
			log.Fatalf("Failed to combine schema files: %v", err)
		}
		
		if cfg.Verbose {
			log.Printf("Combined %d schema files: %v", len(files), files)
		}
	} else {
		// Single file mode
		schemaBytes, err := os.ReadFile(cfg.SchemaFile)
		if err != nil {
			log.Fatalf("Failed to read schema file %s: %v", cfg.SchemaFile, err)
		}
		schemaContent = string(schemaBytes)
	}

	// Remove fragments from schema content before parsing
	// Fragments are client-side constructs and don't belong in schema files
	cleanedSchema := schemaParser.RemoveFragments(schemaContent)
	
	if cfg.Verbose && cleanedSchema != schemaContent {
		log.Printf("Removed fragment definitions from schema")
	}
	
	// Strip code blocks from descriptions to prevent parser from treating
	// example code as actual schema definitions
	strippedSchema := schemaParser.StripCodeBlocksFromDescriptions(cleanedSchema)
	
	if cfg.Verbose && strippedSchema != cleanedSchema {
		log.Printf("Stripped code blocks from descriptions to prevent duplicate definitions")
	}
	
	// Parse GraphQL schema (without fragments and code blocks in descriptions)
	source := &ast.Source{
		Name:  "GraphQL schema",
		Input: strippedSchema,
	}

	doc, gqlErr := parser.ParseSchema(source)
	if gqlErr != nil {
		log.Fatalf("Failed to parse GraphQL schema: %v", gqlErr)
	}
	
	// Now parse the original schema (with code blocks intact) to get the full descriptions
	originalSource := &ast.Source{
		Name:  "GraphQL schema",
		Input: cleanedSchema,
	}
	
	originalDoc, originalErr := parser.ParseSchema(originalSource)
	if originalErr != nil {
		// Fall back to using the stripped version if original fails
		originalDoc = doc
	}

	// Convert document to schema-like structure for generator
	// For now, let's create a simple schema from the doc
	schema := &ast.Schema{
		Types:      make(map[string]*ast.Definition),
		Directives: make(map[string]*ast.DirectiveDefinition),
	}
	
	for _, def := range doc.Definitions {
		// Find corresponding definition in original to get full description
		for _, origDef := range originalDoc.Definitions {
			if origDef.Name == def.Name {
				// Restore original description
				def.Description = origDef.Description
				// Also restore field descriptions
				for _, field := range def.Fields {
					for _, origField := range origDef.Fields {
						if origField.Name == field.Name {
							field.Description = origField.Description
							break
						}
					}
				}
				break
			}
		}
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
	
	// Handle directive definitions
	for _, def := range doc.Directives {
		schema.Directives[def.Name] = def
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
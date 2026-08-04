// Package build turns a configuration into a rendered AsciiDoc document. It
// exists so that both the one-shot CLI and the daemon share a single pipeline
// that reports failures as errors rather than exiting the process.
package build

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/vektah/gqlparser/v2/ast"
	gqlparser "github.com/vektah/gqlparser/v2/parser"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/generator"
	schemaParser "github.com/bovinemagnet/graphqls-to-asciidoc/pkg/parser"
)

// schemaSourceName labels the combined schema passed to the GraphQL parser.
const schemaSourceName = "GraphQL schema"

// Result is a rendered document together with the sources it came from.
type Result struct {
	Content  []byte
	Files    []string
	Duration time.Duration
}

// ResolveFiles returns the schema files named by the configuration, expanding
// the glob pattern when one was given.
func ResolveFiles(cfg *config.Config) ([]string, error) {
	if cfg.SchemaPattern == "" {
		return []string{cfg.SchemaFile}, nil
	}

	files, err := schemaParser.FindSchemaFiles(cfg.SchemaPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find schema files with pattern '%s': %w", cfg.SchemaPattern, err)
	}
	if err := schemaParser.ValidateSchemaFiles(files); err != nil {
		return nil, fmt.Errorf("schema file validation failed: %w", err)
	}
	return files, nil
}

// readSchemaContent loads and combines the schema files.
func readSchemaContent(cfg *config.Config, files []string) (string, error) {
	if cfg.SchemaPattern == "" {
		// #nosec G304 -- the path is the user's own -schema argument, already
		// checked by Validate; reading it is what this tool is for.
		schemaBytes, err := os.ReadFile(cfg.SchemaFile)
		if err != nil {
			return "", fmt.Errorf("failed to read schema file %s: %w", cfg.SchemaFile, err)
		}
		return string(schemaBytes), nil
	}

	content, err := schemaParser.CombineSchemaFiles(files)
	if err != nil {
		return "", fmt.Errorf("failed to combine schema files: %w", err)
	}
	if cfg.Verbose {
		log.Printf("Combined %d schema files: %v", len(files), files)
	}
	return content, nil
}

// Run reads, parses and renders the configured schema, returning the document
// bytes. Every failure is returned rather than fatal, so callers that run
// repeatedly can report a bad schema and carry on.
func Run(cfg *config.Config) (*Result, error) {
	started := time.Now()

	files, err := ResolveFiles(cfg)
	if err != nil {
		return nil, err
	}

	schemaContent, err := readSchemaContent(cfg, files)
	if err != nil {
		return nil, err
	}

	// Fragments are client-side constructs and do not belong in schema files.
	cleanedSchema := schemaParser.RemoveFragments(schemaContent)
	if cfg.Verbose && cleanedSchema != schemaContent {
		log.Printf("Removed fragment definitions from schema")
	}

	// Code blocks in descriptions are safe to parse directly because they sit
	// inside triple-quoted strings.
	doc, gqlErr := gqlparser.ParseSchema(&ast.Source{Name: schemaSourceName, Input: cleanedSchema})
	if gqlErr != nil {
		return nil, fmt.Errorf("failed to parse GraphQL schema: %w", gqlErr)
	}

	schema := schemaParser.BuildSchema(doc)

	var out bytes.Buffer
	if err := generator.New(cfg, schema, &out).Generate(); err != nil {
		return nil, fmt.Errorf("failed to generate documentation: %w", err)
	}

	return &Result{Content: out.Bytes(), Files: files, Duration: time.Since(started)}, nil
}

package build

import (
	"strings"
	"testing"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
)

func TestResolveFilesSingleFile(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaFile = "../../test/schema.graphql"

	files, err := ResolveFiles(cfg)
	if err != nil {
		t.Fatalf("ResolveFiles returned an error: %v", err)
	}
	if len(files) != 1 || files[0] != "../../test/schema.graphql" {
		t.Fatalf("expected the single schema file, got %v", files)
	}
}

func TestResolveFilesPattern(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaPattern = "../../test/multi-schema/*.graphqls"

	files, err := ResolveFiles(cfg)
	if err != nil {
		t.Fatalf("ResolveFiles returned an error: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected the pattern to match at least one file")
	}
}

func TestRunProducesDocument(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaFile = "../../test/schema.graphql"

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	if !strings.HasPrefix(string(result.Content), "= GraphQL Documentation") {
		t.Fatalf("expected a document header, got %.40q", result.Content)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected one source file, got %v", result.Files)
	}
}

// TestRunReturnsErrorOnBadSchema is the regression guard for the log.Fatalf
// extraction: a malformed schema must return an error, never exit the process.
func TestRunReturnsErrorOnBadSchema(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaFile = "../../test/invalid.graphqls"

	if _, err := Run(cfg); err == nil {
		t.Fatal("expected a parse error for the malformed schema, got nil")
	}
}

func TestRunReturnsErrorOnMissingFile(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaFile = "../../test/does-not-exist.graphqls"

	if _, err := Run(cfg); err == nil {
		t.Fatal("expected an error for a missing schema file, got nil")
	}
}

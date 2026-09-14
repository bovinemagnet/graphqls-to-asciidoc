package generator

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
	gqlparser "github.com/vektah/gqlparser/v2/parser"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/parser"
)

// parseTestSchema builds an ast.Schema from GraphQL SDL for catalogue tests.
func parseTestSchema(t *testing.T, sdl string) *ast.Schema {
	t.Helper()
	doc, err := gqlparser.ParseSchema(&ast.Source{Name: "test.graphql", Input: sdl})
	if err != nil {
		t.Fatalf("failed to parse test schema: %v", err)
	}
	return parser.BuildSchema(doc)
}

// catalogueTables returns the AsciiDoc table blocks (|=== ... |===) found in output.
func catalogueTables(output string) []string {
	var tables []string
	rest := output
	for {
		start := strings.Index(rest, "|===")
		if start < 0 {
			return tables
		}
		end := strings.Index(rest[start+4:], "|===")
		if end < 0 {
			return tables
		}
		tables = append(tables, rest[start:start+4+end+4])
		rest = rest[start+4+end+4:]
	}
}

const catalogueSharedSDL = `
type Query {
  """
  Fetch a user.

  add.version: 1.2.0
  """
  user(id: ID!): User
  "List users."
  users: [User!]!
}
type Mutation {
  "Add a user."
  addUser(name: String!): User
  "Remove a user."
  deleteUser(id: ID!): Boolean
}
type Subscription {
  "Emits when a user changes."
  userChanged: User
}
type User { id: ID! name: String }
`

func TestCatalogueTablesMatchBetweenStandaloneAndFullDocument(t *testing.T) {
	var standalone, full bytes.Buffer

	cfg := config.NewConfig()
	cfg.SchemaFile = "test.graphql"
	cfg.IncludeChangelog = true
	cfg.Catalogue = true
	if err := New(cfg, parseTestSchema(t, catalogueSharedSDL), &standalone).Generate(); err != nil {
		t.Fatalf("standalone Generate() error: %v", err)
	}

	cfg = config.NewConfig()
	cfg.SchemaFile = "test.graphql"
	cfg.IncludeChangelog = true
	cfg.IncludeSubscriptions = true
	if err := New(cfg, parseTestSchema(t, catalogueSharedSDL), &full).Generate(); err != nil {
		t.Fatalf("full Generate() error: %v", err)
	}

	standaloneTables := catalogueTables(standalone.String())
	fullTables := catalogueTables(full.String())
	if len(standaloneTables) != 3 {
		t.Fatalf("standalone catalogue should have 3 tables, got %d", len(standaloneTables))
	}
	// The full document has further tables after the catalogue summary; only
	// the first three are the catalogue.
	if len(fullTables) < 3 {
		t.Fatalf("full document should have at least 3 tables, got %d", len(fullTables))
	}
	for i := range standaloneTables {
		if standaloneTables[i] != fullTables[i] {
			t.Errorf("catalogue table %d differs between modes:\nstandalone:\n%s\nfull:\n%s",
				i, standaloneTables[i], fullTables[i])
		}
	}
}

func TestCatalogueSubscriptionsIntroPresentInFullDocument(t *testing.T) {
	var full bytes.Buffer
	cfg := config.NewConfig()
	cfg.SchemaFile = "test.graphql"
	cfg.IncludeSubscriptions = true
	if err := New(cfg, parseTestSchema(t, catalogueSharedSDL), &full).Generate(); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if !strings.Contains(full.String(), "*Subscriptions* are used to *receive real-time updates*") {
		t.Error("full document catalogue should carry the subscriptions intro paragraph")
	}
}

func TestCatalogueStandaloneShowsNoteWhenNoQueries(t *testing.T) {
	var out bytes.Buffer
	cfg := config.NewConfig()
	cfg.SchemaFile = "test.graphql"
	cfg.Catalogue = true
	sdl := `type Mutation { "Add." addThing: Boolean }`
	if err := New(cfg, parseTestSchema(t, sdl), &out).Generate(); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if !strings.Contains(out.String(), "No queries exist in this schema.") {
		t.Error("standalone catalogue should note when no queries exist")
	}
}

func TestCatalogueEntrySignature(t *testing.T) {
	sdl := `
type Query {
  "Fetch."
  user(id: ID!, includeDeleted: Boolean = false, tags: [String!]): User
  "Ping."
  ping: String!
}
type User { id: ID! }
`
	cfg := config.NewConfig()
	cfg.SchemaFile = "test.graphql"
	g := New(cfg, parseTestSchema(t, sdl), &bytes.Buffer{})

	data := g.collectCatalogueData()
	got := map[string]string{}
	for _, e := range data.Queries {
		got[e.Name] = e.Signature
	}
	want := map[string]string{
		"user": "user(id: ID!, includeDeleted: Boolean = false, tags: [String!]): User",
		"ping": "ping: String!",
	}
	for name, sig := range want {
		if got[name] != sig {
			t.Errorf("signature for %s = %q, want %q", name, got[name], sig)
		}
	}
}

func TestCatalogueTableShowsSignature(t *testing.T) {
	var out bytes.Buffer
	cfg := config.NewConfig()
	cfg.SchemaFile = "test.graphql"
	cfg.Catalogue = true
	if err := New(cfg, parseTestSchema(t, catalogueSharedSDL), &out).Generate(); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	for _, row := range []string{
		"| user(id: ID!): User | Fetch a user.",
		"| addUser(name: String!): User | Add a user.",
		"| userChanged: User | Emits when a user changes.",
	} {
		if !strings.Contains(out.String(), row) {
			t.Errorf("catalogue should contain row %q", row)
		}
	}
}

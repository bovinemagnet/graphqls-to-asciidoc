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

func TestCatalogueTableOmitsSignatureByDefault(t *testing.T) {
	var out bytes.Buffer
	cfg := config.NewConfig()
	cfg.SchemaFile = "test.graphql"
	cfg.Catalogue = true
	if err := New(cfg, parseTestSchema(t, catalogueSharedSDL), &out).Generate(); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if strings.Contains(out.String(), "`user(id: ID!): User`") {
		t.Error("catalogue should not contain the signature without --inc-signature")
	}
	if !strings.Contains(out.String(), "| user | Fetch a user.\n| users |") {
		t.Errorf("rows should follow one another directly without the signature line:\n%s", out.String())
	}
}

func TestCatalogueTableShowsSignature(t *testing.T) {
	var out bytes.Buffer
	cfg := config.NewConfig()
	cfg.SchemaFile = "test.graphql"
	cfg.Catalogue = true
	cfg.IncludeSignature = true
	if err := New(cfg, parseTestSchema(t, catalogueSharedSDL), &out).Generate(); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	for _, row := range []string{
		"| user | Fetch a user.\n\n`user(id: ID!): User`\n",
		"| addUser | Add a user.\n\n`addUser(name: String!): User`\n",
		"| userChanged | Emits when a user changes.\n\n`userChanged: User`\n",
	} {
		if !strings.Contains(out.String(), row) {
			t.Errorf("catalogue should contain row %q", row)
		}
	}
}

const catalogueStatusSDL = `
type Query {
  "Fetch."
  user(id: ID!): User
  "Old way. DEPRECATED, use user instead."
  oldUser(id: ID!): User @deprecated(reason: "use user")
  "PREVIEW: experimental search."
  search(term: String!): [User!]!
  "LEGACY lookup."
  legacyLookup: User
  "Internal health probe."
  internalPing: Boolean
}
type User { id: ID! }
`

func TestCatalogueEntryStatus(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SchemaFile = "test.graphql"
	cfg.IncludeDeprecated = true
	cfg.IncludePreview = true
	cfg.IncludeLegacy = true
	cfg.IncludeInternal = true
	g := New(cfg, parseTestSchema(t, catalogueStatusSDL), &bytes.Buffer{})

	got := map[string][]string{}
	for _, e := range g.collectCatalogueData().Queries {
		got[e.Name] = e.Status
	}
	want := map[string][]string{
		"user":         nil,
		"oldUser":      {"DEPRECATED"},
		"search":       {"PREVIEW"},
		"legacyLookup": {"LEGACY"},
		"internalPing": {"INTERNAL"},
	}
	for name, status := range want {
		if strings.Join(got[name], ",") != strings.Join(status, ",") {
			t.Errorf("status for %s = %v, want %v", name, got[name], status)
		}
	}
}

func TestCatalogueTableShowsStatusBadges(t *testing.T) {
	var out bytes.Buffer
	cfg := config.NewConfig()
	cfg.SchemaFile = "test.graphql"
	cfg.Catalogue = true
	cfg.IncludeSignature = true
	cfg.IncludeDeprecated = true
	cfg.IncludePreview = true
	if err := New(cfg, parseTestSchema(t, catalogueStatusSDL), &out).Generate(); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	for _, row := range []string{
		"| user | Fetch.\n\n`user(id: ID!): User`\n",
		"| oldUser | *DEPRECATED* Old way.\n\n`oldUser(id: ID!): User`\n",
		"| search | *PREVIEW* PREVIEW: experimental search.\n\n`search(term: String!): [User!]!`\n",
	} {
		if !strings.Contains(out.String(), row) {
			t.Errorf("catalogue should contain row %q\n%s", row, out.String())
		}
	}
}

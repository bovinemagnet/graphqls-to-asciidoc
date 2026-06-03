package parser

import (
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

// parseDoc is a test helper that parses raw SDL into a SchemaDocument.
func parseDoc(t *testing.T, sdl string) *ast.SchemaDocument {
	t.Helper()
	doc, err := parser.ParseSchema(&ast.Source{Name: "test", Input: sdl})
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}
	return doc
}

// hasField reports whether a definition contains a field with the given name.
func hasField(def *ast.Definition, name string) bool {
	if def == nil {
		return false
	}
	for _, f := range def.Fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

func TestBuildSchema_ExtendQuery(t *testing.T) {
	doc := parseDoc(t, `
type Query {
  base: String
}
extend type Query {
  added: String
}
`)
	schema := BuildSchema(doc)

	if schema.Query == nil {
		t.Fatal("expected Query root type to be set")
	}
	if len(schema.Query.Fields) != 2 {
		t.Fatalf("expected 2 query fields after merge, got %d", len(schema.Query.Fields))
	}
	if !hasField(schema.Query, "base") || !hasField(schema.Query, "added") {
		t.Errorf("expected both 'base' and 'added' fields, got %v", schema.Query.Fields)
	}
}

func TestBuildSchema_ExtendMutation(t *testing.T) {
	doc := parseDoc(t, `
type Mutation {
  doBase: String
}
extend type Mutation {
  doAdded: String
}
`)
	schema := BuildSchema(doc)

	if schema.Mutation == nil {
		t.Fatal("expected Mutation root type to be set")
	}
	if !hasField(schema.Mutation, "doBase") || !hasField(schema.Mutation, "doAdded") {
		t.Errorf("expected both mutation fields, got %v", schema.Mutation.Fields)
	}
}

func TestBuildSchema_ExtendSubscription(t *testing.T) {
	doc := parseDoc(t, `
type Subscription {
  onBase: String
}
extend type Subscription {
  onAdded: String
}
`)
	schema := BuildSchema(doc)

	if schema.Subscription == nil {
		t.Fatal("expected Subscription root type to be set")
	}
	if !hasField(schema.Subscription, "onBase") || !hasField(schema.Subscription, "onAdded") {
		t.Errorf("expected both subscription fields, got %v", schema.Subscription.Fields)
	}
}

func TestBuildSchema_ExtendRegularType(t *testing.T) {
	doc := parseDoc(t, `
type User {
  id: ID!
}
extend type User {
  name: String
}
`)
	schema := BuildSchema(doc)

	user := schema.Types["User"]
	if user == nil {
		t.Fatal("expected User type to be present")
	}
	if !hasField(user, "id") || !hasField(user, "name") {
		t.Errorf("expected merged User fields 'id' and 'name', got %v", user.Fields)
	}
}

func TestBuildSchema_ExtendEnum(t *testing.T) {
	doc := parseDoc(t, `
enum Colour {
  RED
}
extend enum Colour {
  GREEN
}
`)
	schema := BuildSchema(doc)

	colour := schema.Types["Colour"]
	if colour == nil {
		t.Fatal("expected Colour enum to be present")
	}
	if len(colour.EnumValues) != 2 {
		t.Errorf("expected 2 enum values after merge, got %d", len(colour.EnumValues))
	}
}

func TestBuildSchema_OrphanExtension(t *testing.T) {
	// An extension with no matching base definition should be promoted.
	doc := parseDoc(t, `
extend type Orphan {
  field: String
}
`)
	schema := BuildSchema(doc)

	orphan := schema.Types["Orphan"]
	if orphan == nil {
		t.Fatal("expected orphan extension to be promoted to a definition")
	}
	if !hasField(orphan, "field") {
		t.Errorf("expected orphan 'field', got %v", orphan.Fields)
	}
}

func TestBuildSchema_NoExtensions(t *testing.T) {
	// Regression: schemas without extensions behave as before.
	doc := parseDoc(t, `
type Query {
  hello: String
}
directive @auth on FIELD_DEFINITION
`)
	schema := BuildSchema(doc)

	if schema.Query == nil || !hasField(schema.Query, "hello") {
		t.Error("expected Query.hello to be present")
	}
	if schema.Types["Query"] == nil {
		t.Error("expected Query in Types map")
	}
	if _, ok := schema.Directives["auth"]; !ok {
		t.Error("expected @auth directive to be registered")
	}
}

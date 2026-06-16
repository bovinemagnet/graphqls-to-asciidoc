package parser

import "github.com/vektah/gqlparser/v2/ast"

// BuildSchema converts a raw parsed SchemaDocument into an ast.Schema.
//
// parser.ParseSchema keeps base type definitions (e.g. `type Query { ... }`)
// in doc.Definitions and type extensions (e.g. `extend type Query { ... }`)
// separately in doc.Extensions. BuildSchema merges each extension into its
// matching base definition so that extended fields, interfaces, union members,
// enum values, and directives are visible to the generator. An extension with
// no matching base definition is promoted to a definition in its own right.
//
// This keeps the lenient raw-parse behaviour (no full schema validation) while
// honouring the GraphQL `extend` construct.
func BuildSchema(doc *ast.SchemaDocument) *ast.Schema {
	schema := &ast.Schema{
		Types:      make(map[string]*ast.Definition),
		Directives: make(map[string]*ast.DirectiveDefinition),
	}

	for _, def := range doc.Definitions {
		schema.Types[def.Name] = def
	}

	// Merge extensions into their base definitions (or promote orphans).
	for _, ext := range doc.Extensions {
		if base, ok := schema.Types[ext.Name]; ok {
			base.Fields = append(base.Fields, ext.Fields...)
			base.Interfaces = append(base.Interfaces, ext.Interfaces...)
			base.Types = append(base.Types, ext.Types...)
			base.EnumValues = append(base.EnumValues, ext.EnumValues...)
			base.Directives = append(base.Directives, ext.Directives...)
		} else {
			schema.Types[ext.Name] = ext
		}
	}

	// Wire up root operation types from the merged map. A missing entry yields
	// nil, matching the previous behaviour when the type was absent.
	schema.Query = schema.Types["Query"]
	schema.Mutation = schema.Types["Mutation"]
	schema.Subscription = schema.Types["Subscription"]

	for _, def := range doc.Directives {
		schema.Directives[def.Name] = def
	}

	return schema
}

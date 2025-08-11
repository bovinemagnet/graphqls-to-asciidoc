package templates

const FieldTemplate = `
| {{.Type}} | {{.Name}} | {{processDescription .Description}}
{{- if .RequiredOrArray}}

.Notes:
{{- end}}
{{- if .Required}}

.Required:
* {{.Required}}
{{- end}}
{{- if .IsArray}}
.Array:
* True
{{- end}}
{{- if .Directives}}

.Directives:
{{.Directives}}
{{- end }}
{{- if .Changelog}}
{{.Changelog}}
{{- end }}
`

const ScalarTemplate = `
// tag::scalar[]
[[scalars]]
{{.ScalarTag}}

GraphQL specifies a basic set of well-defined Scalar types: Int, Float, String, Boolean, and ID.
{{- if .FoundScalars }}

The following custom scalar types are defined in this schema:

{{- range .Scalars }}
// tag::scalar-{{.Name}}[]
[[scalar-{{.Name}}]]
=== {{.Name}}

{{- if .Description }}

// tag::scalar-description-{{.Name}}[]
{{ .Description | printAsciiDocTagsTmpl }}
// end::scalar-description-{{.Name}}[]

{{ end }}
// end::scalar-{{.Name}}[]

{{ end }}
{{- else }}
[NOTE]
====
No custom scalars exist in this schema.
====
{{ end }}
// end::scalar[]
`

const SubscriptionTemplate = `
// tag::subscription[]
== Subscription

{{- if .FoundSubscriptions }}
{{- range .Subscriptions }}
{{- if .Description }}
{{ .Description | printAsciiDocTagsTmpl }}
{{ end }}

{{ .Details }}

{{ end }}
{{- else }}
[NOTE]
====
No subscriptions exist in this schema.
====
{{ end }}
// end::subscription[]
`

const MutationTemplate = `
// tag::mutation[]
[[mutations]]
{{.MutationTag}}

{{- if .MutationObjectDescription }}
{{ .MutationObjectDescription | printAsciiDocTagsTmpl }}
{{- end }}

GraphQL Mutations are entry points on a GraphQL server that provides write access to our data sources.

{{- if .FoundMutations }}

{{- range .Mutations }}
// tag::mutation-{{.Name}}[]
[[{{.AnchorName}}]]
=== {{.Name}}{{ if .IsInternal }} [INTERNAL]{{ end }}

// tag::method-description-{{.Name}}[]
{{- if .CleanedDescription }}
{{ .CleanedDescription | printAsciiDocTagsTmpl }}
{{- end }}
// end::method-description-{{.Name}}[]

// tag::method-signature-{{.Name}}[]
{{ .MethodSignatureBlock }}
// end::method-signature-{{.Name}}[]

// tag::method-args-{{.Name}}[]
{{- if .NumberedRefs }}
{{ .NumberedRefs }}
{{- end }}
// end::method-args-{{.Name}}[]

// tag::mutation-name-{{.Name}}[]
*Mutation Name:* _{{ .Name }}_
// end::mutation-name-{{.Name}}[]

// tag::mutation-return-{{.Name}}[]
*Return:* {{ .TypeName }}
// end::mutation-return-{{.Name}}[]

{{- if .Changelog }}
// tag::mutation-changelog-{{.Name}}[]
{{ .Changelog }}
// end::mutation-changelog-{{.Name}}[]
{{- end }}

{{- if .HasArguments }}
// tag::arguments-{{.Name}}[]
.Arguments
{{ .Arguments }}
// end::arguments-{{.Name}}[]
{{- end }}

{{- if .HasDirectives }}
// tag::mutation-directives-{{.Name}}[]
.Directives
{{ .Directives }}
// end::mutation-directives-{{.Name}}[]
{{- end }}

// end::mutation-{{.Name}}[]
{{ end }}
{{- else }}
[NOTE]
====
No mutations exist in this schema.
====
{{ end }}
// end::mutation[]
`

const TypeSectionTemplate = `
{{.TypesTag}}
{{range .Types}}
// tag::type-{{.Name}}[]
[[{{.AnchorName}}]]
=== {{.Name}}

{{- if .Description }}
// tag::type-description-{{.Name}}[]
{{ .Description | printAsciiDocTagsTmpl }}
// end::type-description-{{.Name}}[]
{{- end }}

{{- if .Changelog }}
// tag::type-changelog-{{.Name}}[]
{{ .Changelog }}
// end::type-changelog-{{.Name}}[]
{{- end }}

// tag::type-def-{{.Name}}[]
{{ .FieldsTable }}
// end::type-def-{{.Name}}[]

// end::type-{{.Name}}[]

{{end}}
`

const EnumSectionTemplate = `
{{.EnumsTag}}
{{range .Enums}}
// tag::enum-{{.Name}}[]
[[{{.AnchorName}}]]

=== {{.Name}}

{{- if .Description }}
// tag::enum-description-{{.Name}}[]
{{ .Description | printAsciiDocTagsTmpl }}
// end::enum-description-{{.Name}}[]
{{- end }}

// tag::enum-def-{{.Name}}[]
{{ .ValuesTable }}
// end::enum-def-{{.Name}}[]

// end::enum-{{.Name}}[]

{{end}}
`

const DirectiveSectionTemplate = `
{{.DirectivesTag}}
{{- if .FoundDirectives }}

{{ .TableOptions }}
|===
| Directive | Arguments | Description
{{- range .Directives }}
| @{{.Name}} | {{.Arguments}} | {{.Description}}
{{- end }}
|===

{{- else }}
[NOTE]
====
No custom directives exist in this schema.
====
{{- end }}
`

const InputSectionTemplate = `
{{.InputsTag}}
{{range .Inputs}}
// tag::input-{{.Name}}[]
[[{{.AnchorName}}]]
=== {{.Name}}

{{- if .Description }}
// tag::input-description-{{.Name}}[]
{{ .Description | printAsciiDocTagsTmpl }}
// end::input-description-{{.Name}}[]
{{- end }}

{{- if .Changelog }}
// tag::input-changelog-{{.Name}}[]
{{ .Changelog }}
// end::input-changelog-{{.Name}}[]
{{- end }}

// tag::input-def-{{.Name}}[]
{{ .FieldsTable }}
// end::input-def-{{.Name}}[]

// end::input-{{.Name}}[]

{{end}}
`

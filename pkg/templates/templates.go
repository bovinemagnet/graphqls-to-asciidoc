package templates

const FieldTemplate = `
| {{.Type}} | {{.Name}} | {{.Description}}
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
// tag::scalar-changelog-{{.Name}}[]
{{- if .Changelog }}
{{ .Changelog }}
{{- end }}
// end::scalar-changelog-{{.Name}}[]

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
[[subscription]]
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

// tag::mutation-changelog-{{.Name}}[]
{{- if .Changelog }}
{{ .Changelog }}
{{- end }}
// end::mutation-changelog-{{.Name}}[]

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

// tag::type-changelog-{{.Name}}[]
{{- if .Changelog }}
{{ .Changelog }}
{{- end }}
// end::type-changelog-{{.Name}}[]

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
// tag::enum-changelog-{{.Name}}[]
{{- if .Changelog }}
{{ .Changelog }}
{{- end }}
// end::enum-changelog-{{.Name}}[]

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

// tag::input-changelog-{{.Name}}[]
{{- if .Changelog }}
{{ .Changelog }}
{{- end }}
// end::input-changelog-{{.Name}}[]

// tag::input-def-{{.Name}}[]
{{ .FieldsTable }}
// end::input-def-{{.Name}}[]

// end::input-{{.Name}}[]

{{end}}
`

// CatalogueHeader is the document header and introduction for standalone
// catalogue mode.
const CatalogueHeader = `{{- if .SubTitle -}}
= GraphQL API Catalogue: {{.SubTitle}}
{{- else -}}
= GraphQL API Catalogue
{{- end }}
:toc: left
:revdate: {{.RevDate}}
:commandline: {{.CommandLine}}
{{- if .KrokiServerURL}}
:kroki-server-url: {{.KrokiServerURL}}
:kroki-fetch-diagram:
{{- end}}
:reproducible:
:page-partial:
:sect-anchors:
:table-caption!:
:table-stripes: even
:pdf-page-size: A4
:tags: api, GraphQL, nodes, types, query

include::_attributes.adoc[]

GraphQL is a modern API query language and runtime that provides a more flexible and efficient way for clients (like web or mobile apps)
to request data from servers compared to traditional REST APIs.

Instead of having multiple endpoints returning fixed data (like in REST).
GraphQL exposes a *single endpoint* where clients can *ask for exactly the data they need and nothing more.*

`

// Catalogue table sections shared by standalone catalogue mode and the
// summary at the top of a full document. Each declares an explicit anchor so
// its id does not depend on the idprefix/idseparator attributes of the
// rendering toolchain.
const CatalogueQueriesSection = `[[queries]]
== Queries

*Queries* are how clients *read or fetch data* in GraphQL.
They describe _what_ data the client wants, not _how_ to get it.

The following table provides a quick reference to all available queries in the GraphQL API.

{{- if .Queries }}

[options="header",cols="2m,5a"]
|===
| Name | Description
{{- range .Queries }}
| {{.Signature}} | {{range .Status}}*{{.}}* {{end}}{{.Description}}{{if .Changelog}}
{{.Changelog}}{{end}}
{{- end }}
|===
{{- else }}

[NOTE]
====
No queries exist in this schema.
====
{{- end }}

`

const CatalogueMutationsSection = `
[[mutations]]
== Mutations


*Mutations* are how clients *write or modify data* for example, creating, updating, or deleting records.

A mutation looks similar to a query, but it describes an action that changes data.

The following table provides a quick reference to all available mutations in the GraphQL API.

{{- if .MutationGroups }}

[options="header",cols="2m,5a"]
|===
| Name | Description
{{- range .MutationGroups }}
2+^h| {{.GroupName}}
{{- range .Mutations }}
| {{.Signature}} | {{range .Status}}*{{.}}* {{end}}{{.Description}}{{if .Changelog}}
{{.Changelog}}{{end}}
{{- end }}
{{- end }}
|===
{{- else }}

[NOTE]
====
No mutations exist in this schema.
====
{{- end }}

`

const CatalogueSubscriptionsSection = `[[subscriptions]]
== Subscriptions

{{- if .Subscriptions }}

*Subscriptions* are used to *receive real-time updates* from the server.
They let the client “subscribe” to data changes and get notified instantly when something new happens, without needing to poll for updates.

The following table provides a quick reference to all available subscriptions in the GraphQL API.

[options="header",cols="2m,5a"]
|===
| Name | Description
{{- range .Subscriptions }}
| {{.Signature}} | {{range .Status}}*{{.}}* {{end}}{{.Description}}{{if .Changelog}}
{{.Changelog}}{{end}}
{{- end }}
|===
{{- else }}

[NOTE]
====
No subscriptions exist in this schema.
====
{{- end }}
`

// CatalogueTemplate is the complete standalone catalogue document.
const CatalogueTemplate = CatalogueHeader + CatalogueQueriesSection + CatalogueMutationsSection + CatalogueSubscriptionsSection

package generator

// Data structures for template rendering

// FieldData represents field information for template rendering
type FieldData struct {
	Type            string
	Name            string
	Description     string
	RequiredOrArray bool
	Required        string
	IsArray         bool
	Directives      string
	Changelog       string
}

// TypeInfo represents type information for template rendering
type TypeInfo struct {
	Name        string
	Kind        string
	AnchorName  string
	Description string
	FieldsTable string // Pre-rendered AsciiDoc table for fields
	IsInterface bool
	Changelog   string
}

// EnumInfo represents enum information for template rendering
type EnumInfo struct {
	Name        string
	AnchorName  string
	Description string
	ValuesTable string
}

// InputInfo represents input type information for template rendering
type InputInfo struct {
	Name        string
	AnchorName  string
	Description string
	FieldsTable string
	Changelog   string
}

// MutationInfo represents mutation information for template rendering
type MutationInfo struct {
	Name                 string
	AnchorName           string
	Description          string
	CleanedDescription   string
	TypeName             string
	MethodSignatureBlock string
	Arguments            string
	Directives           string
	HasArguments         bool
	HasDirectives        bool
	IsInternal           bool
	Changelog            string
	NumberedRefs         string
}

// ScalarData represents scalar information for template rendering
type ScalarData struct {
	ScalarTag    string
	FoundScalars bool
	Scalars      []ScalarInfo
}

// ScalarInfo represents individual scalar information
type ScalarInfo struct {
	Name        string
	Description string
}

// SubscriptionData represents subscription information for template rendering
type SubscriptionData struct {
	FoundSubscriptions bool
	Subscriptions      []SubscriptionInfo
}

// SubscriptionInfo represents individual subscription information
type SubscriptionInfo struct {
	Description string
	Details     string
}

// DirectiveData represents directive information for template rendering
type DirectiveData struct {
	DirectivesTag   string
	FoundDirectives bool
	TableOptions    string
	Directives      []DirectiveInfo
}

// DirectiveInfo represents individual directive information
type DirectiveInfo struct {
	Name        string
	Arguments   string
	Description string
}

// CatalogueEntry represents a single entry in the catalogue table
type CatalogueEntry struct {
	Name        string
	Description string
	Changelog   string
}

// MutationGroup represents a group of mutations with a common prefix
type MutationGroup struct {
	GroupName string
	Mutations []CatalogueEntry
}

// CatalogueData represents the data for catalogue template rendering
type CatalogueData struct {
	SubTitle       string
	RevDate        string
	CommandLine    string
	Queries        []CatalogueEntry
	Mutations      []CatalogueEntry // Keep for backward compatibility
	MutationGroups []MutationGroup  // Grouped mutations
	Subscriptions  []CatalogueEntry
}

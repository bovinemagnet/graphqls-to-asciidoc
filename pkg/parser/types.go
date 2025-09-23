package parser

// DescriptionStructure represents a parsed structured description with sections
type DescriptionStructure struct {
	RawDescription string                 // Original unparsed description
	Overview       string                 // Main description text
	Parameters     []ParameterDoc         // Parsed @param annotations
	Returns        string                 // @returns documentation
	Errors         []ErrorDoc             // @throws/@errors documentation
	Examples       []Example              // Code examples
	Changelog      []ChangelogEntry       // Version history
	Metadata       map[string]string      // Additional metadata (since, deprecated, etc.)
	Sections       map[string]string      // Custom sections found in description
	IsStructured   bool                   // Whether description uses structured format
}

// ParameterDoc represents documentation for a single parameter
type ParameterDoc struct {
	Name        string         // Parameter name
	Type        string         // Parameter type
	Description string         // Parameter description
	Required    bool           // Whether parameter is required
	Default     string         // Default value if any
	Validation  string         // Validation rules
	Examples    []string       // Example values
	SubParams   []ParameterDoc // Nested parameters for complex types
}

// ErrorDoc represents documentation for an error/exception
type ErrorDoc struct {
	Code        string // Error code or type
	Description string // Error description
	When        string // Conditions when error occurs
}

// Example represents a code example
type Example struct {
	Title       string // Example title/label
	Description string // Example description
	Code        string // Code snippet
	Language    string // Code language for syntax highlighting
}

// ChangelogEntry represents a version change entry
type ChangelogEntry struct {
	Version     string // Version number
	Type        string // Change type (add, update, deprecate, remove)
	Description string // Change description
}

// DescriptionMetrics tracks metrics about description quality
type DescriptionMetrics struct {
	WordCount      int     // Total word count
	Sections       int     // Number of sections
	Examples       int     // Number of examples
	Parameters     int     // Number of documented parameters
	Completeness   float64 // Completeness score (0-1)
	Complexity     string  // Complexity level (simple/moderate/complex)
	HasOverview    bool    // Has overview section
	HasChangelog   bool    // Has changelog entries
	HasExamples    bool    // Has code examples
	HasParameters  bool    // Has parameter documentation
	HasReturns     bool    // Has return documentation
	HasErrors      bool    // Has error documentation
}

// SectionType represents known section types in structured documentation
type SectionType string

const (
	SectionOverview     SectionType = "Overview"
	SectionParameters   SectionType = "Parameters"
	SectionReturns      SectionType = "Returns"
	SectionErrors       SectionType = "Errors"
	SectionExamples     SectionType = "Examples"
	SectionPerformance  SectionType = "Performance"
	SectionSecurity     SectionType = "Security"
	SectionSeeAlso      SectionType = "See Also"
	SectionNotes        SectionType = "Notes"
	SectionDeprecation  SectionType = "Deprecation"
)

// ValidationRule represents a validation constraint
type ValidationRule struct {
	Type  string      // Rule type (min, max, pattern, etc.)
	Value interface{} // Rule value
}

// ParsedDescription wraps both structured and unstructured descriptions
type ParsedDescription struct {
	Structured   *DescriptionStructure // Structured description if available
	Unstructured string                // Fallback unstructured text
	Metrics      *DescriptionMetrics   // Description quality metrics
}
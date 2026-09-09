package describe

// Description represents a structured document that can be rendered
// in multiple formats (table, json, yaml, markdown).
type Description struct {
	Title    string    `json:"title" yaml:"title"`
	Sections []Section `json:"sections" yaml:"sections"`
}

// Section is a titled group of fields, tables, text blocks, and/or nested sub-sections.
type Section struct {
	Title       string    `json:"title,omitempty" yaml:"title,omitempty"`
	Fields      []Field   `json:"fields,omitempty" yaml:"fields,omitempty"`
	Tables      []Table   `json:"tables,omitempty" yaml:"tables,omitempty"`
	Texts       []Text    `json:"texts,omitempty" yaml:"texts,omitempty"`
	SubSections []Section `json:"subSections,omitempty" yaml:"subSections,omitempty"`
}

// Field is a labelled value. ColorFunc is only used by the table renderer.
type Field struct {
	Label     string              `json:"label" yaml:"label"`
	Value     string              `json:"value" yaml:"value"`
	ColorFunc func(string) string `json:"-" yaml:"-"`
}

// Table is a set of rows rendered as a columnar table.
// Row values are keyed by TableColumn.Key; column order follows Headers.
type Table struct {
	Headers []TableColumn       `json:"headers" yaml:"headers"`
	Rows    []map[string]string `json:"rows" yaml:"rows"`
}

// TableColumn describes one column of a Table.
type TableColumn struct {
	Display   string              `json:"display" yaml:"display"`
	Key       string              `json:"key" yaml:"key"`
	ColorFunc func(string) string `json:"-" yaml:"-"`
}

// Text is a free-form text block (e.g. an Ansible playbook).
// Language is used as the fence tag in markdown output.
type Text struct {
	Label    string `json:"label,omitempty" yaml:"label,omitempty"`
	Content  string `json:"content" yaml:"content"`
	Language string `json:"language,omitempty" yaml:"language,omitempty"`
}

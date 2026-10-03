package describe

// Builder constructs a Description using a fluent API.
//
// Fields, tables, and text blocks are appended to the current section.
// If no Section() call has been made, an implicit untitled section is used.
//
// SubSection/EndSubSection push and pop a nesting stack so that
// arbitrarily deep structures can be built without losing the parent context.
type Builder struct {
	desc  Description
	stack []*Section // nesting stack; last element is the active section
}

// NewBuilder starts a new Description with the given title.
func NewBuilder(title string) *Builder {
	return &Builder{
		desc: Description{Title: title},
	}
}

// Section appends a new top-level section and makes it active.
// Any pending sub-section nesting is reset.
func (b *Builder) Section(title string) *Builder {
	b.stack = nil
	b.desc.Sections = append(b.desc.Sections, Section{Title: title})
	b.stack = append(b.stack, &b.desc.Sections[len(b.desc.Sections)-1])
	return b
}

// SubSection appends a nested section inside the current section and pushes it onto the stack.
func (b *Builder) SubSection(title string) *Builder {
	parent := b.current()
	parent.SubSections = append(parent.SubSections, Section{Title: title})
	b.stack = append(b.stack, &parent.SubSections[len(parent.SubSections)-1])
	return b
}

// EndSubSection pops the nesting stack, returning to the parent section.
func (b *Builder) EndSubSection() *Builder {
	if len(b.stack) > 1 {
		b.stack = b.stack[:len(b.stack)-1]
	}
	return b
}

// Field appends a plain field to the current section.
func (b *Builder) Field(label, value string) *Builder {
	s := b.current()
	s.Fields = append(s.Fields, Field{Label: label, Value: value})
	return b
}

// FieldC appends a field with a color hint (used only in table output).
func (b *Builder) FieldC(label, value string, colorFunc func(string) string) *Builder {
	s := b.current()
	s.Fields = append(s.Fields, Field{Label: label, Value: value, ColorFunc: colorFunc})
	return b
}

// Table appends a table to the current section.
func (b *Builder) Table(headers []TableColumn, rows []map[string]string) *Builder {
	s := b.current()
	s.Tables = append(s.Tables, Table{Headers: headers, Rows: rows})
	return b
}

// Text appends a text block to the current section.
// language sets the fence tag in markdown output (e.g. "yaml").
func (b *Builder) Text(label, content, language string) *Builder {
	s := b.current()
	s.Texts = append(s.Texts, Text{Label: label, Content: content, Language: language})
	return b
}

// Build returns the finished Description.
func (b *Builder) Build() *Description {
	return &b.desc
}

// current returns the active section, creating an implicit untitled one if needed.
func (b *Builder) current() *Section {
	if len(b.stack) == 0 {
		b.desc.Sections = append(b.desc.Sections, Section{})
		b.stack = append(b.stack, &b.desc.Sections[len(b.desc.Sections)-1])
	}
	return b.stack[len(b.stack)-1]
}

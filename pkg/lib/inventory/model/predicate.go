package model

import (
	"bytes"
	"reflect"
	"strings"
	"text/template"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// Label SQL.
var LabelSQL = `
{{ $kind := .Kind -}}
{{ if .Len }}
{{ .Pk.Name }} IN
(
{{ range $i,$l := .List -}}
{{ if $i }}
INTERSECT
{{ end -}}
SELECT parent
FROM Label
WHERE kind = '{{ $kind }}' AND
name = {{ $l.Name }} AND
value = {{ $l.Value }}
{{ end -}}
)
{{ end -}}
`

// New Eq (=) predicate.
func Eq(field string, value interface{}) *EqPredicate {
	return &EqPredicate{
		SimplePredicate{
			Field: field,
			Value: value,
		},
	}
}

// New Neq (!=) predicate.
func Neq(field string, value interface{}) *NeqPredicate {
	return &NeqPredicate{
		SimplePredicate{
			Field: field,
			Value: value,
		},
	}
}

// New Gt (>) predicate.
func Gt(field string, value interface{}) *GtPredicate {
	return &GtPredicate{
		SimplePredicate{
			Field: field,
			Value: value,
		},
	}
}

// New Lt (<) predicate.
func Lt(field string, value interface{}) *LtPredicate {
	return &LtPredicate{
		SimplePredicate{
			Field: field,
			Value: value,
		},
	}
}

// AND predicate.
func And(predicates ...Predicate) *AndPredicate {
	return &AndPredicate{
		CompoundPredicate{
			Predicates: predicates,
		},
	}
}

// OR predicate.
func Or(predicates ...Predicate) *OrPredicate {
	return &OrPredicate{
		CompoundPredicate{
			Predicates: predicates,
		},
	}
}

// Label predicate.
func Match(labels Labels) *LabelPredicate {
	return &LabelPredicate{
		Labels: labels,
	}
}

// List predicate.
type Predicate interface {
	// Build the predicate.
	Build(*FilterOptions) error
	// Get the SQL expression.
	Expr() string
}

// Simple predicate.
type SimplePredicate struct {
	// Field name.
	Field string
	// Field value.
	Value interface{}
	// SQL expression.
	expr string
}

// Find referenced field.
func (p *SimplePredicate) match(fields []*Field) (*Field, bool) {
	return p.field(p.Field, fields)
}

// Find field.
func (p *SimplePredicate) field(name string, fields []*Field) (*Field, bool) {
	name = strings.ToLower(name)
	for _, f := range fields {
		if name == strings.ToLower(f.Name) {
			return f, true
		}
	}

	return nil, false
}

// Build.
func (p *SimplePredicate) build(operator string, options *FilterOptions) error {
	f, found := p.match(options.fields)
	if !found {
		return liberr.Wrap(PredicateRefErr)
	}
	switch p.Value.(type) {
	case Field:
		value := p.Value.(Field)
		fv, found := p.field(value.Name, options.fields)
		if !found {
			return liberr.Wrap(PredicateRefErr)
		}
		p.expr = strings.Join(
			[]string{
				f.Name,
				operator,
				fv.Name,
			}, " ")
	default:
		v, err := f.AsValue(p.Value)
		if err != nil {
			return err
		}
		p.expr = strings.Join(
			[]string{
				f.Name,
				operator,
				options.Param(f.Name, v)},
			" ")
	}

	return nil
}

// Equals (=) predicate.
type EqPredicate struct {
	SimplePredicate
}

// Build.
func (p *EqPredicate) Build(options *FilterOptions) error {
	f, found := p.match(options.fields)
	if !found {
		return liberr.Wrap(PredicateRefErr)
	}
	pv := reflect.ValueOf(p.Value)
	switch pv.Kind() {
	case reflect.Slice:
		params := []string{}
		for i := 0; i < pv.Len(); i++ {
			v, err := f.AsValue(pv.Index(i).Interface())
			if err != nil {
				return err
			}
			params = append(
				params,
				options.Param(f.Name, v))
		}
		p.expr = strings.Join(
			[]string{
				f.Name,
				"IN",
				"(",
				strings.Join(params, ","),
				")"},
			" ")
	default:
		return p.build("=", options)
	}

	return nil
}

// Render the expression.
func (p *EqPredicate) Expr() string {
	return p.expr
}

// NotEqual (!=) predicate.
type NeqPredicate struct {
	SimplePredicate
}

// Build.
func (p *NeqPredicate) Build(options *FilterOptions) error {
	return p.build("!=", options)
}

// Render the expression.
func (p *NeqPredicate) Expr() string {
	return p.expr
}

// Greater than (>) predicate.
type GtPredicate struct {
	SimplePredicate
}

// Build.
func (p *GtPredicate) Build(options *FilterOptions) error {
	f, found := p.match(options.fields)
	if !found {
		return liberr.Wrap(PredicateRefErr)
	}
	switch f.Value.Kind() {
	case reflect.String,
		reflect.Bool:
		return PredicateTypeErr
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return p.build(">", options)
	default:
		return FieldTypeErr
	}
}

// Render the expression.
func (p *GtPredicate) Expr() string {
	return p.expr
}

// Less than (<) predicate.
type LtPredicate struct {
	SimplePredicate
}

// Build.
func (p *LtPredicate) Build(options *FilterOptions) error {
	f, found := p.match(options.fields)
	if !found {
		return liberr.Wrap(PredicateRefErr)
	}
	switch f.Value.Kind() {
	case reflect.String,
		reflect.Bool:
		return PredicateTypeErr
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return p.build("<", options)
	default:
		return FieldTypeErr
	}
}

// Render the expression.
func (p *LtPredicate) Expr() string {
	return p.expr
}

// Compound predicate.
type CompoundPredicate struct {
	// List of predicates.
	Predicates []Predicate
}

// And predicate.
type AndPredicate struct {
	CompoundPredicate
}

// Build.
func (p *AndPredicate) Build(options *FilterOptions) error {
	for _, p := range p.Predicates {
		err := p.Build(options)
		if err != nil {
			return err
		}
	}

	return nil
}

// Render the expression.
func (p *AndPredicate) Expr() string {
	predicates := []string{}
	for _, p := range p.Predicates {
		predicates = append(predicates, p.Expr())
	}

	expr := strings.Join(predicates, " AND ")

	return expr
}

// OR predicate.
type OrPredicate struct {
	CompoundPredicate
}

// Build.
func (p *OrPredicate) Build(options *FilterOptions) error {
	for _, p := range p.Predicates {
		err := p.Build(options)
		if err != nil {
			return err
		}
	}

	return nil
}

// Render the expression.
func (p *OrPredicate) Expr() string {
	predicates := []string{}
	for _, p := range p.Predicates {
		predicates = append(predicates, p.Expr())
	}

	expr := strings.Join(predicates, " OR ")

	return expr
}

// Label predicate.
type LabelPredicate struct {
	// Labels
	Labels
	// List options.
	options *FilterOptions
	// Parent PK field name.
	pk *Field
	// SQL expression.
	expr string
}

// Build.
func (p *LabelPredicate) Build(options *FilterOptions) error {
	p.options = options
	for _, f := range options.fields {
		if f.Pk() {
			p.pk = f
			break
		}
	}
	tpl := template.New("")
	tpl, err := tpl.Parse(LabelSQL)
	if err != nil {
		return liberr.Wrap(err)
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(bfr, p)
	if err != nil {
		return liberr.Wrap(err)
	}

	p.expr = bfr.String()

	return nil
}

// Label (parent) kind.
func (p *LabelPredicate) Kind() string {
	return p.options.table
}

// PK field name.
func (p *LabelPredicate) Pk() *Field {
	return p.pk
}

// List of labels.
func (p *LabelPredicate) List() []Label {
	list := []Label{}
	for k, v := range p.Labels {
		k = p.options.Param("k", k)
		v = p.options.Param("v", v)
		list = append(
			list,
			Label{
				Name:  k,
				Value: v,
			})
	}

	return list
}

// Get the number of labels.
func (p *LabelPredicate) Len() int {
	return len(p.Labels)
}

// Render the expression.
func (p *LabelPredicate) Expr() string {
	return p.expr
}

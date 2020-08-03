package model

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/mattn/go-sqlite3"
	"reflect"
	"regexp"
	"strings"
	"text/template"
)

const (
	Tag = "sql"
)

//
// DDL templates.
var TableDDL = `
CREATE TABLE IF NOT EXISTS {{.Table}} (
{{ range $i,$f := .Fields -}}
{{ if $i }},{{ end -}}
{{ $f.DDL }}
{{ end -}}
{{ range $i,$c := .Constraints -}}
,{{ $c }}
{{ end -}}
);
`

var IndexDDL = `
CREATE INDEX IF NOT EXISTS {{.Table}}Index
ON {{.Table}}
(
{{ range $i,$f := .Fields -}}
{{ if $i }},{{ end -}}
{{ $f.Name }}
{{ end -}}
);
`

//
// SQL templates.
var InsertSQL = `
INSERT INTO {{.Table}} (
{{ range $i,$f := .Fields -}}
{{ if $i}},{{ end -}}
{{ $f.Name }}
{{ end -}}
)
VALUES (
{{ range $i,$f := .Fields -}}
{{ if $i }},{{ end -}}
{{ $f.Param }}
{{ end -}}
);
`

var UpdateSQL = `
UPDATE {{.Table}}
SET
{{ range $i,$f := .Fields -}}
{{ if $i }},{{ end -}}
{{ $f.Name }} = 
{{- if $f.Revision -}}
{{ $f.Name }}+1
{{ else -}}
{{ $f.Param }}
{{ end -}}
{{ end -}}
WHERE
{{ if .Pk -}}
{{ .Pk.Name }} = {{ .Pk.Param }}
{{ if .Revision -}}
AND {{ .Revision.Name }} = {{ .Revision.Param }}
{{ end -}}
{{ else -}}
{{ range $i,$f := .Keys -}}
{{ if $i }}AND {{ end -}}
{{ $f.Name }} = {{ $f.Param }}
{{ end -}}
{{ end -}}
;
`

var DeleteSQL = `
DELETE FROM {{.Table}}
WHERE
{{ if .Pk -}}
{{ .Pk.Name }} = {{ .Pk.Param }}
{{ else -}}
{{ range $i,$f := .Keys -}}
{{ if $i }}AND {{ end -}}
{{ $f.Name }} = {{ $f.Param }}
{{ end -}}
{{ end -}}
;
`

var GetSQL = `
SELECT
{{ range $i,$f := .Fields -}}
{{ if $i }},{{ end -}}
{{ $f.Name }}
{{ end -}}
FROM {{.Table}}
WHERE
{{ if .Pk -}}
{{ .Pk.Name }} = {{ .Pk.Param }}
{{ else -}}
{{ range $i,$f := .Keys -}}
{{ if $i }}AND {{ end -}}
{{ $f.Name }} = {{ $f.Param }}
{{ end -}}
{{ end -}}
;
`

var ListSQL = `
SELECT
{{ if .Count -}}
COUNT(*)
{{ else -}}
{{ range $i,$f := .Fields -}}
{{ if $i }},{{ end -}}
{{ $f.Name }}
{{ end -}}
{{ end -}}
FROM {{.Table}}
{{ if or .NotEmpty .Labels -}}
WHERE
{{ end -}}
{{ $fCount := len .NotEmpty -}}
{{ range $i,$f := .NotEmpty -}}
{{ if $i }}AND {{ end -}}
{{ $f.Name }} = {{ $f.Param }}
{{ end -}}
{{ if .Labels -}}
{{ if $fCount }}AND {{ end -}}
{{ .Pk.Name }} IN
(
{{ $kind := .Table -}}
{{ range $i,$l := .Labels -}}
{{ if $i }}
INTERSECT
{{ end -}}
SELECT parent
FROM Label
WHERE kind = '{{ $kind }}' AND
name = '{{$l.Name}}' AND
value = '{{$l.Value}}'
{{ end -}}
)
{{ end -}}
{{ if .Options.Sort -}}
ORDER BY
{{ range $i,$n := .Options.Sort -}}
{{ if $i }},{{ end }}{{ $n }}
{{ end -}}
{{ end -}}
{{ if .Options.Page -}}
LIMIT {{.Options.Page.Limit}} OFFSET {{.Options.Page.Offset}}
{{ end -}}
;
`

//
// Represents a table in the DB.
// Using reflect, the model is inspected to determine the
// table name and columns. The column definition is specified
// using field tags:
//   pk - Primary key.
//   key - Natural key.
//   fk:<table>(field) - Foreign key.
//   unique(<group>) - Unique constraint collated by <group>.
//   const - Not updated.
type Table struct {
	// Database connection.
	DB DBTX
}

//
// Get the table name for the model.
func (t Table) Name(model interface{}) string {
	mt := reflect.TypeOf(model)
	if mt.Kind() == reflect.Ptr {
		mt = mt.Elem()
	}

	return mt.Name()
}

//
// Get table and index create DDL.
func (t Table) DDL(model interface{}) ([]string, error) {
	list := []string{}
	tpl := template.New("")
	fields, err := t.Fields(model)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	for _, f := range fields {
		err := f.Validate()
		if err != nil {
			return nil, liberr.Wrap(err)
		}
	}
	// Table
	tpl, err = tpl.Parse(TableDDL)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	constraints := t.Constraints(fields)
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table:       t.Name(model),
			Constraints: constraints,
			Fields:      fields,
		})
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	list = append(list, bfr.String())
	// Index.
	fields = t.KeyFields(fields)
	if len(fields) > 0 {
		tpl, err = tpl.Parse(IndexDDL)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		bfr = &bytes.Buffer{}
		err = tpl.Execute(
			bfr,
			TmplData{
				Table:  t.Name(model),
				Fields: fields,
			})
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		list = append(list, bfr.String())
	}

	return list, nil
}

//
// Insert the model in the DB.
// Expects the primary key (PK) to be set.
func (t Table) Insert(model interface{}) error {
	fields, err := t.Fields(model)
	if err != nil {
		return liberr.Wrap(err)
	}
	stmt, err := t.insertSQL(t.Name(model), fields)
	if err != nil {
		return liberr.Wrap(err)
	}
	params := t.Params(fields)
	r, err := t.DB.Exec(stmt, params...)
	if err != nil {
		if sql3Err, cast := err.(sqlite3.Error); cast {
			if sql3Err.Code == sqlite3.ErrConstraint {
				return t.Update(model)
			}
		}
		return liberr.Wrap(err)
	}
	_, err = r.RowsAffected()
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

//
// Update the model in the DB.
// Expects the primary key (PK) or natural keys to be set.
func (t Table) Update(model interface{}) error {
	fields, err := t.Fields(model)
	if err != nil {
		return liberr.Wrap(err)
	}
	stmt, err := t.updateSQL(t.Name(model), fields)
	if err != nil {
		return liberr.Wrap(err)
	}
	params := t.Params(fields)
	r, err := t.DB.Exec(stmt, params...)
	if err != nil {
		return liberr.Wrap(err)
	}
	nRows, err := r.RowsAffected()
	if err != nil {
		return liberr.Wrap(err)
	}
	if nRows == 0 {
		if t.Get(model) == nil {
			return liberr.Wrap(Conflict)
		} else {
			return liberr.Wrap(NotFound)
		}
	}

	return nil
}

//
// Delete the model in the DB.
// Expects the primary key (PK) or natural keys to be set.
func (t Table) Delete(model interface{}) error {
	fields, err := t.Fields(model)
	if err != nil {
		return liberr.Wrap(err)
	}
	stmt, err := t.deleteSQL(t.Name(model), fields)
	if err != nil {
		return liberr.Wrap(err)
	}
	params := t.Params(fields)
	r, err := t.DB.Exec(stmt, params...)
	if err != nil {
		return liberr.Wrap(err)
	}
	nRows, err := r.RowsAffected()
	if err != nil {
		return liberr.Wrap(err)
	}
	if nRows == 0 {
		return nil
	}

	return nil
}

//
// Get the model in the DB.
// Expects the primary key (PK) or natural keys to be set.
// Fetch the row and populate the fields in the model.
func (t Table) Get(model interface{}) error {
	fields, err := t.Fields(model)
	if err != nil {
		return liberr.Wrap(err)
	}
	stmt, err := t.getSQL(t.Name(model), fields)
	if err != nil {
		return liberr.Wrap(err)
	}
	params := t.Params(fields)
	row := t.DB.QueryRow(stmt, params...)
	err = t.scan(row, fields)

	return liberr.Wrap(err)
}

//
// List the model in the DB.
// Qualified by the model field values and list options.
// Expects natural keys to be set.
// Else, ALL models fetched.
func (t Table) List(model interface{}, options ListOptions) ([]interface{}, error) {
	fields, err := t.Fields(model)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	stmt, err := t.listSQL(t.Name(model), fields, options)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	params := t.Params(fields)
	cursor, err := t.DB.Query(stmt, params...)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	defer cursor.Close()
	list := []interface{}{}
	for cursor.Next() {
		mt := reflect.TypeOf(model)
		mPtr := reflect.New(mt.Elem())
		mInt := mPtr.Interface()
		newFields, _ := t.Fields(mInt)
		err = t.scan(cursor, newFields)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		list = append(list, mInt)
	}

	return list, nil
}

//
// Count the models in the DB.
// Qualified by the model field values and list options.
// Expects natural keys to be set.
// Else, ALL models counted.
func (t Table) Count(model interface{}, options ListOptions) (int64, error) {
	fields, err := t.Fields(model)
	if err != nil {
		return 0, liberr.Wrap(err)
	}
	options.Count = true
	stmt, err := t.listSQL(t.Name(model), fields, options)
	if err != nil {
		return 0, liberr.Wrap(err)
	}
	count := int64(0)
	params := t.Params(fields)
	row := t.DB.QueryRow(stmt, params...)
	if err != nil {
		return 0, liberr.Wrap(err)
	}
	err = row.Scan(&count)
	if err != nil {
		return 0, liberr.Wrap(err)
	}

	return count, nil
}

//
// Get the `Fields` for the model.
func (t Table) Fields(model interface{}) ([]*Field, error) {
	fields := []*Field{}
	mt := reflect.TypeOf(model)
	mv := reflect.ValueOf(model)
	if mt.Kind() == reflect.Ptr {
		mt = mt.Elem()
		mv = mv.Elem()
	} else {
		return nil, errors.New("must be pointer")
	}
	if mv.Kind() != reflect.Struct {
		return nil, errors.New("must be object")
	}
	for i := 0; i < mt.NumField(); i++ {
		ft := mt.Field(i)
		fv := mv.Field(i)
		switch fv.Kind() {
		case reflect.Struct:
			sfields, err := t.Fields(fv.Addr().Interface())
			if err != nil {
				return nil, nil
			}
			fields = append(fields, sfields...)
		case reflect.String:
			fields = append(
				fields,
				&Field{
					Tag:   ft.Tag.Get(Tag),
					Name:  ft.Name,
					Value: &fv,
				})
		case reflect.Int,
			reflect.Int8,
			reflect.Int16,
			reflect.Int32,
			reflect.Int64:
			fields = append(
				fields,
				&Field{
					Tag:   ft.Tag.Get(Tag),
					Name:  ft.Name,
					Value: &fv,
				})
		}
	}

	return fields, nil
}

//
// Get the populated `Fields` for the model.
func (t Table) NotEmptyFields(fields []*Field) []*Field {
	list := []*Field{}
	for _, f := range fields {
		if !f.Empty() {
			list = append(list, f)
		}
	}

	return list
}

//
// Get the `Fields` referenced as param in SQL.
func (t Table) Params(fields []*Field) []interface{} {
	list := []interface{}{}
	for _, f := range fields {
		if f.isParam {
			p := sql.Named(f.Name, f.Pull())
			list = append(list, p)
		}
	}

	return list
}

//
// Get the mutable `Fields` for the model.
func (t Table) MutableFields(fields []*Field) []*Field {
	list := []*Field{}
	for _, f := range fields {
		if f.Mutable() {
			list = append(list, f)
		}
	}

	return list
}

//
// Get the natural key `Fields` for the model.
func (t Table) KeyFields(fields []*Field) []*Field {
	list := []*Field{}
	for _, f := range fields {
		if f.Key() {
			list = append(list, f)
		}
	}

	return list
}

//
// Get the PK field.
func (t Table) PkField(fields []*Field) *Field {
	for _, f := range fields {
		if f.Pk() {
			return f
		}
	}

	return nil
}

//
// Get the Revision field.
func (t Table) RevisionField(fields []*Field) *Field {
	for _, f := range fields {
		if f.Revision() {
			return f
		}
	}

	return nil
}

//
// Get constraint DDL.
func (t Table) Constraints(fields []*Field) []string {
	constraints := []string{}
	unique := map[string][]string{}
	for _, field := range fields {
		for _, name := range field.Unique() {
			list, found := unique[name]
			if found {
				unique[name] = append(list, field.Name)
			} else {
				unique[name] = []string{field.Name}
			}
		}
	}
	for _, list := range unique {
		constraints = append(
			constraints,
			fmt.Sprintf(
				"UNIQUE (%s)",
				strings.Join(list, ",")))
	}
	for _, field := range fields {
		fk := field.Fk()
		if fk == nil {
			continue
		}
		constraints = append(constraints, fk.DDL(field))
	}

	return constraints
}

//
// Build model insert SQL.
func (t Table) insertSQL(table string, fields []*Field) (string, error) {
	tpl := template.New("")
	tpl, err := tpl.Parse(InsertSQL)
	if err != nil {
		return "", liberr.Wrap(err)
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table:  table,
			Fields: fields,
		})
	if err != nil {
		return "", liberr.Wrap(err)
	}

	return bfr.String(), nil
}

//
// Build model update SQL.
func (t Table) updateSQL(table string, fields []*Field) (string, error) {
	tpl := template.New("")
	tpl, err := tpl.Parse(UpdateSQL)
	if err != nil {
		return "", liberr.Wrap(err)
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table:    table,
			Fields:   t.MutableFields(fields),
			Revision: t.RevisionField(fields),
			Keys:     t.KeyFields(fields),
			Pk:       t.PkField(fields),
		})
	if err != nil {
		return "", liberr.Wrap(err)
	}

	return bfr.String(), nil
}

//
// Build model delete SQL.
func (t Table) deleteSQL(table string, fields []*Field) (string, error) {
	tpl := template.New("")
	tpl, err := tpl.Parse(DeleteSQL)
	if err != nil {
		return "", liberr.Wrap(err)
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table: table,
			Keys:  t.KeyFields(fields),
			Pk:    t.PkField(fields),
		})
	if err != nil {
		return "", liberr.Wrap(err)
	}

	return bfr.String(), nil
}

//
// Build model get SQL.
func (t Table) getSQL(table string, fields []*Field) (string, error) {
	tpl := template.New("")
	tpl, err := tpl.Parse(GetSQL)
	if err != nil {
		return "", liberr.Wrap(err)
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table:  table,
			Keys:   t.KeyFields(fields),
			Fields: fields,
		})
	if err != nil {
		return "", liberr.Wrap(err)
	}

	return bfr.String(), nil
}

//
// Build model list SQL.
func (t Table) listSQL(table string, fields []*Field, options ListOptions) (string, error) {
	tpl := template.New("")
	tpl, err := tpl.Parse(ListSQL)
	if err != nil {
		return "", liberr.Wrap(err)
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table:    table,
			Fields:   fields,
			NotEmpty: t.NotEmptyFields(fields),
			Options:  options,
			Pk:       t.PkField(fields),
			Count:    options.Count,
		})
	if err != nil {
		return "", liberr.Wrap(err)
	}

	return bfr.String(), nil
}

//
// Scan the fetch row into the model.
// The model fields are updated.
func (t Table) scan(row Row, fields []*Field) error {
	list := []interface{}{}
	for _, f := range fields {
		f.Pull()
		list = append(list, f.Ptr())
	}
	err := row.Scan(list...)
	if err == nil {
		for _, f := range fields {
			f.Push()
		}
	}

	return liberr.Wrap(err)
}

//
// Regex used for `unique(group)` tags.
var UniqueRegex = regexp.MustCompile(`(unique)(\()(.+)(\))`)

//
// Regex used for `fk:<table>(field)` tags.
var FkRegex = regexp.MustCompile(`(fk):(.+)(\()(.+)(\))`)

//
// Model (struct) Field
// Tags:
//   `sql:"pk"`
//       The primary key.
//   `sql:"key"`
//       The field is part of the natural key.
//   `sql:"fk:T(F)"`
//       Foreign key `T` = model type, `F` = model field.
//   `sql:"unique(G)"`
//       Unique index. `G` = unique-together fields.
//   `sql:"const"`
//       The field is immutable and not included on update.
//   `sql:"revision"`
//       The field is used in detect an update conflict.
//
type Field struct {
	// reflect.Value of the field.
	Value *reflect.Value
	// Tags.
	Tag string
	// Field name.
	Name string
	// Staging (string) values.
	string string
	// Staging (int) values.
	int int64
	// Referenced as a parameter.
	isParam bool
}

//
// Validate.
func (f *Field) Validate() error {
	switch f.Value.Kind() {
	case reflect.String:
		if f.Revision() {
			err := errors.New("revision must be integer")
			return liberr.Wrap(err)
		}
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
	default:
		err := errors.New("must be: (string, int)")
		return liberr.Wrap(err)
	}

	return nil
}

//
// Pull from model.
// Populate the appropriate `staging` field using the
// model field value.
func (f *Field) Pull() interface{} {
	switch f.Value.Kind() {
	case reflect.String:
		f.string = f.Value.String()
		return f.string
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		f.int = f.Value.Int()
		return f.int
	}

	return nil
}

//
// Pointer used for Scan().
func (f *Field) Ptr() interface{} {
	switch f.Value.Kind() {
	case reflect.String:
		return &f.string
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return &f.int
	}

	return nil
}

//
// Push to the model.
// Set the model field value using the `staging` field.
func (f *Field) Push() {
	switch f.Value.Kind() {
	case reflect.String:
		f.Value.SetString(f.string)
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		f.Value.SetInt(f.int)
	}
}

//
// Column DDL.
func (f *Field) DDL() string {
	part := []string{
		f.Name, // name
		"",     // type
		"",     // constraint
	}
	switch f.Value.Kind() {
	case reflect.String:
		part[1] = "TEXT"
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		part[1] = "INTEGER"
	}
	if f.Pk() {
		part[2] = "PRIMARY KEY"
	} else {
		part[2] = "NOT NULL"
	}

	return strings.Join(part, " ")
}

//
// Get as SQL param.
func (f *Field) Param() string {
	f.isParam = true
	return ":" + f.Name
}

//
// Get whether field is empty.
func (f *Field) Empty() bool {
	f.Pull()
	switch f.Value.Kind() {
	case reflect.String:
		return len(f.string) == 0
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return f.int == 0
	}

	return false
}

//
// Get whether field is the primary key.
func (f *Field) Pk() bool {
	return f.hasOpt("pk")
}

//
// Get whether field is mutable.
// Only mutable fields will be updated.
func (f *Field) Mutable() bool {
	if f.Pk() {
		return false
	}

	return !f.hasOpt("const")
}

//
// Get whether field is a natural key.
func (f *Field) Key() bool {
	return f.hasOpt("key")
}

//
// Get whether the field is unique.
func (f *Field) Unique() []string {
	list := []string{}
	for _, opt := range strings.Split(f.Tag, ",") {
		opt = strings.TrimSpace(opt)
		m := UniqueRegex.FindStringSubmatch(opt)
		if m != nil && len(m) == 5 {
			list = append(list, m[3])
		}
	}

	return list
}

//
// Get whether the field is a foreign key.
func (f *Field) Fk() *FK {
	for _, opt := range strings.Split(f.Tag, ",") {
		opt = strings.TrimSpace(opt)
		m := FkRegex.FindStringSubmatch(opt)
		if m != nil && len(m) == 6 {
			return &FK{
				Table: m[2],
				Field: m[4],
			}
		}
	}

	return nil
}

//
// Get whether field is used for revision management.
func (f *Field) Revision() bool {
	return f.hasOpt("revision")
}

//
// Get whether field has an option.
func (f *Field) hasOpt(name string) bool {
	for _, opt := range strings.Split(f.Tag, ",") {
		opt = strings.TrimSpace(opt)
		if opt == name {
			return true
		}
	}

	return false
}

//
// FK constraint.
type FK struct {
	// Table name.
	Table string
	// Field name.
	Field string
}

//
// Get DDL.
func (f *FK) DDL(field *Field) string {
	return fmt.Sprintf(
		"FOREIGN KEY (%s) REFERENCES %s (%s) ON DELETE CASCADE",
		field.Name,
		f.Table,
		f.Field)
}

//
// Template data.
type TmplData struct {
	// Table name.
	Table string
	// Fields.
	Fields []*Field
	// Constraint DDL.
	Constraints []string
	// Natural key fields.
	Keys []*Field
	// Set (not empty) fields.
	NotEmpty []*Field
	// Primary key.
	Pk *Field
	// Revision to update.
	Revision *Field
	// List options.
	Options ListOptions
	// Row count.
	Count bool
}

//
// Labels list.
func (t TmplData) Labels() []Label {
	list := []Label{}
	for k, v := range t.Options.Labels {
		list = append(list, Label{
			Name:  k,
			Value: v,
		})
	}

	return list
}

//
// List options.
type ListOptions struct {
	// Row count.
	Count bool
	// Labels.
	Labels Labels
	// Pagination.
	Page *Page
	// Sort by field position.
	Sort []int
}

package model

import (
	"bytes"
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	fb "github.com/konveyor/controller/pkg/filebacked"
	"github.com/mattn/go-sqlite3"
	"reflect"
	"strings"
	"text/template"
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
CREATE INDEX IF NOT EXISTS {{.Index}}Index
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
{{ $f.Name }} = {{ $f.Param }}
{{ end -}}
WHERE
{{ .Pk.Name }} = {{ .Pk.Param }}
{{ if .Predicate -}}
AND {{ .Predicate.Expr }}
{{ end -}}
;
`

var DeleteSQL = `
DELETE FROM {{.Table}}
WHERE
{{ .Pk.Name }} = {{ .Pk.Param }}
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
{{ .Pk.Name }} = {{ .Pk.Param }}
;
`

var ListSQL = `
SELECT
{{ if .Count -}}
COUNT(*)
{{ else -}}
{{ range $i,$f := .Options.Fields -}}
{{ if $i }},{{ end -}}
{{ $f.Name }}
{{ end -}}
{{ end -}}
FROM {{.Table}}
{{ if or .Predicate -}}
WHERE
{{ end -}}
{{ if .Predicate -}}
{{ .Predicate.Expr }}
{{ end -}}
{{ if .Sort -}}
ORDER BY
{{ range $i,$n := .Sort -}}
{{ if $i }},{{ end }}{{ $n }}
{{ end -}}
{{ end -}}
{{ if .Page -}}
LIMIT {{.Page.Limit}} OFFSET {{.Page.Offset}}
{{ end -}}
;
`

//
// Errors
var (
	// Must have PK.
	MustHavePkErr = errors.New("must have PK field")
	// Parameter must be pointer error.
	MustBePtrErr = errors.New("must be pointer")
	// Must be slice pointer.
	MustBeSlicePtrErr = errors.New("must be slice pointer")
	// Parameter must be struct error.
	MustBeObjectErr = errors.New("must be object")
	// Field type error.
	FieldTypeErr = errors.New("field type must be (int, str, bool")
	// PK field type error.
	PkTypeErr = errors.New("pk field must be (int, str)")
	// Generated PK error.
	GenPkTypeErr = errors.New("PK field must be `str` when generated")
	// Invalid field referenced in predicate.
	PredicateRefErr = errors.New("predicate referenced unknown field")
	// Invalid predicate for type of field.
	PredicateTypeErr = errors.New("predicate type not valid for field")
	// Invalid predicate value.
	PredicateValueErr = errors.New("predicate value not valid")
	// Invalid detail level.
	DetailErr = errors.New("detail level must be <= MaxDetail")
)

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
	return Definition{}.kind(model)
}

//
// Get table and index create DDL.
func (t Table) DDL(model interface{}, dm *DataModel) (list []string, err error) {
	md, found := dm.FindWith(model)
	if !found {
		return
	}
	list = []string{}
	ddl, err := t.TableDDL(md, dm)
	if err != nil {
		return
	}
	for _, stmt := range ddl {
		list = append(list, stmt)
	}
	ddl, err = t.KeyIndexDDL(md)
	if err != nil {
		return
	}
	for _, stmt := range ddl {
		list = append(list, stmt)
	}
	ddl, err = t.IndexDDL(md)
	if err != nil {
		return
	}
	for _, stmt := range ddl {
		list = append(list, stmt)
	}

	return
}

//
// Build table DDL.
func (t Table) TableDDL(md *Definition, dm *DataModel) (list []string, err error) {
	tpl := template.New("")
	tpl, err = tpl.Parse(TableDDL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	constraints, err := t.Constraints(md, dm)
	if err != nil {
		return
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table:       md.Kind,
			Fields:      md.RealFields(md.Fields),
			Constraints: constraints,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	list = append(list, bfr.String())
	return
}

//
// Build natural key index DDL.
func (t Table) KeyIndexDDL(md *Definition) (list []string, err error) {
	tpl := template.New("")
	keyFields := md.KeyFields()
	if len(keyFields) > 0 {
		tpl, err = tpl.Parse(IndexDDL)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		bfr := &bytes.Buffer{}
		err = tpl.Execute(
			bfr,
			TmplData{
				Table:  md.Kind,
				Index:  md.Kind,
				Fields: md.RealFields(keyFields),
			})
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		list = append(list, bfr.String())
	}

	return
}

//
// Build non-unique index DDL.
func (t Table) IndexDDL(md *Definition) (list []string, err error) {
	tpl := template.New("")
	index := map[string][]*Field{}
	for _, field := range md.Fields {
		for _, group := range field.Index() {
			list, found := index[group]
			if found {
				index[group] = append(list, field)
			} else {
				index[group] = []*Field{field}
			}
		}
	}
	for _, fk := range md.Fks() {
		if !fk.needsIndex() {
			continue
		}
		group := fk.Owner.Name + "__FK__"
		list, found := index[group]
		if found {
			index[group] = append(list, fk.Owner)
		} else {
			index[group] = []*Field{fk.Owner}
		}
	}
	for group, idxFields := range index {
		tpl, err = tpl.Parse(IndexDDL)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		bfr := &bytes.Buffer{}
		err = tpl.Execute(
			bfr,
			TmplData{
				Table:  md.Kind,
				Index:  md.Kind + group,
				Fields: md.RealFields(idxFields),
			})
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		list = append(list, bfr.String())
	}

	return
}

//
// Insert the model in the DB.
// Expects the primary key (PK) to be set.
func (t Table) Insert(model interface{}) (err error) {
	md, err := Inspect(model)
	if err != nil {
		return
	}
	t.EnsurePk(md)
	stmt, err := t.insertSQL(md)
	if err != nil {
		return
	}
	params := t.Params(md)
	r, err := t.DB.Exec(stmt, params...)
	if err != nil {
		if sql3Err, cast := err.(sqlite3.Error); cast {
			if sql3Err.Code == sqlite3.ErrConstraint {
				return t.Update(model)
			}
		}
		err = liberr.Wrap(
			err,
			"sql",
			stmt,
			"params",
			params)
		return
	}
	_, err = r.RowsAffected()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	t.reflectIncremented(md)

	log.V(5).Info(
		"table: model inserted.",
		"sql",
		stmt,
		"params",
		params)

	return
}

//
// Update the model in the DB.
// Expects the primary key (PK) to be set.
func (t Table) Update(model interface{}, predicate ...Predicate) (err error) {
	md, err := Inspect(model)
	if err != nil {
		return
	}
	t.EnsurePk(md)
	options := &ListOptions{}
	if len(predicate) > 0 {
		options.Predicate = And(predicate...)
	}
	stmt, err := t.updateSQL(md, options)
	if err != nil {
		return
	}
	params := append(t.Params(md), options.Params()...)
	r, err := t.DB.Exec(stmt, params...)
	if err != nil {
		err = liberr.Wrap(
			err,
			"sql",
			stmt,
			"params",
			params)
		return
	}
	nRows, err := r.RowsAffected()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if nRows == 0 {
		err = liberr.Wrap(NotFound)
		return
	}

	t.reflectIncremented(md)

	log.V(5).Info(
		"table: model updated.",
		"sql",
		stmt,
		"params",
		params)

	return
}

//
// Delete the model in the DB.
// Expects the primary key (PK) to be set.
func (t Table) Delete(model interface{}) (err error) {
	md, err := Inspect(model)
	if err != nil {
		return
	}
	t.EnsurePk(md)
	stmt, err := t.deleteSQL(md)
	if err != nil {
		return
	}
	params := t.Params(md)
	r, err := t.DB.Exec(stmt, params...)
	if err != nil {
		err = liberr.Wrap(
			err,
			"sql",
			stmt,
			"params",
			params)
		return
	}
	nRows, err := r.RowsAffected()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if nRows == 0 {
		err = liberr.Wrap(NotFound)
		return
	}

	log.V(5).Info(
		"table: model deleted.",
		"sql",
		stmt,
		"params",
		params)

	return
}

//
// Get the model in the DB.
// Expects the primary key (PK) to be set.
// Fetch the row and populate the fields in the model.
func (t Table) Get(model interface{}) (err error) {
	md, err := Inspect(model)
	if err != nil {
		return
	}
	t.EnsurePk(md)
	stmt, err := t.getSQL(md)
	if err != nil {
		return
	}
	params := t.Params(md)
	row := t.DB.QueryRow(stmt, params...)
	err = t.scan(row, md.Fields)
	if err != nil {
		err = liberr.Wrap(
			err,
			"sql",
			stmt,
			"params",
			params)
		return
	}

	log.V(5).Info(
		"table: get succeeded.",
		"sql",
		stmt,
		"params",
		params)

	return
}

//
// List the model in the DB.
// Qualified by the list options.
func (t Table) List(list interface{}, options ListOptions) (err error) {
	var model interface{}
	lt := reflect.TypeOf(list)
	lv := reflect.ValueOf(list)
	switch lt.Kind() {
	case reflect.Ptr:
		lt = lt.Elem()
		lv = lv.Elem()
	default:
		err = liberr.Wrap(MustBeSlicePtrErr)
		return
	}
	switch lt.Kind() {
	case reflect.Slice:
		model = reflect.New(lt.Elem()).Interface()
	default:
		err = liberr.Wrap(MustBeSlicePtrErr)
		return
	}
	md, err := Inspect(model)
	if err != nil {
		return
	}
	stmt, err := t.listSQL(md, &options)
	if err != nil {
		return
	}
	params := options.Params()
	cursor, err := t.DB.Query(stmt, params...)
	if err != nil {
		err = liberr.Wrap(
			err,
			"sql",
			stmt,
			"params",
			params)
		return
	}
	defer func() {
		_ = cursor.Close()
	}()
	mList := reflect.MakeSlice(lt, 0, 0)
	for cursor.Next() {
		mt := reflect.TypeOf(model)
		mPtr := reflect.New(mt.Elem())
		mInt := mPtr.Interface()
		mDef, _ := Inspect(mInt)
		options.fields = mDef.Fields
		err = t.scan(cursor, options.Fields())
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		mList = reflect.Append(mList, mPtr.Elem())
	}

	lv.Set(mList)

	log.V(5).Info(
		"table: list succeeded.",
		"sql",
		stmt,
		"params",
		params,
		"matched",
		lv.Len())

	return
}

//
// Find models in the DB.
// Qualified by the list options.
func (t Table) Find(model interface{}, options ListOptions) (itr fb.Iterator, err error) {
	md, err := Inspect(model)
	if err != nil {
		return
	}
	stmt, err := t.listSQL(md, &options)
	if err != nil {
		return
	}
	params := options.Params()
	cursor, err := t.DB.Query(stmt, params...)
	if err != nil {
		err = liberr.Wrap(err, "sql", stmt, "params", params)
		return
	}
	defer func() {
		_ = cursor.Close()
	}()
	list := fb.NewList()
	for cursor.Next() {
		mt := reflect.TypeOf(model)
		mPtr := reflect.New(mt.Elem())
		mInt := mPtr.Interface()
		mDef, _ := Inspect(mInt)
		options.fields = mDef.Fields
		err = t.scan(cursor, options.Fields())
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		list.Append(mPtr.Interface())
	}

	itr = list.Iter()

	log.V(5).Info(
		"table: find succeeded.",
		"sql",
		stmt,
		"params",
		params,
		"matched",
		itr.Len())

	return
}

//
// Count the models in the DB.
// Qualified by the model field values and list options.
// Else, ALL models are counted.
func (t Table) Count(model interface{}, predicate Predicate) (count int64, err error) {
	md, err := Inspect(model)
	if err != nil {
		return
	}
	options := ListOptions{Predicate: predicate}
	stmt, err := t.countSQL(md, &options)
	if err != nil {
		return
	}
	count = int64(0)
	params := options.Params()
	row := t.DB.QueryRow(stmt, params...)
	err = row.Scan(&count)
	if err != nil {
		err = liberr.Wrap(
			err,
			"sql",
			stmt,
			"params",
			params)
		return
	}

	log.V(5).Info(
		"table: count succeeded.",
		"sql",
		stmt,
		"params",
		params)

	return
}

//
// Get the `Fields` referenced as param in SQL.
func (t Table) Params(md *Definition) (list []interface{}) {
	list = []interface{}{}
	for _, f := range md.Fields {
		if f.isParam {
			p := sql.Named(f.Name, f.Pull())
			list = append(list, p)
		}
	}

	return
}

//
// Ensure PK is generated as specified/needed.
func (t Table) EnsurePk(md *Definition) {
	pk := md.PkField()
	if pk == nil {
		return
	}
	withFields := pk.WithFields()
	if len(withFields) == 0 {
		return
	}
	switch pk.Value.Kind() {
	case reflect.String:
		if pk.Pull() != "" {
			return
		}
	default:
		return
	}
	h := sha1.New()
	for _, f := range md.Fields {
		name := strings.ToLower(f.Name)
		if matched, _ := withFields[name]; !matched {
			continue
		}
		f.Pull()
		switch f.Value.Kind() {
		case reflect.String:
			h.Write([]byte(f.string))
		case reflect.Bool,
			reflect.Int,
			reflect.Int8,
			reflect.Int16,
			reflect.Int32,
			reflect.Int64:
			bfr := new(bytes.Buffer)
			binary.Write(bfr, binary.BigEndian, f.int)
			h.Write(bfr.Bytes())
		}
	}
	pk.string = hex.EncodeToString(h.Sum(nil))
	pk.Push()
}

//
// Get constraint DDL.
func (t Table) Constraints(md *Definition, dm *DataModel) (constraints []string, err error) {
	constraints = []string{}
	unique := map[string][]string{}
	for _, field := range md.Fields {
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
	fkRelation := FkRelation{dm: dm}
	ddl, err := fkRelation.DDL(md)
	if err != nil {
		return
	}
	constraints = append(
		constraints,
		ddl...)

	return
}

//
// Reflect auto-incremented fields.
// Field.int is incremented by Field.Push() called when the
// SQL statement is built. This needs to be propagated to the model.
func (t *Table) reflectIncremented(md *Definition) {
	for _, f := range md.Fields {
		if f.Incremented() {
			f.Value.SetInt(f.int)
		}
	}
}

//
// Build model insert SQL.
func (t Table) insertSQL(md *Definition) (sql string, err error) {
	tpl := template.New("")
	tpl, err = tpl.Parse(InsertSQL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table:  md.Kind,
			Fields: md.RealFields(md.Fields),
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	sql = bfr.String()

	return
}

//
// Build model update SQL.
func (t Table) updateSQL(md *Definition, options *FilterOptions) (sql string, err error) {
	tpl := template.New("")
	tpl, err = tpl.Parse(UpdateSQL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = options.Build(md)
	if err != nil {
		return
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table:   md.Kind,
			Fields:  md.MutableFields(),
			Options: options,
			Pk:      md.PkField(),
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	sql = bfr.String()

	return
}

//
// Build model delete SQL.
func (t Table) deleteSQL(md *Definition) (sql string, err error) {
	tpl := template.New("")
	tpl, err = tpl.Parse(DeleteSQL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table: md.Kind,
			Pk:    md.PkField(),
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	sql = bfr.String()

	return
}

//
// Build model get SQL.
func (t Table) getSQL(md *Definition) (sql string, err error) {
	tpl := template.New("")
	tpl, err = tpl.Parse(GetSQL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table:  md.Kind,
			Pk:     md.PkField(),
			Fields: md.Fields,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	sql = bfr.String()

	return
}

//
// Build model list SQL.
func (t Table) listSQL(md *Definition, options *ListOptions) (sql string, err error) {
	tpl := template.New("")
	tpl, err = tpl.Parse(ListSQL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = options.Build(md)
	if err != nil {
		return
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table:   md.Kind,
			Fields:  md.Fields,
			Options: options,
			Pk:      md.PkField(),
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	sql = bfr.String()

	return
}

//
// Build model count SQL.
func (t Table) countSQL(md *Definition, options *FilterOptions) (sql string, err error) {
	tpl := template.New("")
	tpl, err = tpl.Parse(ListSQL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = options.Build(md)
	if err != nil {
		return
	}
	bfr := &bytes.Buffer{}
	err = tpl.Execute(
		bfr,
		TmplData{
			Table:   md.Kind,
			Fields:  md.Fields,
			Options: options,
			Count:   true,
			Pk:      md.PkField(),
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	sql = bfr.String()

	return
}

//
// Scan the fetch row into the model.
// The model fields are updated.
func (t Table) scan(row Row, fields []*Field) (err error) {
	list := []interface{}{}
	for _, f := range fields {
		f.Pull()
		list = append(list, f.Ptr())
	}
	err = row.Scan(list...)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, f := range fields {
		f.Push()
	}

	return
}

//
// Template data.
type TmplData struct {
	// Table name.
	Table string
	// Index name.
	Index string
	// Fields.
	Fields []*Field
	// Constraint DDL.
	Constraints []string
	// Natural key fields.
	Keys []*Field
	// Primary key.
	Pk *Field
	// Filter options.
	Options *FilterOptions
	// Count
	Count bool
}

//
// Predicate
func (t TmplData) Predicate() Predicate {
	return t.Options.Predicate
}

//
// Pagination.
func (t TmplData) Page() *Page {
	return t.Options.Page
}

//
// Sort criteria
func (t TmplData) Sort() []int {
	return t.Options.Sort
}

//
// FilterOptions options.
type FilterOptions struct {
	// Pagination.
	Page *Page
	// Sort by field position.
	Sort []int
	// Field detail level.
	// Defaults:
	//   0 = primary and natural fields.
	//   1 = other fields.
	Detail int
	// Predicate
	Predicate Predicate
	// Table (name).
	table string
	// Fields.
	fields []*Field
	// Params.
	params []interface{}
}

//
// Validate options.
func (l *FilterOptions) Build(md *Definition) (err error) {
	l.table = md.Kind
	l.fields = md.Fields
	if l.Predicate != nil {
		err = l.Predicate.Build(l)
	}

	return
}

//
// Get an appropriate parameter name.
// Builds a parameter and adds it to the options.param list.
func (l *FilterOptions) Param(name string, value interface{}) (p string) {
	name = fmt.Sprintf("%s%d", name, len(l.params))
	l.params = append(l.params, sql.Named(name, value))
	p = ":" + name
	return
}

//
// Fields filtered by detail level.
func (l *FilterOptions) Fields() (filtered []*Field) {
	for _, f := range l.fields {
		if f.MatchDetail(l.Detail) {
			filtered = append(filtered, f)
		}
	}

	return
}

//
// Get params referenced by the predicate.
func (l *FilterOptions) Params() []interface{} {
	return l.params
}

//
// List options
type ListOptions = FilterOptions

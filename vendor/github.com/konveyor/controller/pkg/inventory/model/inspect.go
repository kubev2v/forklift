package model

import (
	liberr "github.com/konveyor/controller/pkg/error"
	fb "github.com/konveyor/controller/pkg/filebacked"
	"reflect"
	"strings"
)

func Inspect(model interface{}) (md *Definition, err error) {
	md = &Definition{
		model: model,
	}
	md.Kind = md.kind(model)
	md.Fields, err = md.fields(model)
	if err != nil {
		return
	}
	err = md.validate()
	if err != nil {
		return
	}

	return
}

//
// Model definition.
type Definition struct {
	Kind   string
	Fields []*Field
	model  interface{}
}

//
// Get the mutable `Fields` for the model.
func (r *Definition) MutableFields() []*Field {
	list := []*Field{}
	for _, f := range r.Fields {
		if f.Mutable() {
			list = append(list, f)
		}
	}

	return list
}

//
// Get the natural key `Fields` for the model.
func (r *Definition) KeyFields() []*Field {
	list := []*Field{}
	for _, f := range r.Fields {
		if f.Key() {
			list = append(list, f)
		}
	}

	return list
}

//
// Get foreign keys for the model.
func (r *Definition) Fks() []*FK {
	list := []*FK{}
	for _, f := range r.Fields {
		fk := f.Fk()
		if fk != nil {
			list = append(list, fk)
		}
	}

	return list
}

//
// Get the non-virtual `Fields` for the model.
func (r *Definition) RealFields(fields []*Field) []*Field {
	list := []*Field{}
	for _, f := range fields {
		if !f.Virtual() {
			list = append(list, f)
		}
	}

	return list
}

//
// Get the PK field.
func (r *Definition) PkField() *Field {
	for _, f := range r.Fields {
		if f.Pk() {
			return f
		}
	}

	return nil
}

//
// Field by name.
func (r *Definition) Field(name string) *Field {
	name = strings.ToLower(name)
	for _, f := range r.Fields {
		if strings.ToLower(f.Name) == name {
			return f
		}
	}

	return nil
}

//
// Match (case-insensitive) by kind.
func (r *Definition) IsKind(kind string) bool {
	return strings.ToLower(kind) == strings.ToLower(r.Kind)
}

//
// Get the table name for the model.
func (r Definition) kind(model interface{}) string {
	mt := reflect.TypeOf(model)
	if mt.Kind() == reflect.Ptr {
		mt = mt.Elem()
	}

	return mt.Name()
}

//
// Validate the model.
func (r *Definition) validate() (err error) {
	for _, f := range r.Fields {
		err = f.Validate()
		if err != nil {
			return
		}
	}
	pk := r.PkField()
	if pk == nil {
		err = liberr.Wrap(MustHavePkErr)
	}

	return
}

//
// Get the `Fields` for the model.
func (r *Definition) fields(model interface{}) (fields []*Field, err error) {
	mt := reflect.TypeOf(model)
	mv := reflect.ValueOf(model)
	if mt.Kind() == reflect.Ptr {
		mt = mt.Elem()
		mv = mv.Elem()
	} else {
		err = liberr.Wrap(MustBePtrErr)
		return
	}
	if mv.Kind() != reflect.Struct {
		err = liberr.Wrap(MustBeObjectErr)
		return
	}
	for i := 0; i < mt.NumField(); i++ {
		ft := mt.Field(i)
		fv := mv.Field(i)
		if !fv.CanSet() {
			continue
		}
		switch fv.Kind() {
		case reflect.Struct:
			sqlTag, found := ft.Tag.Lookup(Tag)
			if found {
				if sqlTag == "-" {
					break
				}
				fields = append(
					fields,
					&Field{
						Tag:   sqlTag,
						Name:  ft.Name,
						Value: &fv,
						Type:  &ft,
					})
			} else {
				nested, nErr := r.fields(fv.Addr().Interface())
				if nErr != nil {
					return
				}
				fields = append(fields, nested...)
			}
		case reflect.Slice,
			reflect.Map,
			reflect.String,
			reflect.Bool,
			reflect.Int,
			reflect.Int8,
			reflect.Int16,
			reflect.Int32,
			reflect.Int64:
			sqlTag, _ := ft.Tag.Lookup(Tag)
			if sqlTag == "-" {
				continue
			}
			fields = append(
				fields,
				&Field{
					Tag:   sqlTag,
					Name:  ft.Name,
					Value: &fv,
					Type:  &ft,
				})
		}
	}

	return
}

//
// New model for kind.
func (r *Definition) NewModel() (m interface{}) {
	mt := reflect.TypeOf(r.model)
	if mt.Kind() == reflect.Ptr {
		mt = mt.Elem()
	}
	m = reflect.New(mt).Interface()
	return
}

//
// New data Model.
func NewModel(models []interface{}) (dm *DataModel, err error) {
	dm = &DataModel{
		content: make(map[string]*Definition),
	}
	for _, m := range models {
		var md *Definition
		md, err = Inspect(m)
		if err != nil {
			return
		}
		dm.Add(md)
	}

	return
}

//
// DataModel.
// Map of definitions.
type DataModel struct {
	content map[string]*Definition
}

//
// Add definition.
func (r *DataModel) Add(md *Definition) {
	key := strings.ToLower(md.Kind)
	r.content[key] = md
}

//
// Definitions.
func (r *DataModel) Definitions() (list Definitions) {
	list = Definitions{}
	for _, md := range r.content {
		list = append(list, md)
	}

	return
}

//
// Build the DDL.
func (r *DataModel) DDL() (list []string, err error) {
	fkRelation := FkRelation{dm: r}
	for _, md := range fkRelation.Definitions() {
		var ddl []string
		ddl, err = Table{}.DDL(md.model, r)
		if err != nil {
			return
		}
		list = append(list, ddl...)
	}

	return
}

//
// Find by kind.
func (r *DataModel) Find(kind string) (md *Definition, found bool) {
	key := strings.ToLower(kind)
	md, found = r.content[key]
	return
}

//
// Find with model.
func (r *DataModel) FindWith(model interface{}) (md *Definition, found bool) {
	kind := Definition{}.kind(model)
	md, found = r.Find(kind)
	return
}

//
// Find models to be (cascade) deleted.
func (r *DataModel) Deleted(tx *Tx, model interface{}) (cascaded fb.Iterator, err error) {
	md, err := Inspect(model)
	if err != nil {
		return
	}
	relation := &FkRelation{dm: r}
	cascaded, err = r.cascade(tx, relation, md)
	if err != nil {
		return
	}
	cascaded.Reverse()
	return
}

//
// Find models to be (cascade) deleted.
func (r *DataModel) cascade(tx *Tx, relation *FkRelation, md *Definition) (cascaded fb.Iterator, err error) {
	list := fb.NewList()
	referencing := relation.Referencing(md)
	pk := md.PkField()
	pkID := pk.Pull()
	for _, ref := range referencing {
		if !ref.cascade {
			continue
		}
		refModel := ref.md.NewModel()
		var iter fb.Iterator
		iter, err = tx.Find(
			refModel,
			ListOptions{
				Predicate: Eq(
					ref.field,
					pkID),
			})
		if err != nil {
			return
		}
		for {
			var refMd *Definition
			model, hasNext := iter.Next()
			if !hasNext {
				break
			}
			list.Append(model)
			refMd, err = Inspect(model)
			if err != nil {
				return
			}
			var nIter fb.Iterator
			nIter, err = r.cascade(tx, relation, refMd)
			if err != nil {
				return
			}
			list.Append(nIter)
		}
	}

	cascaded = list.Iter()
	list.Close()

	return
}

//
// Model definitions.
type Definitions []*Definition

//
// Append model definition.
func (r *Definitions) Append(md *Definition) {
	*r = append(*r, md)
}

//
// Push model definition.
func (r *Definitions) Push(md *Definition) {
	r.Append(md)
}

//
// Head (first) model definition.
// Returns: nil when empty.
func (r *Definitions) Head(delete bool) (md *Definition) {
	if len(*r) > 0 {
		md = (*r)[0]
	}
	if delete {
		*r = (*r)[1:]
	}
	return
}

//
// Top (last) model definition.
// Returns: nil when empty.
func (r *Definitions) Top() (md *Definition) {
	if len(*r) > 0 {
		last := len(*r) - 1
		md = (*r)[last]
	}

	return
}

//
// Pop model definition.
// Returns: nil when empty.
func (r *Definitions) Pop() (md *Definition) {
	if len(*r) > 0 {
		last := len(*r) - 1
		md = (*r)[last]
		*r = (*r)[:last]
	}

	return
}

//
// Delete model definition.
func (r *Definitions) Delete(index int) {
	_ = (*r)[index]
	s := (*r)[:index]
	if index < len(*r)-1 {
		s = append(s, (*r)[index+1:]...)
	}
	*r = s
}

//
// Reverse.
func (r *Definitions) Reverse() {
	if len(*r) == 0 {
		return
	}
	reversed := []*Definition{}
	for i := len(*r) - 1; i >= 0; i-- {
		reversed = append(reversed, (*r)[i])
	}
	*r = reversed
	return
}

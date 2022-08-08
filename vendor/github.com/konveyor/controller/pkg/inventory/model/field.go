package model

import (
	"encoding/json"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/pkg/errors"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

const (
	// SQL tag.
	Tag = "sql"
	// Max detail level.
	MaxDetail = 9
)

// The default (field) detail level when listing models.
// Applications using custom detail levels
// must adjust to highest level used.
// Example:
//   func init() {
//     model.DefaultDetail = 2
//   }
var DefaultDetail = 0

//
// Regex used for `pk(fields)` tags.
var PkRegex = regexp.MustCompile(`(pk)((\()(.+)(\)))?`)

//
// Regex used for `unique(group)` tags.
var UniqueRegex = regexp.MustCompile(`(unique)(\()(.+)(\))`)

//
// Regex used for `index(group)` tags.
var IndexRegex = regexp.MustCompile(`(index)(\()(.+)(\))`)

//
// Regex used for `fk(table)` tags.
var FkRegex = regexp.MustCompile(`(fk)(\()(.+)(\))`)

//
// Regex used for detail.
var DetailRegex = regexp.MustCompile(`(d)([0-9]+)`)

//
// Model (struct) Field
type Field struct {
	// reflect.Type of the field.
	Type *reflect.StructField
	// reflect.Value of the field.
	Value *reflect.Value
	// Field name.
	Name string
	// SQL tag.
	Tag string
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
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		if len(f.WithFields()) > 0 {
			return liberr.Wrap(GenPkTypeErr)
		}
	default:
		if f.Pk() {
			return liberr.Wrap(PkTypeErr)
		}
	}
	if f.Detail() > MaxDetail {
		return liberr.Wrap(DetailErr)
	}

	return nil
}

//
// Pull from model.
// Populate the appropriate `staging` field using the
// model field value.
func (f *Field) Pull() interface{} {
	switch f.Value.Kind() {
	case reflect.Struct:
		object := f.Value.Interface()
		b, err := json.Marshal(&object)
		if err == nil {
			f.string = string(b)
		}
		return f.string
	case reflect.Slice:
		if !f.Value.IsNil() {
			object := f.Value.Interface()
			b, err := json.Marshal(&object)
			if err == nil {
				f.string = string(b)
			}
		} else {
			f.string = "[]"
		}
		return f.string
	case reflect.Map:
		if !f.Value.IsNil() {
			object := f.Value.Interface()
			b, err := json.Marshal(&object)
			if err == nil {
				f.string = string(b)
			}
		} else {
			f.string = "{}"
		}
		return f.string
	case reflect.String:
		f.string = f.Value.String()
		return f.string
	case reflect.Bool:
		b := f.Value.Bool()
		if b {
			f.int = 1
		}
		return f.int
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		f.int = f.Value.Int()
		if f.Incremented() {
			f.int++
		}
		return f.int
	}

	return nil
}

//
// Pointer used for Scan().
func (f *Field) Ptr() interface{} {
	switch f.Value.Kind() {
	case reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return &f.int
	default:
		return &f.string
	}
}

//
// Push to the model.
// Set the model field value using the `staging` field.
func (f *Field) Push() {
	switch f.Value.Kind() {
	case reflect.Struct:
		if len(f.string) == 0 {
			break
		}
		tv := reflect.New(f.Value.Type())
		object := tv.Interface()
		err := json.Unmarshal([]byte(f.string), &object)
		if err == nil {
			tv = reflect.ValueOf(object)
			f.Value.Set(tv.Elem())
		}
	case reflect.Slice,
		reflect.Map:
		if len(f.string) == 0 {
			break
		}
		tv := reflect.New(f.Value.Type())
		object := tv.Interface()
		err := json.Unmarshal([]byte(f.string), object)
		if err == nil {
			tv = reflect.ValueOf(object)
			tv = reflect.Indirect(tv)
			f.Value.Set(tv)
		}
	case reflect.String:
		f.Value.SetString(f.string)
	case reflect.Bool:
		b := false
		if f.int != 0 {
			b = true
		}
		f.Value.SetBool(b)
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
	case reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		part[1] = "INTEGER"
	default:
		part[1] = "TEXT"
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
// Get whether field is the primary key.
func (f *Field) Pk() (matched bool) {
	for _, opt := range strings.Split(f.Tag, ",") {
		m := PkRegex.FindStringSubmatch(opt)
		if m != nil {
			matched = true
			break
		}
	}
	return
}

//
// Fields used to generate the primary key.
// Map of lower-cased field names. May be empty
// when generation is not enabled.
func (f *Field) WithFields() (withFields map[string]bool) {
	withFields = map[string]bool{}
	for _, opt := range strings.Split(f.Tag, ",") {
		opt = strings.TrimSpace(opt)
		m := PkRegex.FindStringSubmatch(opt)
		if len(m) == 6 {
			for _, name := range strings.Split(m[4], ";") {
				name = strings.TrimSpace(name)
				if len(name) > 0 {
					name = strings.ToLower(name)
					withFields[name] = true
				}
			}
		}
		break
	}

	return
}

//
// Get whether field is mutable.
// Only mutable fields will be updated.
func (f *Field) Mutable() bool {
	if f.Pk() || f.Key() || f.Virtual() {
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
// Get whether field is virtual.
// A `virtual` field is read-only and managed
// internally in the DB.
func (f *Field) Virtual() bool {
	return f.hasOpt("virtual")
}

//
// Get whether the field is unique.
func (f *Field) Unique() []string {
	list := []string{}
	for _, opt := range strings.Split(f.Tag, ",") {
		opt = strings.TrimSpace(opt)
		m := UniqueRegex.FindStringSubmatch(opt)
		if len(m) == 5 {
			list = append(list, m[3])
		}
	}

	return list
}

//
// Get whether the field has non-unique index.
func (f *Field) Index() []string {
	list := []string{}
	for _, opt := range strings.Split(f.Tag, ",") {
		opt = strings.TrimSpace(opt)
		m := IndexRegex.FindStringSubmatch(opt)
		if len(m) == 5 {
			list = append(list, m[3])
		}
	}

	return list
}

//
// Get whether the field is a foreign key.
// Format: fk(table flags..) where flags are optional.
// Flags:
//   +must = referenced model must exist.
//   +cascade = cascade delete.
func (f *Field) Fk() (fk *FK) {
	for _, opt := range strings.Split(f.Tag, ",") {
		opt = strings.TrimSpace(opt)
		m := FkRegex.FindStringSubmatch(opt)
		if len(m) == 5 {
			table := strings.Fields(m[3])
			fk = &FK{
				Table: table[0],
				Owner: f,
			}
			if len(table) > 1 {
				for _, flag := range table[1:] {
					switch strings.TrimSpace(flag) {
					case "+cascade":
						fk.Cascade = true
					case "+must":
						fk.Must = true
					default:
						panic(
							errors.Errorf(
								"FK %s not supported",
								flag))
					}
				}
			}
			break
		}
	}

	return
}

//
// Get whether field is auto-incremented.
func (f *Field) Incremented() bool {
	return f.hasOpt("incremented")
}

// Convert the specified `object` to a value
// (type) appropriate for the field.
func (f *Field) AsValue(object interface{}) (value interface{}, err error) {
	val := reflect.ValueOf(object)
	switch val.Kind() {
	case reflect.Ptr:
		val = val.Elem()
	case reflect.Struct,
		reflect.Slice,
		reflect.Map:
		err = liberr.Wrap(PredicateValueErr)
		return
	}
	switch f.Value.Kind() {
	case reflect.String:
		switch val.Kind() {
		case reflect.String:
			value = val.String()
		case reflect.Bool:
			b := val.Bool()
			value = strconv.FormatBool(b)
		case reflect.Int,
			reflect.Int8,
			reflect.Int16,
			reflect.Int32,
			reflect.Int64:
			n := val.Int()
			value = strconv.FormatInt(n, 0)
		default:
			err = liberr.Wrap(PredicateValueErr)
		}
	case reflect.Bool:
		switch val.Kind() {
		case reflect.String:
			s := val.String()
			b, pErr := strconv.ParseBool(s)
			if pErr != nil {
				err = liberr.Wrap(pErr)
				return
			}
			value = b
		case reflect.Bool:
			value = val.Bool()
		case reflect.Int,
			reflect.Int8,
			reflect.Int16,
			reflect.Int32,
			reflect.Int64:
			n := val.Int()
			if n != 0 {
				value = true
			} else {
				value = false
			}
		default:
			err = liberr.Wrap(PredicateValueErr)
		}
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		switch val.Kind() {
		case reflect.String:
			n, err := strconv.ParseInt(val.String(), 0, 64)
			if err != nil {
				err = liberr.Wrap(err)
			}
			value = n
		case reflect.Bool:
			if val.Bool() {
				value = 1
			} else {
				value = 0
			}
		case reflect.Int,
			reflect.Int8,
			reflect.Int16,
			reflect.Int32,
			reflect.Int64:
			value = val.Int()
		default:
			err = liberr.Wrap(PredicateValueErr)
		}
	default:
		err = liberr.Wrap(FieldTypeErr)
	}

	return
}

//
// Get whether the field is `json` encoded.
func (f *Field) Encoded() (encoded bool) {
	switch f.Value.Kind() {
	case reflect.Struct,
		reflect.Slice,
		reflect.Map:
		encoded = true
	}

	return
}

//
// Detail level.
// Defaults:
//   Key fields = 0.
//        Other = DefaultDetail
func (f *Field) Detail() (level int) {
	level = DefaultDetail
	for _, opt := range strings.Split(f.Tag, ",") {
		opt = strings.TrimSpace(opt)
		m := DetailRegex.FindStringSubmatch(opt)
		if len(m) == 3 {
			level, _ = strconv.Atoi(m[2])
			return
		}
	}
	if f.Pk() || f.Key() {
		level = 0
		return
	}

	return
}

//
// Match detail level.
func (f *Field) MatchDetail(level int) bool {
	return f.Detail() <= level
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
	// FK owner.
	Owner *Field
	// Target table name.
	Table string
	// Target field name.
	Field string
	// +must option enforced by constraint.
	Must bool
	// +cascade delete option.
	Cascade bool
}

//
// Get DDL.
func (f *FK) DDL(field *Field) string {
	return fmt.Sprintf(
		"FOREIGN KEY (%s) REFERENCES %s (%s)",
		field.Name,
		f.Table,
		f.Field)
}

//
// Get whether the FK Needs an explicit index.
// The `must` is implemented by a constraint which is
// implicitly indexed by the DB.
func (f *FK) needsIndex() bool {
	return f.Cascade && !f.Must
}

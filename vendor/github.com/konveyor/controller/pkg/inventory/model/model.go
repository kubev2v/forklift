package model

import (
	"database/sql"
	"errors"
	_ "github.com/mattn/go-sqlite3"
	"reflect"
)

//
// Errors.
var Conflict = errors.New("conflict")
var NotFound = sql.ErrNoRows

//
// Database client interface.
// Support model methods taking either sql.DB or sql.Tx.
type DBTX interface {
	Exec(string, ...interface{}) (sql.Result, error)
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) *sql.Row
}

//
// Database interface.
// Support model `Scan` taking either sql.Row or sql.Rows.
type Row interface {
	Scan(...interface{}) error
}

//
// Page.
// Support pagination.
type Page struct {
	// The page offset.
	Offset int
	// The number of items per/page.
	Limit int
}

//
// Slice the collection according to the page definition.
// The `collection` must be a pointer to a `Slice` which is
// modified as needed.
func (p *Page) Slice(collection interface{}) {
	v := reflect.ValueOf(collection)
	switch v.Kind() {
	case reflect.Ptr:
		v = v.Elem()
	default:
		return
	}
	switch v.Kind() {
	case reflect.Slice:
		sliced := reflect.MakeSlice(v.Type(), 0, 0)
		for i := 0; i < v.Len(); i++ {
			if i < p.Offset {
				continue
			}
			if sliced.Len() == p.Limit {
				break
			}
			sliced = reflect.Append(sliced, v.Index(i))
		}
		v.Set(sliced)
	}
}

//
// Model
// Each model represents a table in the DB.
type Model interface {
	// Get the primary key.
	Pk() string
	// Set the primary key based on natural keys.
	SetPk()
	// Get description of the model.
	String() string
	// Equal comparison.
	Equals(other Model) bool
	// Get labels.
	// Optional and may return nil.
	Labels() Labels
}

type Base struct {
	// Primary key (digest).
	PK string `sql:"pk"`
	// The raw json-encoded k8s resource.
	Object string `sql:""`
}

//
// Get the primary key.
func (m *Base) Pk() string {
	return m.PK
}

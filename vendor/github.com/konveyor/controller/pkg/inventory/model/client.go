package model

import (
	"database/sql"
	liberr "github.com/konveyor/controller/pkg/error"
	"os"
	"reflect"
	"sync"
)

const (
	Pragma = "PRAGMA foreign_keys = ON"
)

//
// Database client.
type DB interface {
	// Open and build the schema.
	Open(bool) error
	// Close.
	Close(bool) error
	// Get the specified model.
	Get(Model) error
	// List models based on the type of slice.
	List(interface{}, ListOptions) error
	// Count based on the specified model.
	Count(Model, Predicate) (int64, error)
	// Begin a transaction.
	Begin() (*Tx, error)
	// Insert a model.
	Insert(Model) error
	// Update a model.
	Update(Model) error
	// Delete a model.
	Delete(Model) error
	// Watch a model collection.
	Watch(Model, EventHandler) (*Watch, error)
	// End a watch.
	EndWatch(watch *Watch)
	// The journal
	Journal() *Journal
}

//
// Database client.
type Client struct {
	labeler Labeler
	// The sqlite3 database will not support
	// concurrent write operations.
	dbMutex sync.Mutex
	// file path.
	path string
	// Model
	models []interface{}
	// Database connection.
	db *sql.DB
	// Journal
	journal Journal
}

//
// Create the database.
// Build the schema to support the specified models.
// Optionally `purge` (delete) the DB first.
func (r *Client) Open(purge bool) error {
	if purge {
		os.Remove(r.path)
	}
	db, err := sql.Open("sqlite3", r.path)
	if err != nil {
		panic(err)
	}
	statements := []string{Pragma}
	r.models = append(r.models, &Label{})
	for _, m := range r.models {
		ddl, err := Table{}.DDL(m)
		if err != nil {
			panic(err)
		}
		statements = append(statements, ddl...)
	}
	for _, ddl := range statements {
		_, err = db.Exec(ddl)
		if err != nil {
			db.Close()
			return liberr.Wrap(err)
		}
	}

	r.db = db

	return nil
}

//
// Close the database.
// Optionally purge (delete) the DB.
func (r *Client) Close(purge bool) error {
	if r.db == nil {
		return nil
	}
	r.journal.Disable()
	err := r.db.Close()
	if err != nil {
		return liberr.Wrap(err)
	}
	r.db = nil
	if purge {
		os.Remove(r.path)
	}

	return nil
}

//
// Get the model.
func (r *Client) Get(model Model) error {
	return Table{r.db}.Get(model)
}

//
// List models.
// The `list` must be: *[]Model.
func (r *Client) List(list interface{}, options ListOptions) error {
	return Table{r.db}.List(list, options)
}

//
// Count models.
func (r *Client) Count(model Model, predicate Predicate) (int64, error) {
	return Table{r.db}.Count(model, predicate)
}

//
// Begin a transaction.
// Example:
//   tx, _ := client.Begin()
//   defer tx.End()
//   client.Insert(model)
//   client.Insert(model)
//   tx.Commit()
func (r *Client) Begin() (*Tx, error) {
	r.dbMutex.Lock()
	real, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	tx := &Tx{
		dbMutex: &r.dbMutex,
		journal: &r.journal,
		real:    real,
	}

	return tx, nil
}

//
// Insert the model.
func (r *Client) Insert(model Model) error {
	r.dbMutex.Lock()
	defer r.dbMutex.Unlock()
	table := Table{r.db}
	err := table.Insert(model)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.labeler.Insert(table, model)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.journal.Created(model)
	r.journal.Commit()

	return nil
}

//
// Update the model.
func (r *Client) Update(model Model) error {
	r.dbMutex.Lock()
	defer r.dbMutex.Unlock()
	table := Table{r.db}
	current := r.journal.copy(model)
	err := table.Get(current)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = table.Update(model)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.labeler.Replace(table, model)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.journal.Updated(current, model)
	r.journal.Commit()

	return nil
}

//
// Delete the model.
func (r *Client) Delete(model Model) error {
	r.dbMutex.Lock()
	defer r.dbMutex.Unlock()
	table := Table{r.db}
	err := table.Delete(model)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.labeler.Delete(table, model)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.journal.Deleted(model)
	r.journal.Commit()

	return nil
}

//
// Watch model events.
func (r *Client) Watch(model Model, handler EventHandler) (*Watch, error) {
	mt := reflect.TypeOf(model)
	switch mt.Kind() {
	case reflect.Ptr:
		mt = mt.Elem()
	}
	watch, err := r.journal.Watch(model, handler)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	listPtr := reflect.New(reflect.SliceOf(mt))
	err = Table{r.db}.List(listPtr.Interface(), ListOptions{})
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	list := listPtr.Elem()
	for i := 0; i < list.Len(); i++ {
		m := list.Index(i).Addr().Interface()
		watch.notify(
			&Event{
				Model:  m.(Model),
				Action: Created,
			})
	}

	watch.Start()

	return watch, nil
}

//
// End watch.
func (r *Client) EndWatch(watch *Watch) {
	r.journal.End(watch)
}

//
// The associated journal.
func (r *Client) Journal() *Journal {
	return &r.journal
}

//
// Database transaction.
type Tx struct {
	labeler Labeler
	// Associated client.
	dbMutex *sync.Mutex
	// Journal
	journal *Journal
	// Reference to real sql.Tx.
	real *sql.Tx
	// Ended
	ended bool
}

//
// Get the model.
func (r *Tx) Get(model Model) error {
	return Table{r.real}.Get(model)
}

//
// List models.
// The `list` must be: *[]Model.
func (r *Tx) List(list interface{}, options ListOptions) error {
	return Table{r.real}.List(list, options)
}

//
// Count models.
func (r *Tx) Count(model Model, predicate Predicate) (int64, error) {
	return Table{r.real}.Count(model, predicate)
}

//
// Insert the model.
func (r *Tx) Insert(model Model) error {
	table := Table{r.real}
	err := table.Insert(model)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.labeler.Insert(table, model)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.journal.Created(model)

	return nil
}

//
// Update the model.
func (r *Tx) Update(model Model) error {
	table := Table{r.real}
	current := r.journal.copy(model)
	err := table.Get(current)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = table.Update(model)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.labeler.Replace(table, model)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.journal.Updated(current, model)

	return nil
}

//
// Delete the model.
func (r *Tx) Delete(model Model) error {
	table := Table{r.real}
	err := table.Delete(model)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.labeler.Delete(table, model)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.journal.Deleted(model)

	return nil
}

//
// Commit a transaction.
// Staged changes are committed in the DB.
// This will end the transaction.
func (r *Tx) Commit() (err error) {
	if r.ended {
		return
	}
	defer func() {
		r.dbMutex.Unlock()
		r.ended = true
	}()
	err = r.real.Commit()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	r.journal.Commit()

	return
}

//
// End a transaction.
// Staged changes are discarded.
// See: Commit().
func (r *Tx) End() (err error) {
	if r.ended {
		return
	}
	defer func() {
		r.dbMutex.Unlock()
		r.ended = true
	}()
	err = r.real.Rollback()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	r.journal.Unstage()

	return
}

//
// Labeler.
type Labeler struct {
}

//
// Insert labels for the model into the DB.
func (r *Labeler) Insert(table Table, model Model) error {
	for l, v := range model.Labels() {
		label := &Label{
			Parent: model.Pk(),
			Kind:   table.Name(model),
			Name:   l,
			Value:  v,
		}
		err := table.Insert(label)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}

//
// Delete labels for a model in the DB.
func (r *Labeler) Delete(table Table, model Model) error {
	list := []Label{}
	err := table.List(
		&list,
		ListOptions{
			Predicate: And(
				Eq("Kind", table.Name(model)),
				Eq("Parent", model.Pk())),
		})
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, label := range list {
		err := table.Delete(&label)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}

//
// Replace labels.
func (r *Labeler) Replace(table Table, model Model) error {
	err := r.Delete(table, model)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.Insert(table, model)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

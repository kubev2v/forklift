package model

import (
	"database/sql"
	"errors"
	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	fb "github.com/konveyor/controller/pkg/filebacked"
	"os"
	"time"
)

//
// Database client.
type DB interface {
	// Open and build the schema.
	Open(bool) error
	// Close.
	Close(bool) error
	// Execute SQL.
	Execute(sql string) (sql.Result, error)
	// Get the specified model.
	Get(Model) error
	// List models based on the type of slice.
	List(interface{}, ListOptions) error
	// Find models.
	Find(interface{}, ListOptions) (fb.Iterator, error)
	// Count based on the specified model.
	Count(Model, Predicate) (int64, error)
	// Begin a transaction.
	Begin(...string) (*Tx, error)
	// Insert a model.
	Insert(Model) error
	// Update a model.
	Update(Model, ...Predicate) error
	// Delete a model.
	Delete(Model) error
	// Watch a model collection.
	Watch(Model, EventHandler) (*Watch, error)
	// End a watch.
	EndWatch(watch *Watch)
}

//
// Database client.
type Client struct {
	// file path.
	path string
	// Model
	models []interface{}
	// Overall data model.
	dm *DataModel
	// Session pool.
	pool Pool
	// Journal
	journal Journal
	// Logger
	log logr.Logger
}

//
// Create the database.
// Build the schema to support the specified models.
// See: Pool.Open().
func (r *Client) Open(delete bool) (err error) {
	if delete {
		_ = os.Remove(r.path)
		r.log.V(3).Info("DB file deleted.")
	}
	err = r.pool.Open(1, 10, r.path, &r.journal)
	if err != nil {
		r.log.V(3).Error(err, "open session pool failed.")
		panic(err)
	}
	defer func() {
		if err != nil {
			_ = r.pool.Close()
			_ = os.Remove(r.path)
		}
	}()
	err = r.build()
	if err != nil {
		panic(err)
	}

	r.log.V(3).Info("session pool opened.")

	return
}

//
// Close the database.
// The session pool and journal are closed.
func (r *Client) Close(delete bool) (err error) {
	jErr := r.journal.Close()
	if jErr != nil {
		r.log.Error(
			jErr,
			"Error closing the journal.")
	}
	pErr := r.pool.Close()
	if pErr != nil {
		r.log.Error(
			pErr,
			"Error closing the session pool.")
	}
	if delete {
		_ = os.Remove(r.path)
		r.log.V(3).Info("DB file deleted.")
	}

	r.log.V(3).Info("DB closed.")

	return
}

//
// Execute SQL.
// Delegated to Tx.Execute().
func (r *Client) Execute(sql string) (result sql.Result, err error) {
	tx, err := r.Begin()
	if err != nil {
		return
	}
	result, err = tx.Execute(sql)
	return
}

//
// Get the model.
func (r *Client) Get(model Model) (err error) {
	session := r.pool.Reader()
	defer session.Return()
	mark := time.Now()
	err = Table{session.db}.Get(model)
	if err == nil {
		r.log.V(4).Info(
			"get succeeded.",
			"model",
			Describe(model),
			"duration",
			time.Since(mark))
	}

	return
}

//
// List models.
// The `list` must be: *[]Model.
func (r *Client) List(list interface{}, options ListOptions) (err error) {
	session := r.pool.Reader()
	defer session.Return()
	mark := time.Now()
	err = Table{session.db}.List(list, options)
	if err == nil {
		r.log.V(4).Info(
			"list succeeded.",
			"options",
			options,
			"duration",
			time.Since(mark))
	}

	return
}

//
// Find models.
func (r *Client) Find(model interface{}, options ListOptions) (itr fb.Iterator, err error) {
	session := r.pool.Reader()
	defer session.Return()
	mark := time.Now()
	itr, err = Table{session.db}.Find(model, options)
	if err == nil {
		r.log.V(4).Info(
			"list succeeded.",
			"options",
			options,
			"duration",
			time.Since(mark))
	}

	return
}

//
// Count models.
func (r *Client) Count(model Model, predicate Predicate) (n int64, err error) {
	session := r.pool.Reader()
	defer session.Return()
	mark := time.Now()
	n, err = Table{session.db}.Count(model, predicate)
	if err == nil {
		r.log.V(4).Info(
			"count succeeded.",
			"predicate",
			predicate,
			"duration",
			time.Since(mark))
	}

	return
}

//
// Begin a transaction.
func (r *Client) Begin(labels ...string) (tx *Tx, error error) {
	mark := time.Now()
	session := r.pool.Writer()
	realTx, err := session.Begin()
	if err != nil {
		err = liberr.Wrap(
			err,
			"db",
			r.path)
		return
	}
	tx = &Tx{
		session: session,
		real:    realTx,
		journal: &r.journal,
		staged:  fb.NewList(),
		dm:      r.dm,
		labeler: Labeler{
			tx:  realTx,
			log: r.log,
		},
		started: time.Now(),
		labels:  labels,
		log:     r.log,
	}

	r.log.V(4).Info("tx begin.", "duration", time.Since(mark))

	return
}

//
// Insert the model.
// Delegated to Tx.Insert().
func (r *Client) Insert(model Model) (err error) {
	tx, err := r.Begin()
	if err != nil {
		return
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.End()
		}
	}()
	err = tx.Insert(model)

	return
}

//
// Update the model.
// Delegated to Tx.Update().
func (r *Client) Update(model Model, predicate ...Predicate) (err error) {
	tx, err := r.Begin()
	if err != nil {
		return
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.End()
		}
	}()
	err = tx.Update(model, predicate...)

	return
}

//
// Delete the model.
// Delegated to Tx.Delete().
func (r *Client) Delete(model Model) (err error) {
	tx, err := r.Begin()
	if err != nil {
		return
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.End()
		}
	}()
	err = tx.Delete(model)

	return
}

//
// Watch model events.
func (r *Client) Watch(model Model, handler EventHandler) (w *Watch, err error) {
	mark := time.Now()
	w, err = r.journal.Watch(model, handler)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			w.End()
			w = nil
		}
	}()
	options := handler.Options()
	var snapshot fb.Iterator
	if options.Snapshot {
		snapshot, err = r.Find(model, ListOptions{Detail: MaxDetail})
		if err != nil {
			return
		}
	} else {
		snapshot = &fb.EmptyIterator{}
	}

	w.Start(snapshot)

	r.log.V(4).Info(
		"watch started.",
		"model",
		Describe(model),
		"options",
		options,
		"duration",
		time.Since(mark))

	return
}

//
// End watch.
func (r *Client) EndWatch(watch *Watch) {
	r.journal.End(watch)
	r.log.V(4).Info(
		"watch ended.",
		"model",
		Describe(watch.Model))
}

//
// Build the data model.
func (r *Client) build() (err error) {
	r.models = append(r.models, &Label{})
	r.dm, err = NewModel(r.models)
	if err != nil {
		return err
	}
	ddls, err := r.dm.DDL()
	if err != nil {
		return err
	}
	session := r.pool.Writer()
	defer session.Return()
	for _, ddl := range ddls {
		_, err := session.db.Exec(ddl)
		if err != nil {
			return liberr.Wrap(
				err,
				"DDL failed.",
				"ddl",
				ddl)
		} else {
			r.log.V(4).Info(
				"DDL succeeded.",
				"ddl",
				ddl)
		}
	}

	return nil
}

//
// Database transaction.
type Tx struct {
	// DB session.
	session *Session
	// Journal.
	journal *Journal
	// Real transaction.
	real *sql.Tx
	// Staged events.
	staged *fb.List
	// Manage labels associated with models.
	labeler Labeler
	// DataModel.
	dm *DataModel
	// Logger.
	log logr.Logger
	// Started timestamp.
	started time.Time
	// Labels associated with the transaction.
	labels []string
	// Ended.
	ended bool
}

//
// Execute SQL.
func (r *Tx) Execute(sql string) (result sql.Result, err error) {
	mark := time.Now()
	result, err = r.real.Exec(sql)
	if err == nil {
		r.log.V(4).Info(
			"execute succeeded.",
			"sql",
			sql,
			"duration",
			time.Since(mark))
	}

	return
}

//
// Get the model.
func (r *Tx) Get(model Model) (err error) {
	mark := time.Now()
	err = Table{r.real}.Get(model)
	if err == nil {
		r.log.V(4).Info(
			"get succeeded.",
			"model",
			Describe(model),
			"duration",
			time.Since(mark))
	}

	return
}

//
// List models.
// The `list` must be: *[]Model.
func (r *Tx) List(list interface{}, options ListOptions) (err error) {
	mark := time.Now()
	err = Table{r.real}.List(list, options)
	if err == nil {
		r.log.V(4).Info(
			"list succeeded.",
			"options",
			options,
			"duration",
			time.Since(mark))
	}

	return
}

//
// List models.
func (r *Tx) Find(model interface{}, options ListOptions) (itr fb.Iterator, err error) {
	mark := time.Now()
	itr, err = Table{r.real}.Find(model, options)
	if err == nil {
		r.log.V(4).Info(
			"iter succeeded",
			"options",
			options,
			"duration",
			time.Since(mark))
	}

	return
}

//
// Count models.
func (r *Tx) Count(model Model, predicate Predicate) (n int64, err error) {
	mark := time.Now()
	n, err = Table{r.real}.Count(model, predicate)
	if err == nil {
		r.log.V(4).Info(
			"count succeeded.",
			"predicate",
			predicate,
			"duration",
			time.Since(mark))
	}
	return
}

//
// Insert the model.
func (r *Tx) Insert(model Model) (err error) {
	mark := time.Now()
	err = Table{r.real}.Insert(model)
	if err != nil {
		return
	}
	event := Event{
		ID:     serial.next(1),
		Labels: r.labels,
		Action: Created,
		Model:  model,
	}
	event.append(r.staged)
	err = r.labeler.Insert(model)
	if err != nil {
		return
	}

	r.log.V(3).Info(
		"insert succeeded.",
		"model",
		Describe(model),
		"duration",
		time.Since(mark))

	return
}

//
// Update the model.
func (r *Tx) Update(model Model, predicate ...Predicate) (err error) {
	mark := time.Now()
	current := model
	current = Clone(model)
	err = Table{r.real}.Get(current)
	if err != nil {
		return
	}
	err = Table{r.real}.Update(model, predicate...)
	if err != nil {
		return
	}
	event := Event{
		ID:      serial.next(1),
		Labels:  r.labels,
		Action:  Updated,
		Model:   current,
		Updated: model,
	}
	event.append(r.staged)
	err = r.labeler.Replace(model)
	if err != nil {
		return
	}

	r.log.V(3).Info(
		"update succeeded.",
		"model",
		Describe(model),
		"duration",
		time.Since(mark))

	return
}

//
// Delete (cascading) of the model.
func (r *Tx) Delete(model Model) (err error) {
	err = Table{r.real}.Get(model)
	if err != nil {
		if errors.As(err, &NotFound) {
			return
		}
		return
	}
	cascaded, err := r.dm.Deleted(r, model)
	if err != nil {
		return
	}
	for {
		m, hasNext := cascaded.Next()
		if hasNext {
			err = r.delete(m.(Model))
			if err != nil {
				return
			}
		} else {
			break
		}
	}
	err = r.delete(model)
	if err != nil {
		return
	}

	return
}

//
// Commit a transaction.
// Staged changes are committed in the DB.
// The transaction is ended and the session returned.
func (r *Tx) Commit() (err error) {
	if r.ended {
		return
	}
	r.ended = true
	defer func() {
		r.session.Return()
		if err == nil {
			r.report()
		}
	}()
	mark := time.Now()
	err = r.real.Commit()
	if err != nil {
		return
	}

	r.log.V(4).Info(
		"tx: committed.",
		"lifespan",
		time.Since(r.started),
		"duration",
		time.Since(mark))

	return
}

//
// End a transaction.
// Staged changes are discarded.
// The session is returned.
func (r *Tx) End() (err error) {
	if r.ended {
		return
	}
	r.ended = true
	defer func() {
		r.session.Return()
		r.staged = fb.NewList()
	}()
	mark := time.Now()
	err = r.real.Rollback()
	if err != nil {
		return
	}

	r.log.V(4).Info(
		"tx: ended.",
		"lifespan",
		time.Since(r.started),
		"duration",
		time.Since(mark))

	return
}

//
// Raw Delete.
// Non-cascading delete of the model.
// The model must be complete (fetched from the DB).
func (r *Tx) delete(model Model) (err error) {
	mark := time.Now()
	err = Table{r.real}.Delete(model)
	if err != nil {
		if errors.As(err, &NotFound) {
			err = nil
		}
		return
	}
	event := Event{
		ID:     serial.next(1),
		Labels: r.labels,
		Action: Deleted,
		Model:  model,
	}
	event.append(r.staged)
	err = r.labeler.Delete(model)
	if err != nil {
		return
	}

	r.log.V(3).Info(
		"delete succeeded.",
		"model",
		Describe(model),
		"duration",
		time.Since(mark))

	return
}

//
// Report staged events to the journal.
func (r *Tx) report() {
	if r.staged.Len() == 0 {
		return
	}
	r.journal.Report(r.staged)
	r.staged = fb.NewList()
}

//
// Labeler.
type Labeler struct {
	// DB transaction.
	tx *sql.Tx
	// Logger.
	log logr.Logger
}

//
// Insert labels for the model into the DB.
func (r *Labeler) Insert(model Model) (err error) {
	table := Table{r.tx}
	kind := table.Name(model)
	if labeled, cast := model.(Labeled); cast {
		for l, v := range labeled.Labels() {
			label := &Label{
				Parent: model.Pk(),
				Kind:   kind,
				Name:   l,
				Value:  v,
			}
			err = table.Insert(label)
			if err != nil {
				return
			}
			r.log.V(2).Info(
				"label inserted.",
				"model",
				Describe(model),
				"kind",
				kind,
				"label",
				l,
				"value",
				v)
		}
	}

	return
}

//
// Delete labels for a model in the DB.
func (r *Labeler) Delete(model Model) (err error) {
	if _, cast := model.(Labeled); !cast {
		return
	}
	list := []Label{}
	table := Table{r.tx}
	err = table.List(
		&list,
		ListOptions{
			Predicate: And(
				Eq("Kind", table.Name(model)),
				Eq("Parent", model.Pk())),
		})
	if err != nil {
		return
	}
	for _, label := range list {
		err = table.Delete(&label)
		if err != nil {
			return
		}
		r.log.V(2).Info(
			"label inserted.",
			"model",
			Describe(model),
			"kind",
			label.Kind,
			"label",
			label.Name,
			"value",
			label.Value)
	}

	return
}

//
// Replace labels.
func (r *Labeler) Replace(model Model) (err error) {
	if _, cast := model.(Labeled); !cast {
		return
	}
	err = r.Delete(model)
	if err != nil {
		return
	}
	err = r.Insert(model)
	if err != nil {
		return
	}

	return
}

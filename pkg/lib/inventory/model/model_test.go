//nolint:errcheck
package model

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/ref"
	"github.com/onsi/gomega"
)

// Adjust default.
func init() {
	DefaultDetail = 5
}

type TestEncoded struct {
	Name string
}

type TestBase struct {
	Parent int    `sql:""`
	Phone  string `sql:""`
}

type PlainObject struct {
	ID   int    `sql:"pk"`
	Name string `sql:""`
	Age  int    `sql:""`
}

func (m *PlainObject) Pk() string {
	return fmt.Sprintf("%d", m.ID)
}

func (m *PlainObject) String() string {
	return fmt.Sprintf(
		"PlainObject: id: %d, name:%s",
		m.ID,
		m.Name)
}

func (m *PlainObject) Equals(other Model) bool {
	return false
}

func (m *PlainObject) Labels() Labels {
	return nil
}

type DetailA struct {
	PK int `sql:"pk"`
	FK int `sql:"fk(PlainObject +cascade +must)"`
}

func (m *DetailA) Pk() string {
	return fmt.Sprintf("%d", m.PK)
}

type DetailB struct {
	PK int `sql:"pk"`
	FK int `sql:"fk(detailA +cascade)"`
}

func (m *DetailB) Pk() string {
	return fmt.Sprintf("%d", m.PK)
}

type DetailC struct {
	PK int `sql:"pk"`
	FK int `sql:"fk(DetailB +cascade)"`
}

func (m *DetailC) Pk() string {
	return fmt.Sprintf("%d", m.PK)
}

type DetailD struct {
	PK int `sql:"pk"`
	FK int `sql:"fk(DetailB +cascade)"`
}

func (m *DetailD) Pk() string {
	return fmt.Sprintf("%d", m.PK)
}

type TestObject struct {
	TestBase
	RowID  int64  `sql:"virtual"`
	PK     string `sql:"pk(id)"`
	ID     int    `sql:"key"`
	Rev    int    `sql:"incremented"`
	Name   string `sql:"index(a)"`
	Age    int    `sql:"index(a)"`
	Int8   int8
	Int16  int16
	Int32  int32
	Bool   bool
	Object TestEncoded `sql:""`
	Slice  []string
	Map    map[string]int
	D1     string `sql:"d1"`
	D2     string `sql:"d2"`
	D3     string `sql:"d3"`
	D4     string `sql:"d4"`
	Phone  string `sql:"-"`
	labels Labels
}

func (m *TestObject) Pk() string {
	return m.PK
}

func (m *TestObject) String() string {
	return fmt.Sprintf(
		"TestObject: id: %d, name:%s",
		m.ID,
		m.Name)
}

func (m *TestObject) Labels() Labels {
	return m.labels
}

// received event.
type TestEvent struct {
	action  uint8
	model   *TestObject
	updated *TestObject
}

type TestHandler struct {
	options WatchOptions
	name    string
	started bool
	parity  bool
	all     []TestEvent
	created []int
	updated []int
	deleted []int
	err     []error
	done    bool
}

func (w *TestHandler) Options() WatchOptions {
	return w.options
}

func (w *TestHandler) Started(uint64) {
	w.started = true
}

func (w *TestHandler) Parity() {
	w.parity = true
}

func (w *TestHandler) Created(e Event) {
	if object, cast := e.Model.(*TestObject); cast {
		w.all = append(w.all, TestEvent{action: e.Action, model: object})
		w.created = append(w.created, object.ID)
	}
}

func (w *TestHandler) Updated(e Event) {
	if object, cast := e.Model.(*TestObject); cast {
		w.all = append(w.all, TestEvent{
			action:  e.Action,
			model:   object,
			updated: e.Updated.(*TestObject),
		})
		w.updated = append(w.updated, object.ID)
	}
}
func (w *TestHandler) Deleted(e Event) {
	if object, cast := e.Model.(*TestObject); cast {
		w.all = append(w.all, TestEvent{action: e.Action, model: object})
		w.deleted = append(w.deleted, object.ID)
	}
}

func (w *TestHandler) Error(err error) {
	w.err = append(w.err, err)
}

func (w *TestHandler) End() {
	w.done = true
}

type MutatingHandler struct {
	options WatchOptions
	DB
	name    string
	started bool
	parity  bool
	created []int
	updated []int
}

func (w *MutatingHandler) Options() WatchOptions {
	return w.options
}

func (w *MutatingHandler) Started(uint64) {
	w.started = true
}

func (w *MutatingHandler) Parity() {
	w.parity = true
}

func (w *MutatingHandler) Created(e Event) {
	tx, _ := w.DB.Begin()
	tx.Get(e.Model)
	e.Model.(*TestObject).Age++
	_ = tx.Update(e.Model)
	_ = tx.Commit()
	w.created = append(w.created, e.Model.(*TestObject).ID)
}

func (w *MutatingHandler) Updated(e Event) {
	label := "echo"
	if e.HasLabel(label) {
		// ignore the echo event.
		return
	}
	tx, _ := w.DB.Begin(label)
	tx.Get(e.Model)
	e.Model.(*TestObject).Age++
	_ = tx.Update(e.Model)
	_ = tx.Commit()
	w.updated = append(w.updated, e.Model.(*TestObject).ID)
}

func (w *MutatingHandler) Deleted(e Event) {
}

func (w *MutatingHandler) Error(err error) {
}

func (w *MutatingHandler) End() {
}

// Used for cascade delete event testing.
type DetailHandler struct {
	StockEventHandler
	deleted []string
}

func (h *DetailHandler) Deleted(e Event) {
	h.deleted = append(
		h.deleted,
		e.Model.Pk())
}

func TestDefinition(t *testing.T) {
	var err error
	g := gomega.NewGomegaWithT(t)
	md, err := Inspect(&TestObject{})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	// ALL
	g.Expect(fieldNames(md.Fields)).To(gomega.Equal(
		[]string{
			"Parent",
			"Phone",
			"RowID",
			"PK",
			"ID",
			"Rev",
			"Name",
			"Age",
			"Int8",
			"Int16",
			"Int32",
			"Bool",
			"Object",
			"Slice",
			"Map",
			"D1",
			"D2",
			"D3",
			"D4",
		}))
	// PK
	g.Expect(md.PkField().Name).To(gomega.Equal("PK"))
	// Natural keys
	g.Expect(fieldNames(md.KeyFields())).To(gomega.Equal([]string{"ID"}))
}

func TestCRUD(t *testing.T) {
	var err error
	g := gomega.NewGomegaWithT(t)
	DB := New(
		"/tmp/test-crud.db",
		&Label{},
		&PlainObject{},
		&TestObject{})
	err = DB.Open(true)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	plainA := &PlainObject{
		ID:   18,
		Name: "Ashley",
		Age:  17,
	}
	err = DB.Insert(plainA)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	plainB := &PlainObject{ID: 18}
	err = DB.Get(plainB)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(plainA.Pk()).To(gomega.Equal(plainB.Pk()))
	g.Expect(plainA.ID).To(gomega.Equal(plainB.ID))
	g.Expect(plainA.Name).To(gomega.Equal(plainB.Name))
	g.Expect(plainA.Age).To(gomega.Equal(plainB.Age))

	objA := &TestObject{
		TestBase: TestBase{
			Parent: 0,
			Phone:  "1234",
		},
		ID:     0,
		Name:   "Elmer",
		Age:    18,
		Int8:   8,
		Int16:  16,
		Int32:  32,
		Bool:   true,
		Object: TestEncoded{Name: "json"},
		Slice:  []string{"hello", "world"},
		Map:    map[string]int{"A": 1, "B": 2},
		labels: Labels{
			"n1": "v1",
			"n2": "v2",
		},
	}
	assertEqual := func(a, b *TestObject) {
		g.Expect(a.PK).To(gomega.Equal(b.PK))
		g.Expect(a.ID).To(gomega.Equal(b.ID))
		g.Expect(a.Rev).To(gomega.Equal(b.Rev))
		g.Expect(a.Name).To(gomega.Equal(b.Name))
		g.Expect(a.Age).To(gomega.Equal(b.Age))
		g.Expect(a.Int8).To(gomega.Equal(b.Int8))
		g.Expect(a.Int16).To(gomega.Equal(b.Int16))
		g.Expect(a.Int32).To(gomega.Equal(b.Int32))
		g.Expect(a.Bool).To(gomega.Equal(b.Bool))
		g.Expect(a.Object).To(gomega.Equal(b.Object))
		g.Expect(a.Slice).To(gomega.Equal(b.Slice))
		g.Expect(a.Map).To(gomega.Equal(b.Map))
		for k, v := range objA.labels {
			l := &Label{
				Kind:   ref.ToKind(a),
				Parent: a.PK,
				Name:   k,
			}
			g.Expect(DB.Get(l)).To(gomega.BeNil())
			g.Expect(v).To(gomega.Equal(l.Value))
		}
	}
	// Insert
	err = DB.Insert(objA)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(objA.Rev).To(gomega.Equal(1))
	objB := &TestObject{ID: objA.ID}
	// Get
	err = DB.Get(objB)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	assertEqual(objA, objB)
	// Update
	objA.Name = "Larry"
	objA.Age = 21
	objA.Bool = false
	err = DB.Update(objA)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(objA.Rev).To(gomega.Equal(2))
	// Update with predicate.
	objA.Name = "Fred"
	objA.Age = 14
	err = DB.Update(objA, Eq("Age", 21))
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(objA.Rev).To(gomega.Equal(3))
	// Get
	objB = &TestObject{ID: objA.ID}
	err = DB.Get(objB)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	assertEqual(objA, objB)
	// Delete
	objA = &TestObject{ID: objA.ID}
	err = DB.Delete(objA)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	// Get (not found)
	objB = &TestObject{ID: objA.ID}
	err = DB.Get(objB)
	g.Expect(errors.Is(err, NotFound)).To(gomega.BeTrue())
}

func TestCascade(t *testing.T) {
	var err error
	g := gomega.NewGomegaWithT(t)
	DB := New(
		"/tmp/test-cascade.db",
		&PlainObject{},
		&DetailC{},
		&DetailD{},
		&DetailB{},
		&DetailA{})
	err = DB.Open(true)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	id := func() int {
		return int(serial.next(-1))
	}

	//
	// Tree A.
	treeA := &PlainObject{
		ID:   0,
		Name: "Emma",
		Age:  18,
	}
	err = DB.Insert(treeA)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	for a := 0; a < 3; a++ {
		detailA := &DetailA{
			PK: id(),
			FK: treeA.ID,
		}
		err = DB.Insert(detailA)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		for b := 0; b < 3; b++ {
			detailB := &DetailB{
				PK: id(),
				FK: detailA.PK,
			}
			err = DB.Insert(detailB)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			for c := 0; c < 3; c++ {
				detailC := &DetailC{
					PK: id(),
					FK: detailB.PK,
				}
				err = DB.Insert(detailC)
				g.Expect(err).ToNot(gomega.HaveOccurred())
			}
		}
	}
	//
	// Tree B
	treeB := &PlainObject{
		ID:   1,
		Name: "Ashley",
		Age:  17,
	}
	err = DB.Insert(treeB)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	for a := 10; a < 13; a++ {
		detailA := &DetailA{
			PK: id(),
			FK: treeB.ID,
		}
		err = DB.Insert(detailA)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		for b := 10; b < 13; b++ {
			detailB := &DetailB{
				PK: id(),
				FK: detailA.PK,
			}
			err = DB.Insert(detailB)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			for c := 10; c < 13; c++ {
				detailC := &DetailC{
					PK: id(),
					FK: detailB.PK,
				}
				err = DB.Insert(detailC)
				g.Expect(err).ToNot(gomega.HaveOccurred())
			}
		}
	}

	// Watch.
	handler := &DetailHandler{}
	_, _ = DB.Watch(&PlainObject{}, handler)
	_, _ = DB.Watch(&DetailA{}, handler)
	_, _ = DB.Watch(&DetailB{}, handler)
	_, _ = DB.Watch(&DetailC{}, handler)

	//
	// Baseline totals.
	n, _ := DB.Count(&DetailA{}, nil)
	g.Expect(n).To(gomega.Equal(int64(6)))
	n, _ = DB.Count(&DetailB{}, nil)
	g.Expect(n).To(gomega.Equal(int64(18)))
	n, _ = DB.Count(&DetailC{}, nil)
	g.Expect(n).To(gomega.Equal(int64(54)))

	// Delete tree A.
	err = DB.Delete(treeA)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	//
	// Tree A gone.
	n, _ = DB.Count(&DetailA{}, nil)
	g.Expect(n).To(gomega.Equal(int64(3)))
	n, _ = DB.Count(&DetailB{}, nil)
	g.Expect(n).To(gomega.Equal(int64(9)))
	n, _ = DB.Count(&DetailC{}, nil)
	g.Expect(n).To(gomega.Equal(int64(27)))

	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 10)
		if len(handler.deleted) != 40 {
			continue
		} else {
			break
		}
	}
	g.Expect(len(handler.deleted)).To(gomega.Equal(40))

}

func TestTransactions(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	DB := New(
		"/tmp/test-transactions.db",
		&TestObject{})
	err := DB.Open(true)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	for i := 0; i < 10; i++ {
		// Begin
		tx, err := DB.Begin()
		defer tx.End()
		g.Expect(err).ToNot(gomega.HaveOccurred())
		object := &TestObject{
			ID:   i,
			Name: "Elmer",
		}
		err = tx.Insert(object)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		// Get (not found)
		object = &TestObject{ID: object.ID}
		err = DB.Get(object)
		g.Expect(errors.Is(err, NotFound)).To(gomega.BeTrue())
		tx.Commit()
		// Get (found)
		object = &TestObject{ID: object.ID}
		err = DB.Get(object)
		g.Expect(err).ToNot(gomega.HaveOccurred())
	}
}

func TestWithTxSucceeded(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	DB := New(
		"/tmp/test-withtx-succeeded.db",
		&TestObject{})
	err := DB.Open(true)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	labels := []string{"A", "B"}
	n := 10
	//
	// Insert in TX.
	insert := func(tx *Tx) (err error) {
		g.Expect(tx.labels).To(gomega.Equal(labels))
		for i := 0; i < n; i++ {
			object := &TestObject{
				ID:   i,
				Name: "Elmer",
			}
			err = tx.Insert(object)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			object = &TestObject{ID: object.ID}
		}
		return
	}
	//
	// Test committed.
	err = DB.With(insert, labels...)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	for i := 0; i < n; i++ {
		object := &TestObject{ID: i}
		err = DB.Get(object)
		g.Expect(err).ToNot(gomega.HaveOccurred())
	}
}

func TestWithTxFailed(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	DB := New(
		"/tmp/test-withtx-failed.db",
		&TestObject{})
	err := DB.Open(true)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	//
	// Insert in TX with duplicate key error.
	fakeErr := liberr.New("Faked")
	insert := func(tx *Tx) (err error) {
		object := &TestObject{}
		err = tx.Insert(object)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		err = fakeErr
		return
	}
	//
	// Test not committed.
	err = DB.With(insert)
	g.Expect(errors.Is(err, fakeErr)).To(gomega.BeTrue())
	n, err := DB.Count(&TestObject{}, nil)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(n).To(gomega.Equal(int64(0)))
}

func TestList(t *testing.T) {
	var err error
	g := gomega.NewGomegaWithT(t)
	DB := New(
		"/tmp/test-list.db",
		&TestObject{})
	err = DB.Open(true)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	N := 10
	for i := 0; i < N; i++ {
		object := &TestObject{
			ID:     i,
			Name:   "Elmer",
			Age:    18,
			Int8:   8,
			Int16:  16,
			Int32:  32,
			Bool:   true,
			Object: TestEncoded{Name: "json"},
			Slice:  []string{"hello", "world"},
			Map:    map[string]int{"A": 1, "B": 2},
			D1:     "d-1",
			D2:     "d-2",
			D3:     "d-3",
			D4:     "d-4",
			labels: Labels{
				"id": fmt.Sprintf("v%d", i),
			},
		}
		err = DB.Insert(object)
		g.Expect(err).ToNot(gomega.HaveOccurred())
	}
	// List detail level=0
	list := []TestObject{}
	err = DB.List(&list, ListOptions{})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(10))
	g.Expect(list[0].Name).To(gomega.Equal(""))
	g.Expect(list[0].Slice).To(gomega.BeNil())
	g.Expect(list[0].D1).To(gomega.Equal(""))
	g.Expect(list[0].D2).To(gomega.Equal(""))
	g.Expect(list[0].D3).To(gomega.Equal(""))
	g.Expect(list[0].D4).To(gomega.Equal(""))
	// List detail level=1
	list = []TestObject{}
	err = DB.List(&list, ListOptions{Detail: 1})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(10))
	g.Expect(list[0].Name).To(gomega.Equal(""))
	g.Expect(list[0].Slice).To(gomega.BeNil())
	g.Expect(list[0].D1).To(gomega.Equal("d-1"))
	g.Expect(list[0].D2).To(gomega.Equal(""))
	g.Expect(list[0].D3).To(gomega.Equal(""))
	g.Expect(list[0].D4).To(gomega.Equal(""))
	// List detail level=2
	list = []TestObject{}
	err = DB.List(&list, ListOptions{Detail: 2})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(10))
	g.Expect(list[0].Name).To(gomega.Equal(""))
	g.Expect(list[0].Slice).To(gomega.BeNil())
	g.Expect(list[0].D1).To(gomega.Equal("d-1"))
	g.Expect(list[0].D2).To(gomega.Equal("d-2"))
	g.Expect(list[0].D3).To(gomega.Equal(""))
	g.Expect(list[0].D4).To(gomega.Equal(""))
	// List detail level=3
	list = []TestObject{}
	err = DB.List(&list, ListOptions{Detail: 3})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(10))
	g.Expect(list[0].Name).To(gomega.Equal(""))
	g.Expect(list[0].Slice).To(gomega.BeNil())
	g.Expect(list[0].D1).To(gomega.Equal("d-1"))
	g.Expect(list[0].D2).To(gomega.Equal("d-2"))
	g.Expect(list[0].D3).To(gomega.Equal("d-3"))
	g.Expect(list[0].D4).To(gomega.Equal(""))
	// List detail level=4
	list = []TestObject{}
	err = DB.List(&list, ListOptions{Detail: 4})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(10))
	g.Expect(list[0].Name).To(gomega.Equal(""))
	g.Expect(list[0].Slice).To(gomega.BeNil())
	g.Expect(list[0].D1).To(gomega.Equal("d-1"))
	g.Expect(list[0].D2).To(gomega.Equal("d-2"))
	g.Expect(list[0].D3).To(gomega.Equal("d-3"))
	g.Expect(list[0].D4).To(gomega.Equal("d-4"))
	// List detail level=10
	list = []TestObject{}
	err = DB.List(&list, ListOptions{Detail: MaxDetail})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(10))
	g.Expect(list[0].Name).To(gomega.Equal("Elmer"))
	g.Expect(len(list[0].Slice)).To(gomega.Equal(2))
	g.Expect(list[0].D1).To(gomega.Equal("d-1"))
	g.Expect(list[0].D2).To(gomega.Equal("d-2"))
	g.Expect(list[0].D3).To(gomega.Equal("d-3"))
	g.Expect(list[0].D4).To(gomega.Equal("d-4"))
	// List = (single).
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Predicate: Eq("ID", 0),
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(1))
	g.Expect(list[0].ID).To(gomega.Equal(0))
	// List = (multiple).
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Predicate: Eq("ID", []int{2, 4}),
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(2))
	g.Expect(list[0].ID).To(gomega.Equal(2))
	g.Expect(list[1].ID).To(gomega.Equal(4))
	// List != AND
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Detail: 2,
			Predicate: And( // Even only.
				Neq("ID", 1),
				Neq("ID", 3),
				Neq("ID", 5),
				Neq("ID", 7),
				Neq("ID", 9)),
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(5))
	g.Expect(list[0].ID).To(gomega.Equal(0))
	g.Expect(list[1].ID).To(gomega.Equal(2))
	g.Expect(list[2].ID).To(gomega.Equal(4))
	g.Expect(list[3].ID).To(gomega.Equal(6))
	g.Expect(list[4].ID).To(gomega.Equal(8))
	// List OR =.
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Predicate: Or(
				Eq("ID", 0),
				Eq("ID", 6)),
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(2))
	g.Expect(list[0].ID).To(gomega.Equal(0))
	g.Expect(list[1].ID).To(gomega.Equal(6))
	// List < (lt).
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Predicate: Lt("ID", 2),
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(2))
	g.Expect(list[0].ID).To(gomega.Equal(0))
	g.Expect(list[1].ID).To(gomega.Equal(1))
	// List > (gt).
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Predicate: Gt("ID", 7),
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(2))
	g.Expect(list[0].ID).To(gomega.Equal(8))
	g.Expect(list[1].ID).To(gomega.Equal(9))
	// List > (gt) virtual.
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Predicate: Gt("RowID", N/2),
			Detail:    MaxDetail,
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(N / 2))
	g.Expect(list[0].RowID).To(gomega.Equal(int64(N/2) + 1))
	// List (Eq) Field values.
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Predicate: Eq("RowID", Field{Name: "int8"}),
			Detail:    MaxDetail,
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(1))
	g.Expect(list[0].RowID).To(gomega.Equal(int64(8)))
	// List (nEq) Field values.
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Predicate: Neq("RowID", Field{Name: "int8"}),
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(N - 1))
	// List (Lt) Field values.
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Predicate: Lt("int8", Field{Name: "int16"}),
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(N))
	// List (Gt) Field values.
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Predicate: Gt("RowID", Field{Name: "int8"}),
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(2))
	// By label.
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Sort: []int{2},
			Predicate: Or(
				Match(Labels{"id": "v4"}),
				Eq("ID", 8)),
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(list)).To(gomega.Equal(2))
	g.Expect(list[0].ID).To(gomega.Equal(4))
	g.Expect(list[1].ID).To(gomega.Equal(8))
	// Test count all.
	count, err := DB.Count(&TestObject{}, nil)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(count).To(gomega.Equal(int64(10)))
	// Test count with predicate.
	count, err = DB.Count(&TestObject{}, Gt("ID", 0))
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(count).To(gomega.Equal(int64(9)))
}

func TestFind(t *testing.T) {
	var err error
	g := gomega.NewGomegaWithT(t)
	DB := New(
		"/tmp/test-iter.db",
		&TestObject{})
	err = DB.Open(true)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	N := 10
	for i := 0; i < N; i++ {
		object := &TestObject{
			ID:     i,
			Name:   "Elmer",
			Age:    18,
			Int8:   8,
			Int16:  16,
			Int32:  32,
			Bool:   true,
			Object: TestEncoded{Name: "json"},
			Slice:  []string{"hello", "world"},
			Map:    map[string]int{"A": 1, "B": 2},
			D4:     "d-4",
			labels: Labels{
				"id": fmt.Sprintf("v%d", i),
			},
		}
		err = DB.Insert(object)
		g.Expect(err).ToNot(gomega.HaveOccurred())
	}
	// List all; detail level=0
	itr, err := DB.Find(
		&TestObject{},
		ListOptions{})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(itr.Len()).To(gomega.Equal(10))
	var list []TestObject
	for {
		object := TestObject{}
		if itr.NextWith(&object) {
			g.Expect(err).ToNot(gomega.HaveOccurred())
			list = append(list, object)
		} else {
			break
		}
	}
	g.Expect(len(list)).To(gomega.Equal(10))
	// List all; detail level=0
	itr, err = DB.Find(
		&TestObject{},
		ListOptions{})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	for object, hasNext := itr.Next(); hasNext; object, hasNext = itr.Next() {
		g.Expect(err).ToNot(gomega.HaveOccurred())
		_, cast := object.(Model)
		g.Expect(cast).To(gomega.BeTrue())
	}
}

func TestWatch(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	DB := New("/tmp/test-watch.db", &TestObject{})
	err := DB.Open(true)
	defer func() {
		_ = DB.Close(false)
	}()
	g.Expect(err).ToNot(gomega.HaveOccurred())
	// Handler A
	handlerA := &TestHandler{
		options: WatchOptions{Snapshot: true},
		name:    "A",
	}
	watchA, err := DB.Watch(&TestObject{}, handlerA)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(watchA).ToNot(gomega.BeNil())
	g.Expect(watchA.Alive()).To(gomega.BeTrue())
	N := 10
	// Insert
	for i := 0; i < N; i++ {
		object := &TestObject{
			ID:   i,
			Name: "Elmer",
		}
		err = DB.Insert(object)
		g.Expect(err).ToNot(gomega.HaveOccurred())
	}
	// Handler B
	handlerB := &TestHandler{
		options: WatchOptions{Snapshot: true},
		name:    "B",
	}
	watchB, err := DB.Watch(&TestObject{}, handlerB)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(watchB).ToNot(gomega.BeNil())
	// Update
	for i := 0; i < N; i++ {
		object := &TestObject{ID: i}
		_ = DB.Get(object)
		object.Name = "Fudd"
		object.Age = 18
		object.Int8 = 8
		object.Int16 = 16
		object.Int32 = 32
		object.Bool = true
		object.Object = TestEncoded{Name: "json"}
		object.Slice = []string{"hello", "world"}
		object.Map = map[string]int{"A": 1, "B": 2}
		object.D4 = "d-4"
		object.labels = Labels{
			"id": fmt.Sprintf("v%d", i),
		}
		err = DB.Update(object)
		g.Expect(err).ToNot(gomega.HaveOccurred())
	}
	// Handler C
	handlerC := &TestHandler{
		options: WatchOptions{Snapshot: true},
		name:    "C",
	}
	watchC, err := DB.Watch(&TestObject{}, handlerC)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(watchC).ToNot(gomega.BeNil())
	// Handler D (no snapshot)
	handlerD := &TestHandler{name: "D"}
	watchD, err := DB.Watch(&TestObject{}, handlerD)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(watchC).ToNot(gomega.BeNil())
	// Delete
	for i := 0; i < N; i++ {
		object := &TestObject{
			ID: i,
		}
		err = DB.Delete(object)
		g.Expect(err).ToNot(gomega.HaveOccurred())
	}
	for i := 0; i < N; i++ {
		time.Sleep(time.Millisecond * 10)
		if len(handlerA.created) != N ||
			len(handlerA.updated) != N ||
			len(handlerA.created) != N ||
			len(handlerB.created) != N ||
			len(handlerB.updated) != N ||
			len(handlerB.created) != N ||
			len(handlerC.created) != N ||
			len(handlerC.created) != N {
			continue
		} else {
			break
		}
	}
	g.Expect(handlerA.started).To(gomega.BeTrue())
	g.Expect(handlerB.started).To(gomega.BeTrue())
	g.Expect(handlerC.started).To(gomega.BeTrue())
	g.Expect(handlerD.started).To(gomega.BeTrue())
	g.Expect(handlerA.parity).To(gomega.BeTrue())
	g.Expect(handlerB.parity).To(gomega.BeTrue())
	g.Expect(handlerC.parity).To(gomega.BeTrue())
	g.Expect(handlerD.parity).To(gomega.BeTrue())
	//
	// The scenario is:
	// 1. handler A created
	// 2. (N) models created. handler A should get (N) CREATE events.
	// 3. handler B created.  handler B should get (N) CREATE events.
	// 4. (N) models updated. handler A & B should get (N) UPDATE events.
	// 5. Handler C created.  handler C should get (N) CREATE events.
	// 6. (N) models deleted. handler A,B,C should get (N) DELETE events.
	all := []TestEvent{}
	deleted := []TestEvent{}
	for _, action := range []uint8{Created, Updated, Deleted} {
		for i := 0; i < N; i++ {
			switch action {
			case Created:
				// Created.
			case Updated:
				// Updated.
			case Deleted:
				deleted = append(
					deleted,
					TestEvent{
						action: action,
						model:  &TestObject{ID: i},
					})
			}
			all = append(
				all,
				TestEvent{
					action: action,
					model:  &TestObject{ID: i},
				})
		}
	}
	g.Expect(func() (eq bool) {
		h := handlerA
		if len(all) != len(h.all) {
			return
		}
		for i := 0; i < len(all); i++ {
			if h.all[i].action == Updated {
				if h.all[i].updated.Rev != h.all[i].model.Rev+1 ||
					h.all[i].updated.Name != "Fudd" {
					return
				}
			}
			if all[i].action != h.all[i].action ||
				all[i].model.ID != h.all[i].model.ID {
				return
			}
		}
		return true
	}()).To(gomega.BeTrue())
	g.Expect(func() (eq bool) {
		h := handlerB
		if len(all) != len(h.all) {
			return
		}
		for i := 0; i < len(all); i++ {
			if all[i].action != h.all[i].action ||
				all[i].model.ID != h.all[i].model.ID {
				return
			}
		}
		return true
	}()).To(gomega.BeTrue())
	all = []TestEvent{}
	for _, action := range []uint8{Created, Deleted} {
		for i := 0; i < N; i++ {
			all = append(
				all,
				TestEvent{
					action: action,
					model:  &TestObject{ID: i},
				})
		}
	}
	g.Expect(func() (eq bool) {
		h := handlerC
		if len(all) != len(h.all) {
			return
		}
		for i := 0; i < len(all); i++ {
			if all[i].action != h.all[i].action ||
				all[i].model.ID != h.all[i].model.ID {
				return
			}
		}
		return true
	}()).To(gomega.BeTrue())
	g.Expect(func() (eq bool) {
		h := handlerD
		if len(deleted) != len(h.deleted) {
			return
		}
		for i := 0; i < len(deleted); i++ {
			if deleted[i].model.ID != h.deleted[i] {
				return
			}
		}
		return true
	}()).To(gomega.BeTrue())

	//
	// Test watch end.
	watchA.End()
	watchB.End()
	watchC.End()
	watchD.End()
	ended := false
	for i := 0; i < 10; i++ {
		if watchA.started || watchB.started || watchC.started || watchD.started {
			time.Sleep(50 * time.Millisecond)
		} else {
			ended = true
			break
		}
	}
	g.Expect(len(watchA.journal.watches)).To(gomega.Equal(0))
	g.Expect(ended).To(gomega.BeTrue())
	g.Expect(handlerA.done).To(gomega.BeTrue())
	g.Expect(handlerB.done).To(gomega.BeTrue())
	g.Expect(handlerC.done).To(gomega.BeTrue())
	g.Expect(handlerD.done).To(gomega.BeTrue())
}

//nolint:errcheck
func TestCloseDB(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	DB := New("/tmp/test-close-db.db", &TestObject{})
	err := DB.Open(true)
	defer func() {
		_ = DB.Close(false)
	}()
	g.Expect(err).ToNot(gomega.HaveOccurred())
	handler := &TestHandler{
		options: WatchOptions{Snapshot: true},
		name:    "A",
	}
	watch, err := DB.Watch(&TestObject{}, handler)
	if err != nil {
		t.Fatalf("Failed to create watch: %v", err)
	}
	for i := 0; i < 10; i++ {
		if !watch.started {
			time.Sleep(50 * time.Millisecond)
		} else {
			break
		}
	}
	g.Expect(handler.started).To(gomega.BeTrue())
	g.Expect(handler.done).To(gomega.BeFalse())
	_ = DB.Close(true)
	for _, session := range DB.(*Client).pool.sessions {
		g.Expect(session.closed).To(gomega.BeTrue())
	}
	for i := 0; i < 100; i++ {
		if !watch.done {
			time.Sleep(50 * time.Millisecond)
		} else {
			break
		}
	}

	g.Expect(handler.done).To(gomega.BeTrue())
}

func TestMutatingWatch(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	DB := New("/tmp/test-mutating-watch.db", &TestObject{})
	err := DB.Open(true)

	g.Expect(err).ToNot(gomega.HaveOccurred())

	// Handler A
	handlerA := &MutatingHandler{
		options: WatchOptions{Snapshot: true},
		name:    "A",
		DB:      DB,
	}
	watchA, err := DB.Watch(&TestObject{}, handlerA)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(watchA).ToNot(gomega.BeNil())
	// Handler B
	handlerB := &MutatingHandler{
		options: WatchOptions{Snapshot: true},
		name:    "B",
		DB:      DB,
	}
	watchB, err := DB.Watch(&TestObject{}, handlerB)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(watchB).ToNot(gomega.BeNil())
	N := 10
	// Insert
	for i := 0; i < N; i++ {
		object := &TestObject{
			ID:   i,
			Name: "Elmer",
		}
		err = DB.Insert(object)
		g.Expect(err).ToNot(gomega.HaveOccurred())
	}

	for {
		time.Sleep(time.Millisecond * 10)
		if len(handlerA.updated) == N*2 {
			break
		}
	}
}

func TestExecute(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	type Person struct {
		ID   int    `sql:"pk"`
		Name string `sql:""`
	}
	DB := New("/tmp/test-execute.db", &Person{})
	err := DB.Open(true)
	defer func() {
		_ = DB.Close(false)
	}()

	g.Expect(err).ToNot(gomega.HaveOccurred())

	result, err := DB.Execute(
		"INSERT INTO Person (id, name) values (0, 'john');")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(result.RowsAffected()).To(gomega.Equal(int64(1)))
}

func TestSession(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	DB := New("/tmp/test-session.db", &TestObject{})
	DB.Open(true)
	defer func() {
		_ = DB.Close(false)
	}()

	pool := DB.(*Client).pool

	w := pool.Writer()
	g.Expect(w.id).To(gomega.Equal(0))
	for n := 1; n < 11; n++ {
		r := pool.Reader()
		g.Expect(r.id).To(gomega.Equal(n))
	}
}

func TestDbLocked(t *testing.T) {
	t.Skip("Skipping DB locked test")
	g := gomega.NewGomegaWithT(t)
	DB := New("/tmp/test-db-locked.db", &TestObject{}, &PlainObject{})
	err := DB.Open(true)
	defer func() {
		_ = DB.Close(false)
	}()
	errChan := make(chan error)
	endChan := make(chan int)
	go func() {
		tx, _ := DB.Begin()
		defer func() {
			errChan <- err
			close(errChan)
			_ = tx.End()
		}()
		for i := 0; i < 20000; i++ {
			object := &TestObject{
				ID:   i,
				Name: "Elmer",
			}
			err = tx.Insert(object)
			errChan <- err
			if err != nil {
				return
			}
		}
		err = tx.Commit()
	}()
	go func() {
		defer close(endChan)
		n := int64(0)
		for err = range errChan {
			g.Expect(err).ToNot(gomega.HaveOccurred())
			n, err = DB.Count(&TestObject{}, nil)
			g.Expect(err).ToNot(gomega.HaveOccurred())
		}
		fmt.Printf("Count:%d", n)
	}()
}

func TestConcurrency(t *testing.T) {
	t.Skip("Skipping Concurrency test")
	var err error

	DB := New("/tmp/test-concurrency.db", &TestObject{})
	DB.Open(true)
	defer func() {
		_ = DB.Close(false)
	}()

	N := 1000

	direct := func(done chan int) {
		for i := 0; i < N; i++ {
			m := &TestObject{
				ID:   i,
				Name: "direct",
			}
			err := DB.Insert(m)
			if err != nil {
				panic(err)
			}
			fmt.Printf("direct|%d\n", i)
			time.Sleep(time.Millisecond * 10)
		}
		done <- 0
	}
	read := func(done chan int) {
		time.Sleep(time.Second)
		for i := 0; i < N; i++ {
			m := &TestObject{
				ID:   i,
				Name: "direct",
			}
			go func() {
				err := DB.Get(m)
				if err != nil {
					if errors.Is(err, NotFound) {
						fmt.Printf("read|%d _____%s\n", i, err)
					} else {
						panic(err)
					}
				}
				fmt.Printf("read|%d\n", i)
			}()
			time.Sleep(time.Millisecond * 100)
		}
		done <- 0
	}
	del := func(done chan int) {
		time.Sleep(time.Second * 3)
		for i := 0; i < N/2; i++ {
			m := &TestObject{
				ID: i,
			}
			go func() {
				err := DB.Delete(m)
				if err != nil {
					if errors.Is(err, NotFound) {
						fmt.Printf("del|%d _____%s\n", i, err)
					} else {
						panic(err)
					}
				}
				fmt.Printf("del|%d\n", i)
			}()
			time.Sleep(time.Millisecond * 300)
		}
		done <- 0
	}
	update := func(done chan int) {
		for i := 0; i < N; i++ {
			m := &TestObject{
				ID:   i,
				Name: "direct",
			}
			go func() {
				err := DB.Update(m)
				if err != nil {
					if errors.Is(err, NotFound) {
						fmt.Printf("update|%d _____%s\n", i, err)
					} else {
						panic(err)
					}
				}
				fmt.Printf("update|%d\n", i)
			}()
			time.Sleep(time.Millisecond * 20)
		}
		done <- 0
	}
	transaction := func(done chan int) {
		time.Sleep(time.Millisecond * 100)
		var tx *Tx
		defer func() {
			if tx != nil {
				err := tx.Commit()
				if err != nil {
					panic(err)
				}
			}
		}()
		threshold := float64(10)
		for i := N; i < N*2; i++ {
			if tx == nil {
				tx, err = DB.Begin()
				if err != nil {
					panic(err)
				}
			}
			m := &TestObject{
				ID:   i,
				Name: "transaction",
			}
			err = tx.Insert(m)
			if err != nil {
				panic(err)
			}
			//time.Sleep(time.Second*3)
			if math.Mod(float64(i), threshold) == 0 {
				err = tx.Commit()
				if err != nil {
					panic(err)
				}
				tx = nil
				fmt.Printf("commit|%d\n", i)
			}
			fmt.Printf("transaction|%d\n", i)
			time.Sleep(time.Millisecond * 100)
		}
		done <- 0
	}

	mark := time.Now()

	done := make(chan int)
	fnList := []func(chan int){
		direct,
		transaction,
		read,
		update,
		del,
	}
	for _, fn := range fnList {
		go fn(done)
	}
	for range fnList {
		<-done
	}

	fmt.Println(time.Since(mark))
}

func TestDefinitions(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	list := Definitions{}
	// Push
	list.Push(&Definition{Kind: "0"})
	list.Push(&Definition{Kind: "1"})
	list.Push(&Definition{Kind: "2"})
	g.Expect(list).To(gomega.Equal(
		Definitions{
			&Definition{Kind: "0"},
			&Definition{Kind: "1"},
			&Definition{Kind: "2"},
		}))
	// Pop
	d2 := list.Pop()
	g.Expect(d2.Kind).To(gomega.Equal("2"))
	g.Expect(list).To(gomega.Equal(
		Definitions{
			&Definition{Kind: "0"},
			&Definition{Kind: "1"},
		}))
	// Append
	list.Push(&Definition{Kind: "2"})
	g.Expect(list).To(gomega.Equal(
		Definitions{
			&Definition{Kind: "0"},
			&Definition{Kind: "1"},
			&Definition{Kind: "2"},
		}))
	// Delete
	list.Delete(1)
	g.Expect(list).To(gomega.Equal(
		Definitions{
			&Definition{Kind: "0"},
			&Definition{Kind: "2"},
		}))
	list = Definitions{
		&Definition{Kind: "0"},
		&Definition{Kind: "1"},
		&Definition{Kind: "2"},
	}
	list.Delete(0)
	g.Expect(list).To(gomega.Equal(
		Definitions{
			&Definition{Kind: "1"},
			&Definition{Kind: "2"},
		}))
	list = Definitions{
		&Definition{Kind: "0"},
		&Definition{Kind: "1"},
		&Definition{Kind: "2"},
	}
	list.Delete(2)
	g.Expect(list).To(gomega.Equal(
		Definitions{
			&Definition{Kind: "0"},
			&Definition{Kind: "1"},
		}))
	// Head and Top.
	list = Definitions{
		&Definition{Kind: "0"},
		&Definition{Kind: "1"},
		&Definition{Kind: "2"},
	}
	g.Expect(list.Top().Kind).To(gomega.Equal("2"))
	g.Expect(list.Head(false).Kind).To(gomega.Equal("0"))
	g.Expect(list.Head(true).Kind).To(gomega.Equal("0"))
	g.Expect(list).To(gomega.Equal(
		Definitions{
			&Definition{Kind: "1"},
			&Definition{Kind: "2"},
		}))
}

func fieldNames(fields []*Field) (names []string) {
	for _, f := range fields {
		names = append(names, f.Name)
	}

	return
}

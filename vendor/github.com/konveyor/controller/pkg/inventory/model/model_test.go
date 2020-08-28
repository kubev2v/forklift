package model

import (
	"errors"
	"fmt"
	"github.com/konveyor/controller/pkg/ref"
	"github.com/onsi/gomega"
	"math"
	"testing"
	"time"
)

type TestObject struct {
	PK     string `sql:"pk,generated(id)"`
	ID     int    `sql:"key"`
	Name   string `sql:""`
	Age    int    `sql:""`
	Int8   int8   `sql:""`
	Int16  int16  `sql:""`
	Int32  int32  `sql:""`
	Bool   bool   `sql:""`
	labels Labels
}

func (m *TestObject) Pk() string {
	return fmt.Sprintf("%s", m.PK)
}

func (m *TestObject) String() string {
	return fmt.Sprintf(
		"TestObject: id: %d, name:%s",
		m.ID,
		m.Name)
}

func (m *TestObject) Equals(other Model) bool {
	return false
}

func (m *TestObject) Labels() Labels {
	return m.labels
}

type TestHandler struct {
	name    string
	created []int
	updated []int
	deleted []int
	err     []error
	done    bool
}

func (w *TestHandler) Created(e Event) {
	if object, cast := e.Model.(*TestObject); cast {
		w.created = append(w.created, object.ID)
	}
}

func (w *TestHandler) Updated(e Event) {
	if object, cast := e.Model.(*TestObject); cast {
		w.updated = append(w.updated, object.ID)
	}
}
func (w *TestHandler) Deleted(e Event) {
	if object, cast := e.Model.(*TestObject); cast {
		w.deleted = append(w.deleted, object.ID)
	}
}

func (w *TestHandler) Error(err error) {
	w.err = append(w.err, err)
}

func (w *TestHandler) End() {
}

func TestCRUD(t *testing.T) {
	var err error
	g := gomega.NewGomegaWithT(t)
	DB := New(
		"/tmp/test.db",
		&Label{},
		&TestObject{})
	err = DB.Open(true)
	g.Expect(err).To(gomega.BeNil())
	objA := &TestObject{
		ID:    0,
		Name:  "Elmer",
		Age:   18,
		Int8:  8,
		Int16: 16,
		Int32: 32,
		Bool:  true,
		labels: Labels{
			"n1": "v1",
			"n2": "v2",
		},
	}
	assertEqual := func(a, b *TestObject) {
		g.Expect(a.PK).To(gomega.Equal(b.PK))
		g.Expect(a.ID).To(gomega.Equal(b.ID))
		g.Expect(a.Name).To(gomega.Equal(b.Name))
		g.Expect(a.Age).To(gomega.Equal(b.Age))
		g.Expect(a.Int8).To(gomega.Equal(b.Int8))
		g.Expect(a.Int16).To(gomega.Equal(b.Int16))
		g.Expect(a.Int32).To(gomega.Equal(b.Int32))
		g.Expect(a.Bool).To(gomega.Equal(b.Bool))
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
	g.Expect(err).To(gomega.BeNil())
	objB := &TestObject{ID: objA.ID}
	// Get
	err = DB.Get(objB)
	g.Expect(err).To(gomega.BeNil())
	assertEqual(objA, objB)
	// Update
	objA.Name = "Larry"
	objA.Age = 21
	objA.Bool = false
	err = DB.Update(objA)
	g.Expect(err).To(gomega.BeNil())
	// Get
	objB = &TestObject{ID: objA.ID}
	err = DB.Get(objB)
	g.Expect(err).To(gomega.BeNil())
	assertEqual(objA, objB)
	// Delete
	objA = &TestObject{ID: objA.ID}
	err = DB.Delete(objA)
	g.Expect(err).To(gomega.BeNil())
	// Get (not found)
	objB = &TestObject{ID: objA.ID}
	err = DB.Get(objB)
	g.Expect(errors.Is(err, NotFound)).To(gomega.BeTrue())
}

func TestTransactions(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	DB := New(
		"/tmp/test.db",
		&Label{},
		&TestObject{})
	err := DB.Open(true)
	g.Expect(err).To(gomega.BeNil())
	// Begin
	tx, err := DB.Begin()
	defer tx.End()
	g.Expect(err).To(gomega.BeNil())
	g.Expect(tx.ref).To(gomega.Equal(DB.(*Client).tx))
	object := &TestObject{
		ID:   0,
		Name: "Elmer",
	}
	err = DB.Insert(object)
	g.Expect(err).To(gomega.BeNil())
	// Get (not found)
	object = &TestObject{ID: object.ID}
	err = DB.Get(object)
	g.Expect(errors.Is(err, NotFound)).To(gomega.BeTrue())
	tx.Commit()
	// Get (found)
	object = &TestObject{ID: object.ID}
	err = DB.Get(object)
	g.Expect(err).To(gomega.BeNil())
}

func TestGetForUpdate(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	DB := New(
		"/tmp/test.db",
		&Label{},
		&TestObject{})
	err := DB.Open(true)
	g.Expect(err).To(gomega.BeNil())
	// Insert
	object := &TestObject{
		ID:   0,
		Name: "Elmer",
	}
	err = DB.Insert(object)
	g.Expect(err).To(gomega.BeNil())
	tx, err := DB.GetForUpdate(object)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(tx.ref).To(gomega.Equal(DB.(*Client).tx))
	tx.Commit()
}

func TestList(t *testing.T) {
	var err error
	g := gomega.NewGomegaWithT(t)
	DB := New(
		"/tmp/test.db",
		&Label{},
		&TestObject{})
	err = DB.Open(true)
	g.Expect(err).To(gomega.BeNil())
	for i := 0; i < 10; i++ {
		object := &TestObject{
			ID: i,
			labels: Labels{
				"id": fmt.Sprintf("v%d", i),
			},
		}
		err = DB.Insert(object)
		g.Expect(err).To(gomega.BeNil())
	}
	// List all.
	list := []TestObject{}
	err = DB.List(&list, ListOptions{})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(list)).To(gomega.Equal(10))
	// List = (single).
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Predicate: Eq("ID", 0),
		})
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(list)).To(gomega.Equal(1))
	g.Expect(list[0].ID).To(gomega.Equal(0))
	// List != AND
	list = []TestObject{}
	err = DB.List(
		&list,
		ListOptions{
			Predicate: And( // Even only.
				Neq("ID", 1),
				Neq("ID", 3),
				Neq("ID", 5),
				Neq("ID", 7),
				Neq("ID", 9)),
		})
	g.Expect(err).To(gomega.BeNil())
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
	g.Expect(err).To(gomega.BeNil())
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
	g.Expect(err).To(gomega.BeNil())
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
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(list)).To(gomega.Equal(2))
	g.Expect(list[0].ID).To(gomega.Equal(8))
	g.Expect(list[1].ID).To(gomega.Equal(9))
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
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(list)).To(gomega.Equal(2))
	g.Expect(list[0].ID).To(gomega.Equal(4))
	g.Expect(list[1].ID).To(gomega.Equal(8))
	// Test count all.
	count, err := DB.Count(&TestObject{}, nil)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(count).To(gomega.Equal(int64(10)))
	// Test count with predicate.
	count, err = DB.Count(&TestObject{}, Gt("ID", 0))
	g.Expect(err).To(gomega.BeNil())
	g.Expect(count).To(gomega.Equal(int64(9)))
}

func TestWatch(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	DB := New(
		"/tmp/test.db",
		&Label{},
		&TestObject{})
	err := DB.Open(true)
	g.Expect(err).To(gomega.BeNil())
	DB.Journal().Enable()
	// Handler A
	handlerA := &TestHandler{name: "A"}
	watchA, err := DB.Watch(&TestObject{}, handlerA)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(watchA).ToNot(gomega.BeNil())
	N := 10
	// Insert
	for i := 0; i < N; i++ {
		object := &TestObject{
			ID:   i,
			Name: "Elmer",
		}
		err = DB.Insert(object)
		g.Expect(err).To(gomega.BeNil())
	}
	// Handler B
	handlerB := &TestHandler{name: "B"}
	watchB, err := DB.Watch(&TestObject{}, handlerB)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(watchB).ToNot(gomega.BeNil())
	// Update
	for i := 0; i < N; i++ {
		object := &TestObject{
			ID:   i,
			Name: "Fudd",
		}
		err = DB.Update(object)
		g.Expect(err).To(gomega.BeNil())
	}
	// Handler C
	handlerC := &TestHandler{name: "C"}
	watchC, err := DB.Watch(&TestObject{}, handlerC)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(watchC).ToNot(gomega.BeNil())
	// Delete
	for i := 0; i < N; i++ {
		object := &TestObject{
			ID: i,
		}
		err = DB.Delete(object)
		g.Expect(err).To(gomega.BeNil())
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
}

//
// Remove leading __ to enable.
func __TestConcurrency(t *testing.T) {
	var err error

	DB := New("/tmp/test.db", &TestObject{})
	DB.Open(true)

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
			time.Sleep(time.Millisecond * time.Duration(100))
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
			time.Sleep(time.Millisecond * time.Duration(300))
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
			time.Sleep(time.Millisecond * time.Duration(20))
		}
		done <- 0
	}
	transaction := func(done chan int) {
		var tx *Tx
		threshold := float64(10)
		for i := N; i < N*2; i++ {
			if i == N || math.Mod(float64(i), threshold) == 0 {
				tx, err = DB.Begin()
				if err != nil {
					panic(err)
				}
			}
			m := &TestObject{
				ID:   i,
				Name: "transaction",
			}
			DB.Insert(m)
			if err != nil {
				panic(err)
			}
			//time.Sleep(time.Second*3)
			if math.Mod(float64(i), threshold) == 0 {
				err = tx.Commit()
				if err != nil {
					panic(err)
				}
				fmt.Printf("commit|%d\n", i)
			}
			fmt.Printf("transaction|%d\n", i)
			time.Sleep(time.Millisecond * time.Duration(100))
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

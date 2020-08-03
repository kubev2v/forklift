package model

import (
	"errors"
	"fmt"
	"github.com/onsi/gomega"
	"math"
	"testing"
	"time"
)

type Thing struct {
	PK       string `sql:"pk"`
	ID       int    `sql:"key"`
	Name     string `sql:"key"`
	Revision int64  `sql:"revision"`
	Int8     int8   `sql:"int8"`
	Int16    int16  `sql:"int16"`
	Int32    int32  `sql:"int32"`
	labels   Labels
}

func (m *Thing) Pk() string {
	return m.PK
}

func (m *Thing) SetPk() {
	m.PK = fmt.Sprintf("%d", m.ID)
}

func (m *Thing) String() string {
	return fmt.Sprintf(
		"Thing: id: %d, name:%s",
		m.ID,
		m.Name)
}

func (m *Thing) Equals(other Model) bool {
	return false
}

func (m *Thing) Labels() Labels {
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

func (w *TestHandler) Created(model Model) {
	if thing, cast := model.(*Thing); cast {
		w.created = append(w.created, thing.ID)
	}
}

func (w *TestHandler) Updated(model Model) {
	if thing, cast := model.(*Thing); cast {
		w.updated = append(w.updated, thing.ID)
	}
}
func (w *TestHandler) Deleted(model Model) {
	if thing, cast := model.(*Thing); cast {
		w.deleted = append(w.deleted, thing.ID)
	}
}

func (w *TestHandler) Error(err error) {
	w.err = append(w.err, err)
}

func (w *TestHandler) End() {
}

func TestModels(t *testing.T) {
	var err error

	g := gomega.NewGomegaWithT(t)

	// Build the DB.
	DB := New(
		"/tmp/test.db",
		&Label{},
		&Thing{})
	err = DB.Open(true)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(DB.(*Client).db).ToNot(gomega.BeNil())

	// Test create handler.
	DB.Journal().Enable()
	handlerA := &TestHandler{name: "A"}
	watchA, err := DB.Watch(&Thing{}, handlerA)
	g.Expect(watchA).ToNot(gomega.BeNil())
	g.Expect(err).To(gomega.BeNil())

	// Create a model.
	thing := &Thing{
		ID:   0,
		Name: "Elmer",
		labels: Labels{
			"role": "main",
		},
	}

	// Test CRUD.
	err = DB.Insert(thing)
	g.Expect(err).To(gomega.BeNil())
	err = DB.Get(thing)
	g.Expect(err).To(gomega.BeNil())
	err = DB.Update(thing)
	g.Expect(err).To(gomega.BeNil())

	// Test conflict.
	DB.Update(thing)
	err = DB.Update(thing)
	g.Expect(err).To(gomega.BeNil())
	err = DB.Update(thing)
	g.Expect(errors.Is(err, Conflict)).To(gomega.BeTrue())

	// Test List
	list := []Thing{}
	err = DB.List(thing, ListOptions{}, &list)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(list)).To(gomega.Equal(1))

	// Test List by label.
	list = []Thing{}
	err = DB.List(
		thing,
		ListOptions{
			Labels: Labels{
				"role": "main",
			}},
		&list)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(list)).To(gomega.Equal(1))
	list = []Thing{}
	err = DB.List(
		thing,
		ListOptions{
			Labels: Labels{
				"job": "other",
			}},
		&list)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(len(list)).To(gomega.Equal(0))

	// Test Tx - commit
	thing.ID = 1
	tx, err := DB.Begin()
	g.Expect(err).To(gomega.BeNil())
	g.Expect(tx.ref).To(gomega.Equal(DB.(*Client).tx))
	err = DB.Insert(thing)
	g.Expect(err).To(gomega.BeNil())
	err = DB.Get(thing)
	g.Expect(errors.Is(err, NotFound)).To(gomega.BeTrue())
	err = tx.Commit()
	g.Expect(err).To(gomega.BeNil())
	g.Expect(DB.(*Client).tx).To(gomega.BeNil())
	err = DB.Get(thing)
	g.Expect(err).To(gomega.BeNil())

	// Test Tx - rollback
	thing.ID = 2
	tx, err = DB.Begin()
	g.Expect(err).To(gomega.BeNil())
	err = DB.Insert(thing)
	g.Expect(err).To(gomega.BeNil())
	err = DB.Get(thing)
	g.Expect(errors.Is(err, NotFound)).To(gomega.BeTrue())
	tx.End()
	g.Expect(DB.(*Client).tx).To(gomega.BeNil())
	err = DB.Get(thing)
	g.Expect(errors.Is(err, NotFound)).To(gomega.BeTrue())

	handlerB := &TestHandler{name: "B"}
	watchB, err := DB.Watch(&Thing{}, handlerB)
	g.Expect(watchB).ToNot(gomega.BeNil())
	g.Expect(err).To(gomega.BeNil())

	created := []int{0, 1}
	updated := []int{0, 0}
	for i := 2; i < 100; i++ {
		created = append(created, i)
		thing.ID = i
		err = DB.Insert(thing)
		g.Expect(err).To(gomega.BeNil())
	}

	for i := 0; i < 10; i++ {
		time.Sleep(time.Second)
		if len(handlerA.created) != len(created) ||
			len(handlerA.updated) != len(updated) ||
			len(handlerB.created) != len(created) {
			continue
		} else {
			break
		}
	}

	g.Expect(handlerA.created).To(
		gomega.Equal(created))
	g.Expect(handlerA.updated).To(
		gomega.Equal(updated))
	g.Expect(handlerB.created).To(
		gomega.Equal(created))
}

//
// Remove leading __ to enable.
func __TestConcurrency(t *testing.T) {
	var err error

	DB := New("/tmp/test.db", &Thing{})
	DB.Open(true)

	N := 1000

	direct := func(done chan int) {
		for i := 0; i < N; i++ {
			m := &Thing{
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
			m := &Thing{
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
			m := &Thing{
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
			m := &Thing{
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
			m := &Thing{
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

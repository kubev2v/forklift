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
	labels   Labels
}

func (m *Thing) Pk() string {
	return m.PK
}

func (m *Thing) SetPk() {
	m.PK = fmt.Sprintf("%d", m.ID)
}

func (m *Thing) String() string {
	return m.Name
}

func (m *Thing) Equals(other Model) bool {
	return false
}

func (m *Thing) Labels() Labels {
	return m.labels
}

func TestModels(t *testing.T) {
	var err error
	DB := New(
		"/tmp/test.db",
		&Label{},
		&Thing{})
	DB.Open(true)
	client := DB.(*Client)

	g := gomega.NewGomegaWithT(t)
	g.Expect(err).To(gomega.BeNil())
	g.Expect(client.db).ToNot(gomega.BeNil())

	thing := &Thing{
		ID: 0,
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
	g.Expect(tx.ref).To(gomega.Equal(client.tx))
	err = DB.Insert(thing)
	g.Expect(err).To(gomega.BeNil())
	err = DB.Get(thing)
	g.Expect(errors.Is(err, NotFound)).To(gomega.BeTrue())
	err = tx.Commit()
	g.Expect(err).To(gomega.BeNil())
	g.Expect(client.tx).To(gomega.BeNil())
	err = DB.Get(thing)
	g.Expect(err).To(gomega.BeNil())

	// Test Tx - rellback
	thing.ID = 2
	tx, err = DB.Begin()
	g.Expect(err).To(gomega.BeNil())
	err = DB.Insert(thing)
	g.Expect(err).To(gomega.BeNil())
	err = DB.Get(thing)
	g.Expect(errors.Is(err, NotFound)).To(gomega.BeTrue())
	tx.rollback()
	g.Expect(client.tx).To(gomega.BeNil())
	err = DB.Get(thing)
	g.Expect(errors.Is(err, NotFound)).To(gomega.BeTrue())
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

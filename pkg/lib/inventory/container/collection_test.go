package container

import (
	"errors"
	"strconv"
	"testing"

	fb "github.com/kubev2v/forklift/pkg/lib/filebacked"
	"github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/onsi/gomega"
)

type TestObject2 struct {
	ID       int    `sql:"pk"`
	Revision int    `sql:"incremented"`
	Name     string `sql:""`
	Age      int    `sql:""`
}

func (r *TestObject2) Pk() string {
	return strconv.Itoa(r.ID)
}

func TestCollection(t *testing.T) {
	var err error
	g := gomega.NewGomegaWithT(t)
	DB := model.New("/tmp/test2.db", &TestObject2{})
	err = DB.Open(true)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	desired := []TestObject2{}
	for i := 0; i < 10; i++ {
		m := TestObject2{
			ID:   i,
			Name: strconv.Itoa(i),
			Age:  i,
		}
		err = DB.Insert(&m)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		desired = append(desired, m)
		g.Expect(err).ToNot(gomega.HaveOccurred())
	}

	//
	// Test nothing changed.
	stored, err := DB.Find(
		&TestObject2{},
		model.ListOptions{
			Detail: model.MaxDetail,
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	collection := Collection{
		Stored: stored,
	}
	err = collection.Reconcile(asIter(desired))
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(collection.Added).To(gomega.Equal(0))
	g.Expect(collection.Updated).To(gomega.Equal(0))
	g.Expect(collection.Deleted).To(gomega.Equal(0))

	//
	// Test adds.
	stored, err = DB.Find(
		&TestObject2{},
		model.ListOptions{
			Detail: model.MaxDetail,
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	for i := 11; i < 15; i++ {
		desired = append(
			desired, TestObject2{
				ID:   i,
				Name: strconv.Itoa(i),
				Age:  i,
			})
	}
	tx, _ := DB.Begin()
	defer func() {
		_ = tx.End()
	}()
	collection = Collection{
		Stored: stored,
		Tx:     tx,
	}
	err = collection.Add(asIter(desired))
	g.Expect(err).ToNot(gomega.HaveOccurred())
	_ = tx.Commit()
	g.Expect(collection.Added).To(gomega.Equal(4))
	g.Expect(collection.Updated).To(gomega.Equal(0))
	g.Expect(collection.Deleted).To(gomega.Equal(0))

	//
	// Test updates.
	stored, err = DB.Find(
		&TestObject2{},
		model.ListOptions{
			Detail: model.MaxDetail,
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	desired[6].Name = "Larry"
	desired[8].Age = 100
	tx, _ = DB.Begin()
	defer func() {
		_ = tx.End()
	}()
	collection = Collection{
		Stored: stored,
		Tx:     tx,
	}
	err = collection.Update(asIter(desired))
	g.Expect(err).ToNot(gomega.HaveOccurred())
	_ = tx.Commit()
	g.Expect(collection.Added).To(gomega.Equal(0))
	g.Expect(collection.Updated).To(gomega.Equal(2))
	g.Expect(collection.Deleted).To(gomega.Equal(0))
	updated := &TestObject2{ID: 6}
	err = DB.Get(updated)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(updated.Name).To(gomega.Equal("Larry"))
	updated = &TestObject2{ID: 8}
	err = DB.Get(updated)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(updated.Age).To(gomega.Equal(100))

	//
	// Test deletes.
	stored, err = DB.Find(
		&TestObject2{},
		model.ListOptions{
			Detail: model.MaxDetail,
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	desired = desired[2:]
	tx, _ = DB.Begin()
	defer func() {
		_ = tx.End()
	}()
	collection = Collection{
		Stored: stored,
		Tx:     tx,
	}
	err = collection.Delete(asIter(desired))
	g.Expect(err).ToNot(gomega.HaveOccurred())
	_ = tx.Commit()
	g.Expect(collection.Added).To(gomega.Equal(0))
	g.Expect(collection.Updated).To(gomega.Equal(0))
	g.Expect(collection.Deleted).To(gomega.Equal(2))
	deleted := &TestObject2{ID: 0}
	err = DB.Get(deleted)
	g.Expect(errors.Is(err, model.NotFound)).To(gomega.BeTrue())
	updated = &TestObject2{ID: 1}
	err = DB.Get(deleted)
	g.Expect(errors.Is(err, model.NotFound)).To(gomega.BeTrue())

	// Test reconcile.
	stored, err = DB.Find(
		&TestObject2{},
		model.ListOptions{
			Detail: model.MaxDetail,
		})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	// delete
	desired = desired[2:]
	// update
	desired[3].Name = "Ashley"
	desired[5].Name = "Courtney"
	// add
	for i := 15; i < 20; i++ {
		desired = append(
			desired, TestObject2{
				ID:   i,
				Name: strconv.Itoa(i),
				Age:  i,
			})
	}
	tx, _ = DB.Begin()
	defer func() {
		_ = tx.End()
	}()
	collection = Collection{
		Stored: stored,
		Tx:     tx,
	}
	err = collection.Reconcile(asIter(desired))
	g.Expect(err).ToNot(gomega.HaveOccurred())
	_ = tx.Commit()
	g.Expect(collection.Added).To(gomega.Equal(5))
	g.Expect(collection.Updated).To(gomega.Equal(2))
	g.Expect(collection.Deleted).To(gomega.Equal(2))
}

// Iterator of models.
func asIter(models []TestObject2) fb.Iterator {
	list := fb.NewList()
	for _, m := range models {
		list.Append(m)
	}

	return list.Iter()
}

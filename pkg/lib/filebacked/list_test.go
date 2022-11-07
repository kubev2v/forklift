package filebacked

import (
	"fmt"
	"github.com/onsi/gomega"
	"testing"
	"time"
)

func TestList(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	type Ref struct {
		ID string
	}

	type Person struct {
		ID   int
		Name string
		Age  int
		List []string
		Ref  []Ref
	}

	type User struct {
		ID   int
		Name string
	}

	input := []interface{}{}
	for i := 0; i < 10; i++ {
		input = append(
			input,
			&Person{
				ID:   i,
				Name: "Elmer",
				Age:  i + 10,
				List: []string{"A", "B"},
				Ref:  []Ref{{"id0"}},
			})
		input = append(
			input,
			User{
				ID:   i,
				Name: "john",
			})
	}

	cat := &catalog

	list := NewList()

	// append
	for i := 0; i < len(input); i++ {
		list.Append(input[i])
	}
	g.Expect(len(cat.content)).To(gomega.Equal(2))
	g.Expect(list.Len()).To(gomega.Equal(len(input)))

	// iterate
	itr := list.Iter()
	g.Expect(itr.Len()).To(gomega.Equal(len(input)))
	for i := 0; i < len(input); i++ {
		object, hasNext := itr.Next()
		g.Expect(object).ToNot(gomega.BeNil())
		g.Expect(hasNext).To(gomega.BeTrue())
		g.Expect(itr.Len()).To(gomega.Equal(len(input)))
	}

	// next()
	n := 0
	itr = list.Iter()
	for {
		object, hasNext := itr.Next()
		if hasNext {
			n++
		} else {
			break
		}
		g.Expect(object).ToNot(gomega.BeNil())
		g.Expect(hasNext).To(gomega.BeTrue())
	}
	g.Expect(n).To(gomega.Equal(len(input)))

	// nextWith()
	itr = list.Iter()
	for n = 0; ; n += 2 {
		person := &Person{}
		hasNext := itr.NextWith(person)
		if !hasNext {
			break
		}
		user := &User{}
		hasNext = itr.NextWith(user)
		if !hasNext {
			break
		}
		g.Expect(person).ToNot(gomega.BeNil())
		g.Expect(person.ID).To(gomega.Equal(n / 2))
		g.Expect(user).ToNot(gomega.BeNil())
		g.Expect(user.ID).To(gomega.Equal(n / 2))
		g.Expect(hasNext).To(gomega.BeTrue())
	}
	g.Expect(n).To(gomega.Equal(len(input)))

	// Direct index.
	itr = list.Iter()
	for n = 0; n < itr.Len(); n++ {
		object := itr.At(n)
		g.Expect(object).ToNot(gomega.BeNil())
	}
	g.Expect(n).To(gomega.Equal(len(input)))

	// Mixed direct index and nextWith().
	itr = list.Iter()
	for n = 0; n < itr.Len(); n += 2 {
		person := &Person{}
		person4 := &Person{}
		itr.AtWith(8, person4)
		_ = itr.NextWith(person)
		g.Expect(person.ID).To(gomega.Equal(n / 2))
		g.Expect(person4.ID).To(gomega.Equal(4))
		user := &User{}
		user2 := &User{}
		_ = itr.NextWith(user)
		itr.AtWith(4, user2)
		g.Expect(user.ID).To(gomega.Equal(n / 2))
		g.Expect(user2.ID).To(gomega.Equal(2))
	}
	g.Expect(n).To(gomega.Equal(len(input)))

	// Direct index (with).
	itr = list.Iter()
	for n = 0; n < itr.Len(); n += 2 {
		person := &Person{}
		itr.AtWith(n, person)
		user := &User{}
		itr.AtWith(n+1, user)
		g.Expect(person).ToNot(gomega.BeNil())
		g.Expect(person.ID).To(gomega.Equal(n / 2))
		g.Expect(user).ToNot(gomega.BeNil())
		g.Expect(user.ID).To(gomega.Equal(n / 2))
	}
	g.Expect(n).To(gomega.Equal(len(input)))

	// List direct index.
	for n = 0; n < list.Len(); n++ {
		object := list.At(n)
		g.Expect(object).ToNot(gomega.BeNil())
	}
	g.Expect(n).To(gomega.Equal(len(input)))

	// List direct index (with).
	for n = 0; n < list.Len(); n += 2 {
		person := &Person{}
		list.AtWith(n, person)
		user := &User{}
		list.AtWith(n+1, user)
		g.Expect(person).ToNot(gomega.BeNil())
		g.Expect(person.ID).To(gomega.Equal(n / 2))
		g.Expect(user).ToNot(gomega.BeNil())
		g.Expect(user.ID).To(gomega.Equal(n / 2))
	}
	g.Expect(n).To(gomega.Equal(len(input)))

	// Reverse
	list = NewList()
	for i := 0; i < 3; i++ {
		list.Append(i)
	}
	itr = list.Iter()
	itr.Reverse()
	slice := []int{}
	for {
		n, hasNext := itr.Next()
		if hasNext {
			slice = append(slice, *n.(*int))
		} else {
			break
		}
	}
	g.Expect(slice).To(gomega.Equal([]int{2, 1, 0}))

	// Append iterator.
	listA := NewList()
	for i := 0; i < 3; i++ {
		listA.Append(i)
	}
	listB := NewList()
	listB.Append(listA.Iter())
	g.Expect(listA.Len()).To(gomega.Equal(listB.Len()))
}

// Disabled by default.
func __TestListPerf(t *testing.T) {
	list := NewList()
	defer list.Close()

	N := 100000

	mark := time.Now()
	for n := 0; n < N; n++ {
		list.Append(n)
	}
	duration := time.Since(mark)
	fmt.Printf("Append() total=%s per:%s\n", duration, duration/time.Duration(N))

	mark = time.Now()
	itr := list.Iter()
	for i := 0; i < itr.Len(); i++ {
		_, _ = itr.Next()
	}
	itr.Close()
	duration = time.Since(mark)
	fmt.Printf("Next() total=%s per:%s\n", duration, duration/time.Duration(N))

	n := 0
	mark = time.Now()
	itr = list.Iter()
	for i := 0; i < itr.Len(); i++ {
		_ = itr.NextWith(&n)
	}
	itr.Close()
	duration = time.Since(mark)
	fmt.Printf("NextWith() total=%s per:%s\n", duration, duration/time.Duration(N))

	mark = time.Now()
	itr = list.Iter()
	for i := 0; i < itr.Len(); i++ {
		_ = itr.At(i)
	}
	itr.Close()
	duration = time.Since(mark)
	fmt.Printf("At() total=%s per:%s\n", duration, duration/time.Duration(N))

	mark = time.Now()
	itr = list.Iter()
	for i := 0; i < itr.Len(); i++ {
		itr.AtWith(i, &n)
	}
	itr.Close()
	duration = time.Since(mark)
	fmt.Printf("AtWith() total=%s per:%s\n", duration, duration/time.Duration(N))
}

/*
Provides file-backed list.

//
// New list.
list := fb.NewList()

//
// Append an object.
list.Append(object)

//
// Iterate the list.
itr := list.Iter()
for i := 0; i < itr.Len(); i++ {
    person := itr.At(i)
    ...
}

//
// Iterate the list.
itr := list.Iter()
for i := 0; i < itr.Len(); i++ {
    person := Person{}
    itr.AtWith(i, &person))
    ...
}

//
// Iterate the list.
itr := list.Iter()
for {
    object, hasNext := itr.Next()
    if !hasNext {
        break
    }
    ...
}

//
// Iterate the list.
itr := list.Iter()
for object, hasNext := itr.Next(); hasNext; object, hasNext = itr.Next() {
    ...
}

//
// Iterate the list.
itr := list.Iter()
for {
    person := Person{}
    hasNext := itr.NextWith(&person))
    if !hasNext {
        break
    }
    ...
}
*/
package filebacked

import (
	"runtime"
)

//
// List factory.
func NewList() (list *List) {
	list = &List{}
	runtime.SetFinalizer(
		list,
		func(l *List) {
			l.Close()
		})
	return
}

//
// File-backed list.
type List struct {
	// File writer.
	writer Writer
}

//
// Append an object.
func (l *List) Append(object interface{}) {
	switch object.(type) {
	case Iterator:
		itr := object.(Iterator)
		for {
			object, hasNext := itr.Next()
			if hasNext {
				l.writer.Append(object)
			} else {
				break
			}
		}
	default:
		l.writer.Append(object)
	}
}

//
// Length.
// Number of objects.
func (l *List) Len() int {
	return len(l.writer.index)
}

// Object at index.
func (l *List) At(index int) (object interface{}) {
	reader := l.writer.Reader(true)
	object = reader.At(index)
	return
}

// Object at index.
func (l *List) AtWith(index int, object interface{}) {
	reader := l.writer.Reader(true)
	reader.AtWith(index, object)
	return
}

//
// Get an iterator.
func (l *List) Iter() (itr Iterator) {
	if l.Len() > 0 {
		itr = &FbIterator{
			Reader: l.writer.Reader(false),
		}
	} else {
		itr = &EmptyIterator{}
	}

	return
}

//
// Close (delete) the list.
func (l *List) Close() {
	l.writer.Close()
}

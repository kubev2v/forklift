package filebacked

import (
	"reflect"
	"sync"
)

// Catalog (singleton).
var catalog = Catalog{}

// Type catalog.
type Catalog struct {
	sync.Mutex
	content []interface{}
}

// Add object (proto) to the catalog.
func (r *Catalog) add(object interface{}) (kind uint16) {
	if object == nil {
		return
	}
	r.Lock()
	defer r.Unlock()
	ot := reflect.TypeOf(object)
	ov := reflect.ValueOf(object)
	if reflect.TypeOf(object).Kind() == reflect.Ptr {
		ot = ot.Elem()
		ov = ov.Elem()
	}
	// Found.
	for k, f := range r.content {
		if ot == reflect.TypeOf(f) {
			kind = uint16(k)
			return
		}
	}
	// Added.
	kind = uint16(len(r.content))
	r.content = append(r.content, ov.Interface())

	return
}

// Build object using the catalog.
func (r *Catalog) build(kind uint16) (object interface{}, found bool) {
	r.Lock()
	defer r.Unlock()
	content := r.content
	i := int(kind)
	if i < len(content) {
		object = content[i]
		object = reflect.New(reflect.TypeOf(object)).Interface()
		found = true
	}

	return
}

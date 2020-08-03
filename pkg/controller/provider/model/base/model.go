package base

import "encoding/json"

//
// Bool
type Bool bool

//
// Encode the bool.
func (r *Bool) Encode() int {
	if *r {
		return 1
	}

	return 0
}

//
// Decode the bool.
func (r *Bool) With(n int) *Bool {
	*r = n != 0
	return r
}

//
// Bool pointer.
func BoolPtr(v bool) *Bool {
	b := Bool(v)
	return &b
}

//
// Annotations
type Annotation map[string]string

//
// Encode the annotations.
func (r *Annotation) Encode() string {
	j, _ := json.Marshal(r)
	return string(j)
}

//
// Unmarshal the json `j` into self.
func (r *Annotation) With(j string) *Annotation {
	json.Unmarshal([]byte(j), r)
	return r
}

//
// Annotation pointer.
func AnnotationPtr() *Annotation {
	a := Annotation{}
	return &a
}

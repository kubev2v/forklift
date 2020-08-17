package base

import "encoding/json"

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

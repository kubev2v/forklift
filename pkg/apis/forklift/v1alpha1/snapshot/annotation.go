package snapshot

import (
	"encoding/json"
	"errors"
	liberr "github.com/konveyor/controller/pkg/error"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

const (
	// Root domain.
	Domain = "snapshot"
)

var NotFoundErr = errors.New("not found")

//
// Snapshot.
type Snapshot interface {
	// Set object.
	Set(string, interface{}) error
	// Get object.
	Get(string, interface{}) error
	// Contains key.
	Contains(string) bool
	// Delete all.
	Purge()
	// Update from another object.
	Update(meta.Object)
}

//
// Factory.
func New(object meta.Object) Snapshot {
	return &Annotation{Object: object}
}

//
// Annotation-based snapshot.
type Annotation struct {
	meta.Object
}

//
// Set object.
func (r *Annotation) Set(key string, object interface{}) (err error) {
	if object == nil {
		return
	}
	key = r.key(key)
	b, jErr := json.Marshal(object)
	if jErr != nil {
		err = liberr.Wrap(jErr)
		return
	}
	m := r.Object.GetAnnotations()
	if m == nil {
		m = map[string]string{}
		r.SetAnnotations(m)
	}
	m[key] = string(b)
	return
}

//
// Get object.
func (r *Annotation) Get(key string, object interface{}) (err error) {
	m := r.GetAnnotations()
	if m == nil {
		err = liberr.Wrap(NotFoundErr)
		return
	}
	key = r.key(key)
	if s, found := m[key]; found {
		jErr := json.Unmarshal([]byte(s), object)
		if jErr != nil {
			err = liberr.Wrap(jErr)
		}
	}

	return
}

//
// Contains key.
func (r *Annotation) Contains(key string) (found bool) {
	_, found = r.GetAnnotations()[r.key(key)]
	return
}

//
// Purge all snapshots.
func (r *Annotation) Purge() {
	m := r.GetAnnotations()
	if m == nil {
		return
	}
	for key := range m {
		if r.hasDomain(key) {
			delete(m, key)
		}
	}
}

//
// Update from another object.
func (r *Annotation) Update(object meta.Object) {
	m := r.Object.GetAnnotations()
	if m == nil {
		m = map[string]string{}
		r.SetAnnotations(m)
	}
	for k, v := range object.GetAnnotations() {
		if r.hasDomain(k) {
			m[k] = v
		}
	}
}

//
// build key.
func (r *Annotation) key(key string) string {
	if !r.hasDomain(key) {
		key = strings.Join([]string{Domain, key}, ".")
	}

	return key
}

//
// Key is scoped to the this domain.
func (r *Annotation) hasDomain(key string) bool {
	parts := strings.Split(key, ".")
	return parts[0] == Domain
}

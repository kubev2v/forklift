package container

import (
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/ref"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sync"
)

//
// Reconciler key.
type Key core.ObjectReference

//
// A container manages a collection of `Reconciler`.
type Container struct {
	// Collection of reconcilers.
	content map[Key]Reconciler
	// Mutex - protect the map..
	mutex sync.RWMutex
}

//
// Get a reconciler by (CR) object.
func (c *Container) Get(object meta.Object) (Reconciler, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	p, found := c.content[c.key(object)]
	return p, found
}

//
// Add a reconciler.
func (c *Container) Add(object meta.Object, reconciler Reconciler) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	key := c.key(object)
	if current, found := c.content[key]; found {
		current.Shutdown(false)
	}
	c.content[key] = reconciler
	err := reconciler.Start()
	if err != nil {
		delete(c.content, key)
		reconciler.Shutdown(false)
		return liberr.Wrap(err)
	}

	return nil
}

//
// Delete the reconciler.
func (c *Container) Delete(object meta.Object) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	key := c.key(object)
	if r, found := c.content[key]; found {
		delete(c.content, key)
		r.Shutdown(true)
	}
}

//
// Build a reconciler key for an object.
func (*Container) key(object meta.Object) Key {
	return Key{
		Kind:      ref.ToKind(object),
		Namespace: object.GetNamespace(),
		Name:      object.GetName(),
	}
}

//
// Data reconciler.
type Reconciler interface {
	// The name.
	Name() string
	// Start the reconciler.
	Start() error
	// Shutdown the reconciler.
	Shutdown(bool)
	// The reconciler has achieved consistency.
	HasConsistency() bool
	// Get the associated DB.
	DB() model.DB
	// Reset
	Reset()
}

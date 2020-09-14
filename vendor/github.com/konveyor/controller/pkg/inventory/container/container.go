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
func (c *Container) Get(owner meta.Object) (Reconciler, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	p, found := c.content[c.key(owner)]
	return p, found
}

//
// List all reconcilers.
func (c *Container) List() []Reconciler {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	list := []Reconciler{}
	for _, r := range c.content {
		list = append(list, r)
	}

	return list
}

//
// Add a reconciler.
func (c *Container) Add(reconciler Reconciler) (err error) {
	add := func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		key := c.key(reconciler.Owner())
		if _, found := c.content[key]; found {
			err = liberr.New("duplicate")
			return
		}
		c.content[key] = reconciler
	}
	add()
	if err != nil {
		return
	}
	err = reconciler.Start()
	if err != nil {
		return liberr.Wrap(err)
	}

	return
}

//
// Replace a reconciler.
func (c *Container) Replace(reconciler Reconciler) (p Reconciler, found bool, err error) {
	replace := func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		key := c.key(reconciler.Owner())
		if p, found := c.content[key]; found {
			p.Shutdown()
		}
		c.content[key] = reconciler
	}
	replace()
	if err != nil {
		return
	}
	err = reconciler.Start()
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}

//
// Delete the reconciler.
func (c *Container) Delete(owner meta.Object) (p Reconciler, found bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	key := c.key(owner)
	if p, found = c.content[key]; found {
		delete(c.content, key)
		p.Shutdown()
	}

	return
}

//
// Build a reconciler key for an object.
func (*Container) key(owner meta.Object) Key {
	return Key{
		Kind:      ref.ToKind(owner),
		Namespace: owner.GetNamespace(),
		Name:      owner.GetName(),
	}
}

//
// Data reconciler.
type Reconciler interface {
	// The name.
	Name() string
	// The resource that owns the reconciler.
	Owner() meta.Object
	// Start the reconciler.
	// Expected to do basic validation, start a
	// goroutine and return quickly.
	Start() error
	// Shutdown the reconciler.
	// Expected to disconnect, destroy created resources
	// and return quickly.
	Shutdown()
	// The reconciler has achieved consistency.
	HasConsistency() bool
	// Get the associated DB.
	DB() model.DB
	// Reset
	Reset()
}

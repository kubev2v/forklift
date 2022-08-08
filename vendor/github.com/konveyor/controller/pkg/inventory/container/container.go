package container

import (
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/controller/pkg/ref"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sync"
)

//
// Logger.
var log = logging.WithName("container")

//
// Collector key.
type Key core.ObjectReference

//
// A container manages a collection of `Collector`.
type Container struct {
	// Collection of data collectors.
	content map[Key]Collector
	// Mutex - protect the map..
	mutex sync.RWMutex
}

//
// Get a collector by (CR) object.
func (c *Container) Get(owner meta.Object) (Collector, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	p, found := c.content[c.key(owner)]
	return p, found
}

//
// List all collectors.
func (c *Container) List() []Collector {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	list := []Collector{}
	for _, r := range c.content {
		list = append(list, r)
	}

	return list
}

//
// Add a collector.
func (c *Container) Add(collector Collector) (err error) {
	owner := collector.Owner()
	key := c.key(owner)
	add := func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		if _, found := c.content[key]; found {
			err = liberr.New("duplicate")
			return
		}
		c.content[key] = collector
	}
	add()
	if err != nil {
		return
	}
	err = collector.Start()
	if err != nil {
		return liberr.Wrap(err)
	}

	log.V(3).Info(
		"collector added.",
		"owner",
		key)

	return
}

//
// Replace a collector.
func (c *Container) Replace(collector Collector) (p Collector, found bool, err error) {
	key := c.key(collector.Owner())
	replace := func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		if p, found := c.content[key]; found {
			p.Shutdown()
		}
		c.content[key] = collector
	}
	replace()
	err = collector.Start()
	if err != nil {
		err = liberr.Wrap(err)
	}

	log.V(3).Info(
		"collector replaced.",
		"owner",
		key)

	return
}

//
// Delete the collector.
func (c *Container) Delete(owner meta.Object) (p Collector, found bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	key := c.key(owner)
	if p, found = c.content[key]; found {
		delete(c.content, key)
		p.Shutdown()
		log.V(3).Info(
			"collector deleted.",
			"owner",
			key)
	}

	return
}

//
// Build a collector key for an object.
func (*Container) key(owner meta.Object) Key {
	return Key{
		Kind: ref.ToKind(owner),
		UID:  owner.GetUID(),
	}
}

//
// Data collector.
type Collector interface {
	// The name.
	Name() string
	// The resource that owns the collector.
	Owner() meta.Object
	// Start the collector.
	// Expected to do basic validation, start a
	// goroutine and return quickly.
	Start() error
	// Shutdown the collector.
	// Expected to disconnect, destroy created resources
	// and return quickly.
	Shutdown()
	// The collector has achieved parity.
	HasParity() bool
	// Get the associated DB.
	DB() model.DB
	// Test connection with credentials.
	Test() error
	// Reset
	Reset()
}

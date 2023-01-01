package ref

import (
	"k8s.io/api/core/v1"
	"sync"
)

// A resource that contains an ObjectReference.
type Owner v1.ObjectReference

// The resource that is the target of an ObjectReference.
type Target v1.ObjectReference

// A 1-n mapping of Target => [Owner, ...].
type RefMap struct {
	Content map[Target]map[Owner]bool
	mutex   sync.RWMutex
}

// Add mapping of a ref-owner to a ref-target.
func (r *RefMap) Add(owner Owner, target Target) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	owners, found := r.Content[target]
	if !found {
		owners = map[Owner]bool{}
		r.Content[target] = owners
	}

	r.Content[target][owner] = true

	log.V(3).Info(
		"map: added.",
		"owner",
		owner,
		"target",
		target)
}

// Delete mapping of a ref-owner to a ref-target.
func (r *RefMap) Delete(owner Owner, target Target) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	owners, found := r.Content[target]
	if found {
		delete(owners, owner)
		log.V(3).Info(
			"map: owner deleted.",
			"owner",
			owner)
	}
	r.Prune()
}

// Delete all mappings to an owner.
func (r *RefMap) DeleteOwner(owner Owner) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, owners := range r.Content {
		delete(owners, owner)
		log.V(3).Info(
			"map: owner deleted.",
			"owner",
			owner)
	}
	r.Prune()
}

// Determine if target mapped to owner.
func (r *RefMap) Match(target Target, owner Owner) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if owners, found := r.Content[target]; found {
		_, found = owners[owner]
		return found
	}

	return false
}

// Find all owners mapped to the target.
func (r *RefMap) Find(target Target) []Owner {
	list := []Owner{}
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if owners, found := r.Content[target]; found {
		for owner := range owners {
			list = append(list, owner)
		}
	}
	log.V(4).Info(
		"map: found owner for target.",
		"target",
		target,
		"owner",
		list)

	return list
}

// Prune empty mappings.
func (r *RefMap) Prune() {
	for key, owners := range r.Content {
		if len(owners) == 0 {
			delete(r.Content, key)
		}
	}
}

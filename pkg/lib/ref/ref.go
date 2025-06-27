package ref

import (
	"reflect"

	"github.com/kubev2v/forklift/pkg/lib/logging"
	v1 "k8s.io/api/core/v1"
)

// Global
var Map *RefMap
var Mapper *EventMapper

var log = logging.WithName("ref")

// Build globals.
func init() {
	Map = &RefMap{
		Content: map[Target]map[Owner]bool{},
	}
	Mapper = &EventMapper{
		Map: Map,
	}
}

// Determine if the ref is `set`.
// Must not be `nil` with Namespace and Name not "".
func RefSet(ref *v1.ObjectReference) bool {
	return ref != nil &&
		ref.Namespace != "" &&
		ref.Name != ""
}

// Equals comparison.
// May be used with `nil` pointers.
func DeepEquals(refA, refB *v1.ObjectReference) bool {
	if refA == nil || refB == nil {
		return false
	}

	return reflect.DeepEqual(refA, refB)
}

// Determind if both refs have the same name and namespace
func Equals(refA, refB *v1.ObjectReference) bool {
	if refA == nil || refB == nil {
		return refA == refB
	}

	return refA.Name == refB.Name && refA.Namespace == refB.Namespace
}

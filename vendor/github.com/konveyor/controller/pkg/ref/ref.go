package ref

import (
	"github.com/konveyor/controller/pkg/logging"
	"k8s.io/api/core/v1"
	"reflect"
)

// Global
var Map *RefMap
var Mapper *EventMapper

var log = logging.WithName("ref")

//
// Build globals.
func init() {
	Map = &RefMap{
		Content: map[Target]map[Owner]bool{},
	}
	Mapper = &EventMapper{
		Map: Map,
	}
}

//
// Determine if the ref is `set`.
// Must not be `nil` with Namespace and Name not "".
func RefSet(ref *v1.ObjectReference) bool {
	return ref != nil &&
		ref.Namespace != "" &&
		ref.Name != ""
}

//
// Equals comparison.
// May be used with `nil` pointers.
func Equals(refA, refB *v1.ObjectReference) bool {
	if refA == nil || refB == nil {
		return false
	}

	return reflect.DeepEqual(refA, refB)
}

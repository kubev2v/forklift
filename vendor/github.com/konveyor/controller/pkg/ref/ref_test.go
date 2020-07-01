package ref

import (
	"github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"testing"
)

type _ThingSpec struct {
	RefD *v1.ObjectReference `json:"refD" ref:"ThingD"`
}

type _Thing struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	RefA            *v1.ObjectReference `json:"refA" ref:"ThingA"`
	RefB            *v1.ObjectReference `json:"refB" ref:"ThingB"`
	RefC            v1.ObjectReference  `json:"refC" ref:"ThingC"`
	Spec            _ThingSpec
}

func (t *_Thing) GetObjectKind() schema.ObjectKind {
	return nil
}

func (t *_Thing) DeepCopyObject() runtime.Object {
	return t
}

func TestFindRefs(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup
	thing := &_Thing{
		ObjectMeta: meta.ObjectMeta{
			Namespace: "ns0",
			Name:      "joe",
		},
		RefA: &v1.ObjectReference{
			Namespace: "nsA",
			Name:      "thingA",
		},
		RefB: &v1.ObjectReference{
			Namespace: "nsB",
			Name:      "thingB",
		},
		RefC: v1.ObjectReference{
			Namespace: "",
			Name:      "thingC",
		},
	}

	// Test
	mapper := EventMapper{Map}
	aRefs := mapper.findRefs(thing)

	// Validation
	g.Expect(len(aRefs)).To(gomega.Equal(2))
}

func TestMapperCreate(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup
	m := &RefMap{
		Content: map[Target]map[Owner]bool{},
	}
	thing := &_Thing{
		ObjectMeta: meta.ObjectMeta{
			Namespace: "ns0",
			Name:      "joe",
		},
		RefA: &v1.ObjectReference{
			Namespace: "nsA",
			Name:      "thingA",
		},
		RefB: &v1.ObjectReference{
			Namespace: "nsB",
			Name:      "thingB",
		},
		RefC: v1.ObjectReference{
			Namespace: "",
			Name:      "thingC",
		},
	}

	// Test
	mapper := EventMapper{Map: m}
	mapper.Create(
		event.CreateEvent{
			Meta:   thing,
			Object: thing,
		})

	// Validation
	owner := Owner{
		Kind:      ToKind(thing),
		Namespace: "ns0",
		Name:      "joe",
	}
	targetA := Target{
		Kind:      "ThingA",
		Namespace: "nsA",
		Name:      "thingA",
	}
	targetB := Target{
		Kind:      "ThingB",
		Namespace: "nsB",
		Name:      "thingB",
	}
	g.Expect(len(m.Content)).To(gomega.Equal(2))
	g.Expect(m.Match(targetA, owner)).To(gomega.BeTrue())
	g.Expect(m.Match(targetB, owner)).To(gomega.BeTrue())
}

func TestMapperUpdate(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup
	m := &RefMap{
		Content: map[Target]map[Owner]bool{},
	}
	old := &_Thing{
		ObjectMeta: meta.ObjectMeta{
			Namespace: "ns0",
			Name:      "joe",
		},
		RefA: &v1.ObjectReference{
			Namespace: "nsA",
			Name:      "thingA",
		},
		RefB: &v1.ObjectReference{
			Namespace: "nsB",
			Name:      "thingB",
		},
		RefC: v1.ObjectReference{
			Namespace: "",
			Name:      "thingC",
		},
	}
	new := &_Thing{
		ObjectMeta: meta.ObjectMeta{
			Namespace: "ns0",
			Name:      "joe",
		},
		RefB: &v1.ObjectReference{
			Namespace: "nsB",
			Name:      "thingB",
		},
		RefC: v1.ObjectReference{
			Namespace: "nsC",
			Name:      "thingC",
		},
	}

	// Test
	mapper := EventMapper{Map: m}
	mapper.Create(
		event.CreateEvent{
			Meta:   old,
			Object: old,
		})
	mapper.Update(
		event.UpdateEvent{
			MetaOld:   old,
			ObjectOld: old,
			MetaNew:   new,
			ObjectNew: new,
		})

	// Validation
	owner := Owner{
		Kind:      ToKind(old),
		Namespace: "ns0",
		Name:      "joe",
	}
	targetA := Target{
		Kind:      "ThingA",
		Namespace: "nsA",
		Name:      "thingA",
	}
	targetB := Target{
		Kind:      "ThingB",
		Namespace: "nsB",
		Name:      "thingB",
	}
	targetC := Target{
		Kind:      "ThingC",
		Namespace: "nsC",
		Name:      "thingC",
	}
	g.Expect(len(m.Content)).To(gomega.Equal(2))
	g.Expect(m.Match(targetA, owner)).To(gomega.BeFalse())
	g.Expect(m.Match(targetB, owner)).To(gomega.BeTrue())
	g.Expect(m.Match(targetC, owner)).To(gomega.BeTrue())
}

func TestMapperDelete(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup
	m := &RefMap{
		Content: map[Target]map[Owner]bool{},
	}
	thing := &_Thing{
		ObjectMeta: meta.ObjectMeta{
			Namespace: "ns0",
			Name:      "joe",
		},
		RefA: &v1.ObjectReference{
			Namespace: "nsA",
			Name:      "thingA",
		},
		RefB: &v1.ObjectReference{
			Namespace: "nsB",
			Name:      "thingB",
		},
		RefC: v1.ObjectReference{
			Namespace: "",
			Name:      "thingC",
		},
	}

	mapper := EventMapper{Map: m}
	mapper.Create(
		event.CreateEvent{
			Meta:   thing,
			Object: thing,
		})
	owner := Owner{
		Kind:      ToKind(thing),
		Namespace: "ns0",
		Name:      "joe",
	}
	targetA := Target{
		Kind:      "ThingA",
		Namespace: "nsA",
		Name:      "thingA",
	}
	targetB := Target{
		Kind:      "ThingB",
		Namespace: "nsB",
		Name:      "thingB",
	}
	g.Expect(len(m.Content)).To(gomega.Equal(2))
	g.Expect(m.Match(targetA, owner)).To(gomega.BeTrue())
	g.Expect(m.Match(targetB, owner)).To(gomega.BeTrue())

	// Test
	mapper.Delete(
		event.DeleteEvent{
			Meta:   thing,
			Object: thing,
		})

	// Validation
	g.Expect(len(m.Content)).To(gomega.Equal(0))
}

func TestHandler(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup
	Map.Content = map[Target]map[Owner]bool{}
	owner := &_Thing{
		ObjectMeta: meta.ObjectMeta{
			Namespace: "ns0",
			Name:      "joe",
		},
		RefA: &v1.ObjectReference{
			Namespace: "nsA",
			Name:      "thingA",
		},
		RefB: &v1.ObjectReference{
			Namespace: "nsB",
			Name:      "thingB",
		},
		RefC: v1.ObjectReference{
			Namespace: "",
			Name:      "thingC",
		},
	}

	type ThingB struct {
		_Thing
	}
	target := &ThingB{
		_Thing: _Thing{
			ObjectMeta: meta.ObjectMeta{
				Namespace: "nsB",
				Name:      "thingB",
			},
		},
	}

	// Test
	mapper := EventMapper{Map}
	mapper.Create(
		event.CreateEvent{
			Meta:   owner,
			Object: owner,
		})

	list := GetRequests(
		handler.MapObject{
			Meta:   target,
			Object: target,
		})

	g.Expect(len(list)).To(gomega.Equal(1))
}

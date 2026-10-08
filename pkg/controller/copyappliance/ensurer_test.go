package copyappliance

import (
	"context"
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testEnsurer(t *testing.T, objs ...client.Object) *Ensurer {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := api.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme forklift: %v", err)
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()
	return &Ensurer{Client: cl, Log: testLog()}
}

// labelled is an appliance the API server has already named, as one read back
// from a List is.
func labelled(name string, labels map[string]string) *api.CopyAppliance {
	return &api.CopyAppliance{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: "forklift",
			Labels:    labels,
		},
	}
}

func applianceLabels() map[string]string {
	labeler := Labeler{}
	return labeler.ApplianceLabels(testProvider(), testMigrationUID, testRef.ID)
}

func checkLabels() map[string]string {
	labeler := Labeler{}
	return labeler.CheckLabels(testProvider())
}

func TestFind(t *testing.T) {
	t.Run("nothing matching", func(t *testing.T) {
		e := testEnsurer(t)
		found, err := e.Find(context.TODO(), "forklift", applianceLabels(), true)
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		if found != nil {
			t.Errorf("found %q, want nil", found.Name)
		}
	})

	t.Run("one match", func(t *testing.T) {
		e := testEnsurer(t, labelled("appliance-abcde", applianceLabels()))
		found, err := e.Find(context.TODO(), "forklift", applianceLabels(), true)
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		if found == nil || found.Name != "appliance-abcde" {
			t.Errorf("found %v, want appliance-abcde", found)
		}
	})

	// The appliance holds read locks on the disks it serves. A second one for
	// the same VM is not something to pick between quietly.
	t.Run("two live matches", func(t *testing.T) {
		e := testEnsurer(t,
			labelled("appliance-abcde", applianceLabels()),
			labelled("appliance-fghij", applianceLabels()))
		found, err := e.Find(context.TODO(), "forklift", applianceLabels(), true)
		if err == nil {
			t.Fatal("Find succeeded with two matching appliances")
		}
		if !strings.Contains(err.Error(), "multiple") {
			t.Errorf("error = %v, want it to say there are several", err)
		}
		if found != nil {
			t.Errorf("found %q alongside the error, want nil", found.Name)
		}
	})

	// A migration appliance and a provider's check appliance share every label
	// but the subapp, and must never be taken for each other.
	t.Run("the subapp keeps the kinds apart", func(t *testing.T) {
		e := testEnsurer(t, labelled("appliance-abcde", applianceLabels()))
		found, err := e.Find(context.TODO(), "forklift", checkLabels(), true)
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		if found != nil {
			t.Errorf("a check lookup found the migration appliance %q", found.Name)
		}
	})
}

// A torn-down appliance is no use to whatever is asking: it is releasing its
// disk locks, not serving. A live-only Find hides it so that a retry builds a
// new one; one that is not live-only shows it so a teardown can be waited on.
func TestFindAndTerminating(t *testing.T) {
	terminating := labelled("appliance-abcde", applianceLabels())
	now := meta.Now()
	terminating.DeletionTimestamp = &now
	terminating.Finalizers = []string{"forklift"}
	e := testEnsurer(t, terminating)

	found, err := e.Find(context.TODO(), "forklift", applianceLabels(), true)
	if err != nil {
		t.Fatalf("live-only Find: %v", err)
	}
	if found != nil {
		t.Errorf("live-only Find returned the terminating appliance %q", found.Name)
	}

	found, err = e.Find(context.TODO(), "forklift", applianceLabels(), false)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if found == nil || found.Name != "appliance-abcde" {
		t.Errorf("Find found %v, want the terminating appliance", found)
	}
}

// One appliance per set of labels, however many passes ask for one. This is
// what replaces the API server rejecting a duplicate name.
func TestAppliance(t *testing.T) {
	e := testEnsurer(t)
	build := func() *api.CopyAppliance {
		return &api.CopyAppliance{
			ObjectMeta: meta.ObjectMeta{
				GenerateName: "copy-appliance-",
				Namespace:    "forklift",
				Labels:       applianceLabels(),
			},
		}
	}

	created, err := e.Appliance(context.TODO(), build())
	if err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if created.Name == "" {
		t.Fatal("the created appliance has no name")
	}

	again, err := e.Appliance(context.TODO(), build())
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if again.Name != created.Name {
		t.Errorf("second pass returned %q, want the %q it already made",
			again.Name, created.Name)
	}

	list := &api.CopyApplianceList{}
	if err := e.List(context.TODO(), list); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Errorf("created %d appliances, want one", len(list.Items))
	}
}

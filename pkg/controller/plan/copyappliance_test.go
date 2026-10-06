package plan

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	appliancectrl "github.com/kubev2v/forklift/pkg/controller/copyappliance"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const testMigrationUID = types.UID("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")

var testVM = &plan.VMStatus{VM: plan.VM{Ref: ref.Ref{ID: "vm-101"}}}

// testCopyApplianceMigration is a migration over a fake client holding objs.
func testCopyApplianceMigration(t *testing.T, objs ...client.Object) *Migration {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := api.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme forklift: %v", err)
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()
	context := &plancontext.Context{
		Client:    cl,
		Log:       logging.WithName("copy-appliance-plan-test"),
		Plan:      &api.Plan{ObjectMeta: meta.ObjectMeta{Namespace: "forklift", Name: "plan", UID: "plan-uid"}},
		Migration: &api.Migration{ObjectMeta: meta.ObjectMeta{Namespace: "forklift", Name: "migration", UID: testMigrationUID}},
	}
	context.Source.Provider = &api.Provider{
		ObjectMeta: meta.ObjectMeta{Namespace: "forklift", Name: "vcenter", UID: "provider-uid"},
	}
	return &Migration{
		Context: context,
		copyApplianceEnsurer: appliancectrl.Ensurer{
			Client: cl,
			Log:    context.Log,
		},
	}
}

// testCopyAppliance is the appliance serving testVM for the migration, as the
// API server would have named it.
func testCopyAppliance() *api.CopyAppliance {
	labeler := appliancectrl.Labeler{}
	return &api.CopyAppliance{
		ObjectMeta: meta.ObjectMeta{
			Namespace:  "forklift",
			Name:       "copy-appliance-abcde",
			Labels:     labeler.ApplianceLabels(&api.Provider{ObjectMeta: meta.ObjectMeta{UID: "provider-uid"}}, testMigrationUID, testVM.ID),
			Finalizers: []string{api.CopyApplianceFinalizer},
		},
	}
}

func TestGetCopyAppliance(t *testing.T) {
	t.Run("no appliance is not an error", func(t *testing.T) {
		m := testCopyApplianceMigration(t)
		appliance, err := m.getCopyAppliance(testVM, true)
		if err != nil {
			t.Fatalf("getCopyAppliance: %v", err)
		}
		if appliance != nil {
			t.Errorf("found %q, want nil", appliance.Name)
		}
	})

	t.Run("the appliance is found by its labels", func(t *testing.T) {
		m := testCopyApplianceMigration(t, testCopyAppliance())
		appliance, err := m.getCopyAppliance(testVM, true)
		if err != nil {
			t.Fatalf("getCopyAppliance: %v", err)
		}
		if appliance == nil || appliance.Name != "copy-appliance-abcde" {
			t.Errorf("found %v, want copy-appliance-abcde", appliance)
		}
	})

	// A name-based lookup resolved a terminating appliance and handed back
	// something releasing its disk locks. The label lookup does not, so a
	// retry builds a new one rather than adopting the one on its way out.
	t.Run("an appliance being torn down is not found", func(t *testing.T) {
		terminating := testCopyAppliance()
		now := meta.Now()
		terminating.DeletionTimestamp = &now
		m := testCopyApplianceMigration(t, terminating)

		appliance, err := m.getCopyAppliance(testVM, true)
		if err != nil {
			t.Fatalf("getCopyAppliance: %v", err)
		}
		if appliance != nil {
			t.Errorf("found the terminating appliance %q", appliance.Name)
		}
	})
}

// Teardown is reported on the appliance's own status, so unlike every other
// caller it has to keep seeing the appliance after the delete is issued.
func TestTeardownCopyAppliance(t *testing.T) {
	t.Run("nothing to tear down is done", func(t *testing.T) {
		m := testCopyApplianceMigration(t)
		done, err := m.teardownCopyAppliance(testVM)
		if err != nil {
			t.Fatalf("teardownCopyAppliance: %v", err)
		}
		if !done {
			t.Error("done = false, want true when there is no appliance")
		}
	})

	t.Run("a live appliance is deleted and waited on", func(t *testing.T) {
		m := testCopyApplianceMigration(t, testCopyAppliance())

		done, err := m.teardownCopyAppliance(testVM)
		if err != nil {
			t.Fatalf("first pass: %v", err)
		}
		if done {
			t.Error("done = true on the pass that issued the delete")
		}

		// The finalizer keeps it around, which is what the next pass reads.
		done, err = m.teardownCopyAppliance(testVM)
		if err != nil {
			t.Fatalf("second pass: %v", err)
		}
		if done {
			t.Error("done = true while teardown is still running")
		}
	})

	t.Run("a finished teardown is done", func(t *testing.T) {
		terminating := testCopyAppliance()
		now := meta.Now()
		terminating.DeletionTimestamp = &now
		terminating.Status.Phase = api.PhaseTeardownCompleted
		m := testCopyApplianceMigration(t, terminating)

		done, err := m.teardownCopyAppliance(testVM)
		if err != nil {
			t.Fatalf("teardownCopyAppliance: %v", err)
		}
		if !done {
			t.Error("done = false, want true once teardown completed")
		}
	})
}

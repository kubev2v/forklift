package copyappliance

import (
	"context"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

func teardownFor(appliance *api.CopyAppliance) *TeardownRunner {
	return &TeardownRunner{context: &ApplianceContext{Appliance: appliance, Log: testLog()}}
}

// An appliance whose teardown never reaches TeardownCompleted keeps its
// finalizer forever, so begin has to have an answer for every state a deploy
// can have left behind.
func TestTeardownBegin(t *testing.T) {
	tests := []struct {
		name      string
		moRef     string
		taskRef   string
		phase     string
		wantPhase string
	}{
		{
			name:      "teardown is already complete when no VM was recorded",
			phase:     api.PhaseDeployFailed,
			wantPhase: api.PhaseTeardownCompleted,
		},
		{
			name:      "teardown starts at power off when a VM was recorded",
			moRef:     "vm-42",
			phase:     api.PhaseWaitForClone,
			wantPhase: api.PhasePowerOff,
		},
		{
			name:      "a task left behind by the deploy is not waited on",
			moRef:     "vm-42",
			taskRef:   "task-7",
			phase:     api.PhaseWaitForClone,
			wantPhase: api.PhasePowerOff,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			appliance := testAppliance()
			appliance.Status.MoRef = tc.moRef
			appliance.Status.TaskRef = tc.taskRef
			appliance.Status.Phase = tc.phase

			runner := teardownFor(appliance)
			if err := runner.begin(); err != nil {
				t.Fatalf("begin: %v", err)
			}

			if appliance.Status.Phase != tc.wantPhase {
				t.Errorf("phase = %q, want %q", appliance.Status.Phase, tc.wantPhase)
			}
			if appliance.Status.TaskRef != "" {
				t.Errorf("task reference = %q, want it cleared", appliance.Status.TaskRef)
			}
		})
	}
}

// Run decides for itself whether the appliance is already partway through a
// teardown. Getting that wrong in either direction is a CR that never loses its
// finalizer: one that is never seeded parks on a deploy phase, and one that is
// seeded every pass restarts the power off forever.
func TestTeardownRunSeeds(t *testing.T) {
	t.Run("a phase left behind by a deploy is seeded", func(t *testing.T) {
		appliance := testAppliance()
		appliance.Status.Phase = api.PhaseDeployFailed

		_, err := teardownFor(appliance).Run(context.TODO())

		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		// Nothing was recorded to tear down, so the seeded phase is the last
		// one and the pass runs it.
		if appliance.Status.Phase != api.PhaseTeardownCompleted {
			t.Errorf("phase = %q, want %q", appliance.Status.Phase, api.PhaseTeardownCompleted)
		}
	})

	t.Run("a phase of the teardown pipeline is resumed", func(t *testing.T) {
		appliance := testAppliance()
		appliance.Status.MoRef = "vm-42"
		appliance.Status.Phase = api.PhaseTeardownCompleted

		_, err := teardownFor(appliance).Run(context.TODO())

		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if appliance.Status.Phase != api.PhaseTeardownCompleted {
			t.Errorf("phase = %q, want the teardown left where it was", appliance.Status.Phase)
		}
	})
}

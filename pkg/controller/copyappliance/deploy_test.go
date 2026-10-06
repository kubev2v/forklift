package copyappliance

import (
	"context"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// LoadImage and Configure ran in the reconciler and now run in a setup pod.
// Neither is in the itinerary any more, so an appliance an older controller
// left on one of them has no successor to walk to and would fail as an unknown
// phase.
func TestRunSendsBackAnApplianceLeftOnAnOldPhase(t *testing.T) {
	for _, phase := range []string{api.PhaseLoadImage, api.PhaseConfigure} {
		t.Run(phase, func(t *testing.T) {
			ac := sshContext(t, nil, closedAddr(t))
			ac.Appliance.Status.Phase = phase
			// Set, as it is on an appliance that got as far as Configure. The
			// pod does the load again either way.
			ac.Appliance.Status.ExporterImage = testLoadedImage
			runner := DeployRunner{context: ac}

			_, err := runner.Run(context.TODO())

			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if ac.Appliance.Status.Phase != api.PhaseSetupAppliance {
				t.Errorf("phase = %q, want %q", ac.Appliance.Status.Phase, api.PhaseSetupAppliance)
			}
		})
	}
}

func TestDeployBegin(t *testing.T) {
	t.Run("deploy starts at clone", func(t *testing.T) {
		appliance := testAppliance()
		appliance.Status.TaskRef = "task-7"

		runner := DeployRunner{context: testContext(appliance, "uuid-a")}
		if err := runner.begin(); err != nil {
			t.Fatalf("begin: %v", err)
		}

		if appliance.Status.Phase != api.PhaseCloneVM {
			t.Errorf("phase = %q, want %q", appliance.Status.Phase, api.PhaseCloneVM)
		}
		if appliance.Status.TaskRef != "" {
			t.Errorf("task reference = %q, want it cleared", appliance.Status.TaskRef)
		}
	})

	// A moRef means nothing without the vCenter it was recorded against, and
	// the clone that follows is what produces one.
	t.Run("the connected vCenter is recorded", func(t *testing.T) {
		appliance := testAppliance()

		runner := DeployRunner{context: testContext(appliance, "uuid-a")}
		if err := runner.begin(); err != nil {
			t.Fatalf("begin: %v", err)
		}

		if appliance.Status.VCenterInstanceUUID != "uuid-a" {
			t.Errorf("recorded vCenter = %q, want %q", appliance.Status.VCenterInstanceUUID, "uuid-a")
		}
	})
}

// setupRunner is a deploy parked on PhaseSetupAppliance, with the appliance's
// container image in a registry it can read.
func setupRunner(t *testing.T, pods ...runtime.Object) (runner *DeployRunner, img v1.Image) {
	t.Helper()
	spec, img := pushImage(t, testRegistry(t, nil))
	runner = setupDeployRunner(t, pods...)
	runner.context.Appliance.Spec.ContainerImage = spec
	runner.context.Appliance.Status.Phase = api.PhaseSetupAppliance
	return
}

// The phase is create-pod-and-poll: the image is resolved and recorded once, on
// the pass that creates the pod, and every pass after that reads the pod.
func TestSetupAppliance(t *testing.T) {
	t.Run("a pod is created when there is none", func(t *testing.T) {
		runner, img := setupRunner(t)
		want, err := makeTag(img)
		if err != nil {
			t.Fatalf("makeTag: %v", err)
		}

		done, err := runner.SetupAppliance(context.TODO())
		if err != nil {
			t.Fatalf("SetupAppliance: %v", err)
		}

		if done {
			t.Error("SetupAppliance = true, want the pod to be waited on")
		}
		// The supervisor the pod installs runs this reference, and the export
		// steps read it back, so it has to be recorded whatever the pod does.
		if got := runner.context.Appliance.Status.ExporterImage; got != want.Name() {
			t.Errorf("exporter image = %q, want %q", got, want.Name())
		}
		onlySetupPod(t, runner)
	})

	t.Run("a running pod is waited on rather than replaced", func(t *testing.T) {
		running := aSetupPod(testAppliance(), "appliance-setup-a", core.PodRunning)
		runner, _ := setupRunner(t, running)

		done, err := runner.SetupAppliance(context.TODO())
		if err != nil {
			t.Fatalf("SetupAppliance: %v", err)
		}

		if done {
			t.Error("SetupAppliance = true, want the pod to be waited on")
		}
		if pod := onlySetupPod(t, runner); pod.Name != running.Name {
			t.Errorf("setup pod = %q, want %q", pod.Name, running.Name)
		}
	})

	t.Run("a pod that succeeded advances the phase", func(t *testing.T) {
		runner, _ := setupRunner(t,
			aSetupPod(testAppliance(), "appliance-setup-a", core.PodSucceeded))

		done, err := runner.SetupAppliance(context.TODO())
		if err != nil {
			t.Fatalf("SetupAppliance: %v", err)
		}

		if !done {
			t.Error("SetupAppliance = false, want the step finished")
		}
	})

	// PhaseDeployFailed is where an operator finds out the appliance is never
	// coming up, and what the pod wrote is the only account of why.
	t.Run("a pod that failed fails the deploy", func(t *testing.T) {
		failed := aSetupPod(testAppliance(), "appliance-setup-a", core.PodFailed)
		failed.Status.ContainerStatuses = []core.ContainerStatus{{
			State: core.ContainerState{Terminated: &core.ContainerStateTerminated{
				Message: "the appliance is not answering on SSH"}},
		}}
		runner, _ := setupRunner(t, failed)

		_, err := runner.SetupAppliance(context.TODO())

		if err == nil {
			t.Fatal("SetupAppliance succeeded after the setup pod failed")
		}
		if !errorMentions(t, err, "the appliance is not answering on SSH") {
			t.Errorf("error = %q, want it to carry what the pod reported", err)
		}
		if !errorMentions(t, err, failed.Name) {
			t.Errorf("error = %q, want it to name the pod", err)
		}
	})

	// The pod reads the image, so a spec that resolves to nothing has to fail
	// on the CopyAppliance rather than as a pod that exits non-zero minutes
	// later.
	t.Run("an image that cannot be resolved fails before the pod", func(t *testing.T) {
		runner, _ := setupRunner(t)
		runner.context.Appliance.Spec.ContainerImage =
			testRegistry(t, nil) + "/forklift/copy-appliance:never-pushed"

		_, err := runner.SetupAppliance(context.TODO())

		if err == nil {
			t.Fatal("SetupAppliance succeeded for an image that is not there")
		}
		pod, podErr := runner.setupPod(context.TODO())
		if podErr != nil {
			t.Fatalf("setupPod: %v", podErr)
		}
		if pod != nil {
			t.Errorf("pod = %q, want none", pod.Name)
		}
	})
}

func TestWaitForCloneAcceptsAdoptedMoRef(t *testing.T) {
	appliance := testAppliance()
	appliance.Status.MoRef = "vm-42"
	runner := DeployRunner{context: testContext(appliance, "uuid-a")}

	done, err := runner.WaitForClone(context.TODO())
	if err != nil {
		t.Fatalf("WaitForClone: %v", err)
	}
	if !done {
		t.Error("WaitForClone = false, want true when MoRef is set with no task")
	}
}

package copyappliance

import (
	"context"
	"testing"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
)

// slowTaskDelay is how long the simulator is made to leave a task running. It
// only has to outlast the passes a test drives while the task is in flight.
const slowTaskDelay = time.Second

// slowTasks makes the simulator leave its tasks running for a while. It
// otherwise finishes one before the call that started it returns, so every wait
// step reads as done and no pass ever stops on one — which is why a runner that
// mishandles a wait looks fine here.
//
// The delay is a package global that the goroutine running a task reads, so the
// task the appliance is left waiting on is waited out before it is restored.
func slowTasks(t *testing.T, ctx context.Context, applianceContext *ApplianceContext) {
	t.Helper()
	previous := simulator.TaskDelay
	t.Cleanup(func() {
		awaitTask(t, ctx, applianceContext)
		simulator.TaskDelay = previous
	})
	simulator.TaskDelay = simulator.DelayConfig{
		Delay: int(slowTaskDelay.Milliseconds()),
		// Without this the simulator holds the entity lock across the delay and
		// nothing can read a property until the task has finished.
		MethodDelay: map[string]int{"LockHandoff": 0},
	}
}

// awaitTask blocks until the task the appliance last recorded has finished.
func awaitTask(t *testing.T, ctx context.Context, applianceContext *ApplianceContext) {
	t.Helper()
	deadline := time.Now().Add(slowTaskDelay * 10)
	for {
		done, _, err := applianceContext.WaitForTask(ctx)
		if err != nil {
			t.Errorf("wait for the outstanding task: %v", err)
			return
		}
		if done {
			return
		}
		if time.Now().After(deadline) {
			t.Error("the outstanding task never finished")
			return
		}
		time.Sleep(slowTaskDelay / 20)
	}
}

// simulatedVCenter starts a vcsim instance and returns a client connected to
// it, along with the model, whose registry is how a test reaches behind the
// API to set what the simulator would otherwise never report. The server is
// closed when the test ends.
func simulatedVCenter(t *testing.T) (ctx context.Context, model *simulator.Model, client *govmomi.Client) {
	t.Helper()
	ctx = context.Background()
	model = simulator.VPX()
	err := model.Create()
	if err != nil {
		t.Fatalf("create simulator model: %v", err)
	}
	t.Cleanup(model.Remove)
	server := model.Service.NewServer()
	t.Cleanup(server.Close)
	client, err = govmomi.NewClient(ctx, server.URL, true)
	if err != nil {
		t.Fatalf("connect to simulator: %v", err)
	}
	return
}

// simulatedAppliance adopts one of the simulator's VMs as the appliance VM.
func simulatedAppliance(t *testing.T, ctx context.Context, client *govmomi.Client) (*ApplianceContext, *object.VirtualMachine) {
	t.Helper()
	vm, err := find.NewFinder(client.Client, false).VirtualMachine(ctx, "/DC0/vm/DC0_H0_VM0")
	if err != nil {
		t.Fatalf("find simulated VM: %v", err)
	}
	appliance := testAppliance()
	appliance.Status.MoRef = vm.Reference().Value
	appliance.Status.VCenterInstanceUUID = client.ServiceContent.About.InstanceUuid
	return &ApplianceContext{
		Appliance: appliance,
		VCenter:   client,
		Log:       testLog(),
	}, vm
}

// runTeardown drives the runner the way the reconciler does, one pass at a
// time, until it settles. The bound is what makes a runner that parks on a
// phase forever a failure instead of a hang. When a pass asks to be polled
// (reQ > 0), sleep briefly so an in-flight vSphere task can finish instead of
// burning the pass budget on no-op waits.
func runTeardown(t *testing.T, ctx context.Context, runner TeardownRunner) (phases []string) {
	t.Helper()
	status := &runner.context.Appliance.Status
	for pass := 0; pass < 40; pass++ {
		reQ, err := runner.Run(ctx)
		if err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
		phases = append(phases, status.Phase)
		if status.Phase == api.PhaseTeardownCompleted || status.Phase == api.PhaseTeardownFailed {
			if status.Phase == api.PhaseTeardownCompleted && status.MoRef != "" {
				// TeardownCompleted clears MoRef on the pass that runs it.
				continue
			}
			return
		}
		if reQ > 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	t.Fatalf("teardown did not settle; phases: %v", phases)
	return
}

// The teardown steps are written against what vSphere actually does with a
// power off, a reconfigure that removes disks, and a destroy. A hand-rolled
// fake would assert those assumptions back at us.
func TestTeardownAgainstSimulatedVCenter(t *testing.T) {
	// TaskDelay is a process-wide simulator knob; clear it so a prior test
	// cannot make the happy-path subtest burn its pass budget waiting.
	simulator.TaskDelay = simulator.DelayConfig{}
	t.Cleanup(func() { simulator.TaskDelay = simulator.DelayConfig{} })

	t.Run("teardown runs to completion and leaves no VM behind", func(t *testing.T) {
		ctx, _, client := simulatedVCenter(t)
		applianceContext, vm := simulatedAppliance(t, ctx, client)
		runner := TeardownRunner{context: applianceContext, client: testClient(t)}

		// No begin: an appliance that has never been torn down is seeded by
		// the first pass, which is how the reconciler drives it.
		phases := runTeardown(t, ctx, runner)

		status := applianceContext.Appliance.Status
		if status.Phase != api.PhaseTeardownCompleted {
			t.Fatalf("phase = %q after %v, want %q", status.Phase, phases, api.PhaseTeardownCompleted)
		}
		if status.MoRef != "" || status.TaskRef != "" || status.Addresses != nil {
			t.Errorf("status still names a VM, a task or an address: %+v", status)
		}
		_, err := vm.PowerState(ctx)
		if err == nil {
			t.Error("the appliance VM is still in the inventory")
		}
	})

	t.Run("an appliance with no recorded VM is torn down in a single pass", func(t *testing.T) {
		ctx, _, client := simulatedVCenter(t)
		appliance := testAppliance()
		runner := TeardownRunner{
			context: &ApplianceContext{Appliance: appliance, VCenter: client, Log: testLog()},
			client:  testClient(t),
		}

		if err := runner.begin(); err != nil {
			t.Fatalf("begin: %v", err)
		}
		phases := runTeardown(t, ctx, runner)

		if len(phases) != 1 {
			t.Errorf("took %d passes (%v), want 1", len(phases), phases)
		}
		if appliance.Status.Phase != api.PhaseTeardownCompleted {
			t.Errorf("phase = %q, want %q", appliance.Status.Phase, api.PhaseTeardownCompleted)
		}
	})

	// Teardown used to record the phase the pass started on rather than the one
	// it stopped in. A pass that started the power off and found the task still
	// running recorded api.PhasePowerOff, so the next pass started a second power
	// off and overwrote the reference to the first — which nothing then
	// observed, because an action phase is requeued immediately on the
	// assumption that the pass walks straight through it. Teardown is the
	// deletion path, so that is an appliance whose finalizer is never released.
	t.Run("a pass waiting on a task records the wait, not the step that started it", func(t *testing.T) {
		ctx, _, client := simulatedVCenter(t)
		applianceContext, _ := simulatedAppliance(t, ctx, client)
		slowTasks(t, ctx, applianceContext)
		runner := TeardownRunner{context: applianceContext, client: testClient(t)}
		status := &applianceContext.Appliance.Status

		err := runner.begin()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		_, err = runner.Run(ctx)
		if err != nil {
			t.Fatalf("first pass: %v", err)
		}

		if status.Phase != api.PhaseWaitForPowerOff {
			t.Fatalf("phase = %q, want %q: the pass started the power off and stopped waiting for it",
				status.Phase, api.PhaseWaitForPowerOff)
		}
		started := status.TaskRef
		if started == "" {
			t.Fatal("the pass recorded no task to wait on")
		}

		_, err = runner.Run(ctx)
		if err != nil {
			t.Fatalf("second pass: %v", err)
		}

		if status.Phase != api.PhaseWaitForPowerOff {
			t.Errorf("phase = %q on the second pass, want %q", status.Phase, api.PhaseWaitForPowerOff)
		}
		if status.TaskRef != started {
			t.Errorf("task = %q, want the first pass's %q: the second pass started another power off",
				status.TaskRef, started)
		}
	})

	// A step with nothing to do records no task, and its wait step reads that
	// as done. If any of these started a task anyway, teardown would wait on a
	// power off that never happens.
	t.Run("a step with nothing to do starts no task", func(t *testing.T) {
		ctx, _, client := simulatedVCenter(t)
		applianceContext, vm := simulatedAppliance(t, ctx, client)
		runner := TeardownRunner{context: applianceContext, client: testClient(t)}
		status := &applianceContext.Appliance.Status

		powerOff, err := vm.PowerOff(ctx)
		if err != nil {
			t.Fatalf("power off the simulated VM: %v", err)
		}
		err = powerOff.Wait(ctx)
		if err != nil {
			t.Fatalf("power off the simulated VM: %v", err)
		}

		status.Phase = api.PhasePowerOff
		_, err = runner.execute(ctx)
		if err != nil {
			t.Fatalf("PowerOff: %v", err)
		}
		if status.Phase != api.PhaseWaitForPowerOff {
			t.Fatalf("phase = %q, want %q", status.Phase, api.PhaseWaitForPowerOff)
		}
		if status.TaskRef != "" {
			t.Errorf("a VM that is already off started task %q", status.TaskRef)
		}

		detach, err := applianceContext.DetachDisks(ctx, vm)
		if err != nil {
			t.Fatalf("DetachDisks: %v", err)
		}
		err = detach.Wait(ctx)
		if err != nil {
			t.Fatalf("detach the disks: %v", err)
		}
		status.Phase = api.PhaseDetachDisks
		_, err = runner.execute(ctx)
		if err != nil {
			t.Fatalf("DetachDisks: %v", err)
		}
		if status.Phase != api.PhaseWaitForDetachDisks {
			t.Fatalf("phase = %q, want %q", status.Phase, api.PhaseWaitForDetachDisks)
		}
		if status.TaskRef != "" {
			t.Errorf("a VM with no disks started task %q", status.TaskRef)
		}
	})

	// The VM can be destroyed out from under us between one reconcile and the
	// next. That is the outcome teardown wants, not a failure to report.
	t.Run("a VM that is already gone completes teardown", func(t *testing.T) {
		ctx, _, client := simulatedVCenter(t)
		applianceContext, vm := simulatedAppliance(t, ctx, client)
		runner := TeardownRunner{context: applianceContext, client: testClient(t)}

		// vSphere refuses to destroy a running VM, so the simulator does too.
		powerOff, err := vm.PowerOff(ctx)
		if err != nil {
			t.Fatalf("power off the simulated VM: %v", err)
		}
		err = powerOff.Wait(ctx)
		if err != nil {
			t.Fatalf("power off the simulated VM: %v", err)
		}
		destroy, err := vm.Destroy(ctx)
		if err != nil {
			t.Fatalf("destroy the simulated VM: %v", err)
		}
		err = destroy.Wait(ctx)
		if err != nil {
			t.Fatalf("destroy the simulated VM: %v", err)
		}

		if err := runner.begin(); err != nil {
			t.Fatalf("begin: %v", err)
		}
		phases := runTeardown(t, ctx, runner)

		if applianceContext.Appliance.Status.Phase != api.PhaseTeardownCompleted {
			t.Errorf("phase = %q after %v, want %q",
				applianceContext.Appliance.Status.Phase, phases, api.PhaseTeardownCompleted)
		}
	})

	// A moRef recorded against another vCenter names some unrelated VM here.
	// Teardown must refuse rather than destroy it.
	t.Run("a VM recorded against another vCenter is not destroyed", func(t *testing.T) {
		ctx, _, client := simulatedVCenter(t)
		applianceContext, vm := simulatedAppliance(t, ctx, client)
		applianceContext.Appliance.Status.VCenterInstanceUUID = "some-other-vcenter"
		runner := TeardownRunner{context: applianceContext, client: testClient(t)}

		if err := runner.begin(); err != nil {
			t.Fatalf("begin: %v", err)
		}
		_, err := runner.Run(ctx)

		if err == nil {
			t.Fatal("teardown accepted a moRef from another vCenter")
		}
		state, err := vm.PowerState(ctx)
		if err != nil {
			t.Fatalf("the VM was destroyed: %v", err)
		}
		if state != types.VirtualMachinePowerStatePoweredOn {
			t.Errorf("power state = %v, want the VM left alone", state)
		}
	})
}

package vsphere

import (
	"context"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

var testLog = logging.WithName("vsphere-client-test")

var _ = Describe("vsphere client tests", func() {
	var client *Client
	var inventory *mockInventory

	setup := func(fn func(context.Context, *vim25.Client)) {
		simulator.Run(func(ctx context.Context, vimClient *vim25.Client) error {
			govmomiClient := &govmomi.Client{
				SessionManager: session.NewManager(vimClient),
				Client:         vimClient,
			}

			// Find the default VM in the simulator
			searchIndex := object.NewSearchIndex(vimClient)
			vm, err := searchIndex.FindByInventoryPath(ctx, "/DC0/vm/DC0_C0_RP0_VM0")
			Expect(err).NotTo(HaveOccurred())
			vmObj := vm.(*object.VirtualMachine)

			var mo mo.VirtualMachine
			err = vmObj.Properties(ctx, vmObj.Reference(), []string{"config.uuid"}, &mo)
			Expect(err).NotTo(HaveOccurred())
			vmUUID := mo.Config.Uuid

			inventory = &mockInventory{
				vm: model.VM{
					VM1: model.VM1{
						VM0: model.VM0{
							ID:   "vm-16", // Default simulator VM ID
							Name: "DC0_C0_RP0_VM0",
						},
					},
					UUID: vmUUID,
				},
			}

			client = &Client{
				Context: &plancontext.Context{
					Source: plancontext.Source{
						Provider:  &v1beta1.Provider{},
						Secret:    &core.Secret{},
						Inventory: inventory,
					},
					Plan: &v1beta1.Plan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-plan",
							Namespace: "test-namespace",
						},
					},
					Log: testLog,
				},
				client: govmomiClient,
			}

			fn(ctx, vimClient)
			return nil
		})
	}

	It("should get the correct power state for a powered on VM", func() {
		setup(func(ctx context.Context, c *vim25.Client) {
			state, err := client.PowerState(ref.Ref{ID: "vm-16"})
			Expect(err).NotTo(HaveOccurred())
			Expect(state).To(Equal(planapi.VMPowerStateOn))
		})
	})

	It("should power off a VM", func() {
		setup(func(ctx context.Context, c *vim25.Client) {
			// The simulator's ShutdownGuest works without guest tools.
			err := client.PowerOff(ref.Ref{ID: "vm-16"})
			Expect(err).NotTo(HaveOccurred())

			poweredOff, err := client.PoweredOff(ref.Ref{ID: "vm-16"})
			Expect(err).NotTo(HaveOccurred())
			Expect(poweredOff).To(BeTrue())
		})
	})

	It("should power on a VM", func() {
		setup(func(ctx context.Context, c *vim25.Client) {
			// First, power it off.
			err := client.PowerOff(ref.Ref{ID: "vm-16"})
			Expect(err).NotTo(HaveOccurred())

			// Then power it on.
			err = client.PowerOn(ref.Ref{ID: "vm-16"})
			Expect(err).NotTo(HaveOccurred())

			// Check state.
			state, err := client.PowerState(ref.Ref{ID: "vm-16"})
			Expect(err).NotTo(HaveOccurred())
			Expect(state).To(Equal(planapi.VMPowerStateOn))
		})
	})

	It("should create and remove a snapshot", func() {
		setup(func(ctx context.Context, c *vim25.Client) {
			snapshotID, taskID, err := client.CreateSnapshot(ref.Ref{ID: "vm-16"}, func() (map[string]*v1beta1.Host, error) {
				return map[string]*v1beta1.Host{}, nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(snapshotID).To(BeEmpty())
			Expect(taskID).NotTo(BeEmpty())

			task := object.NewTask(c, types.ManagedObjectReference{Type: "Task", Value: taskID})
			err = task.Wait(ctx)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.RemoveSnapshot(ref.Ref{ID: "vm-16"}, snapshotName, func() (map[string]*v1beta1.Host, error) {
				return map[string]*v1beta1.Host{}, nil
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should set checkpoints", func() {
		setup(func(ctx context.Context, c *vim25.Client) {
			err := client.SetCheckpoints(ref.Ref{}, []planapi.Precopy{
				{Snapshot: "snap1"},
				{Snapshot: "snap2"},
			}, []cdi.DataVolume{}, false, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should finalize", func() {
		setup(func(ctx context.Context, c *vim25.Client) {
			client.Finalize([]*planapi.VMStatus{}, "")
		})
	})

	It("should run pre-transfer actions", func() {
		setup(func(ctx context.Context, c *vim25.Client) {
			ready, err := client.PreTransferActions(ref.Ref{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ready).To(BeTrue())
		})
	})

	It("should get snapshot deltas", func() {
		setup(func(ctx context.Context, c *vim25.Client) {
			_, err := client.GetSnapshotDeltas(ref.Ref{ID: "vm-16"}, "", func() (map[string]*v1beta1.Host, error) {
				return map[string]*v1beta1.Host{}, nil
			})
			Expect(err).To(HaveOccurred())
		})
	})

	It("should check if a snapshot is removed", func() {
		setup(func(ctx context.Context, c *vim25.Client) {
			_, err := client.CheckSnapshotRemove(ref.Ref{ID: "vm-16"}, planapi.Precopy{}, func() (map[string]*v1beta1.Host, error) {
				return map[string]*v1beta1.Host{}, nil
			})
			Expect(err).To(HaveOccurred())
		})
	})

	It("should check if a snapshot is ready", func() {
		setup(func(ctx context.Context, c *vim25.Client) {
			_, _, err := client.CheckSnapshotReady(ref.Ref{ID: "vm-16"}, planapi.Precopy{}, func() (map[string]*v1beta1.Host, error) {
				return map[string]*v1beta1.Host{}, nil
			})
			Expect(err).To(HaveOccurred())
		})
	})

	It("should detach disks", func() {
		setup(func(ctx context.Context, c *vim25.Client) {
			err := client.DetachDisks(ref.Ref{})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("checkTaskStatus", func() {
		It("should return ready for success state", func() {
			taskInfo := &types.TaskInfo{State: types.TaskInfoStateSuccess}
			ready, err := client.checkTaskStatus(taskInfo)
			Expect(err).NotTo(HaveOccurred())
			Expect(ready).To(BeTrue())
		})

		It("should return an error for error state", func() {
			taskInfo := &types.TaskInfo{
				State: types.TaskInfoStateError,
				Error: &types.LocalizedMethodFault{
					LocalizedMessage: "task failed",
				},
			}
			ready, err := client.checkTaskStatus(taskInfo)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("task failed"))
			Expect(ready).To(BeFalse())
		})

		It("should return not ready for other states", func() {
			taskInfo := &types.TaskInfo{State: types.TaskInfoStateQueued}
			ready, err := client.checkTaskStatus(taskInfo)
			Expect(err).NotTo(HaveOccurred())
			Expect(ready).To(BeFalse())

			taskInfo = &types.TaskInfo{State: types.TaskInfoStateRunning}
			ready, err = client.checkTaskStatus(taskInfo)
			Expect(err).NotTo(HaveOccurred())
			Expect(ready).To(BeFalse())
		})
	})

	It("should close the client connection", func() {
		setup(func(ctx context.Context, c *vim25.Client) {
			Expect(client.client).NotTo(BeNil())
			client.Close()
			Expect(client.client).To(BeNil())
		})
	})
})

package migrator

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EC2 Migrator Reset", func() {
	newMigrator := func() *Migrator {
		ec2Type := api.EC2
		return &Migrator{
			Context: &plancontext.Context{
				Plan: &api.Plan{},
				Source: plancontext.Source{
					Provider: &api.Provider{Spec: api.ProviderSpec{Type: &ec2Type}},
				},
			},
			log: logging.WithName("test"),
		}
	}

	It("should reset VM to PhaseStarted, clear error, and remove conditions", func() {
		m := newMigrator()
		vm := &planapi.VMStatus{
			VM: planapi.VM{Ref: ref.Ref{ID: "i-456"}},
		}
		vm.Phase = PhaseCreateVolumes
		vm.Error = &planapi.Error{Reasons: []string{"snapshot timeout"}}
		vm.SetCondition(libcnd.Condition{Type: api.ConditionCanceled, Status: "True"})
		vm.SetCondition(libcnd.Condition{Type: api.ConditionFailed, Status: "True"})

		pipeline := []*planapi.Step{
			{Task: planapi.Task{Name: PhaseCreateSnapshots}},
			{Task: planapi.Task{Name: api.PhaseCompleted}},
		}
		m.Reset(vm, pipeline)

		Expect(vm.Phase).To(Equal(api.PhaseStarted))
		Expect(vm.Error).To(BeNil())
		Expect(vm.Pipeline).To(Equal(pipeline))
		Expect(vm.HasCondition(api.ConditionCanceled)).To(BeFalse())
		Expect(vm.HasCondition(api.ConditionFailed)).To(BeFalse())
	})

	It("should preserve EC2-specific behavior (no DisksCopied reset)", func() {
		m := newMigrator()
		vm := &planapi.VMStatus{
			VM: planapi.VM{Ref: ref.Ref{ID: "i-789"}},
		}
		vm.Phase = PhaseWaitForVolumes
		vm.DisksCopied = true

		pipeline := []*planapi.Step{
			{Task: planapi.Task{Name: PhaseCreateSnapshots}},
		}
		m.Reset(vm, pipeline)

		Expect(vm.Phase).To(Equal(api.PhaseStarted))
		Expect(vm.DisksCopied).To(BeTrue(), "EC2 Reset does not clear DisksCopied")
	})
})

var _ = Describe("EC2 Itinerary", func() {
	newMigrator := func() *Migrator {
		ec2Type := api.EC2
		return &Migrator{
			Context: &plancontext.Context{
				Plan: &api.Plan{},
				Source: plancontext.Source{
					Provider: &api.Provider{Spec: api.ProviderSpec{Type: &ec2Type}},
				},
			},
			log: logging.WithName("test"),
		}
	}

	Describe("Itinerary selection", func() {
		It("should return cold itinerary", func() {
			m := newMigrator()
			vm := planapi.VM{Ref: ref.Ref{ID: "i-123"}}
			itr := m.Itinerary(vm)
			Expect(itr.Name).To(Equal("EC2 Cold Migration"))
		})

		It("cold itinerary should include all phases", func() {
			m := newMigrator()
			vm := planapi.VM{Ref: ref.Ref{ID: "i-123"}}
			itr := m.Itinerary(vm)

			phaseNames := make(map[string]bool)
			for _, step := range itr.Pipeline {
				phaseNames[step.Name] = true
			}
			Expect(phaseNames).To(HaveKey(PhaseCreateSnapshots))
			Expect(phaseNames).To(HaveKey(PhaseWaitForSnapshots))
			Expect(phaseNames).To(HaveKey(PhaseCreateVolumes))
			Expect(phaseNames).To(HaveKey(PhaseWaitForVolumes))
			Expect(phaseNames).To(HaveKey(PhaseCreatePVsAndPVCs))
			Expect(phaseNames).To(HaveKey(PhaseRemoveSnapshots))
			Expect(phaseNames).To(HaveKey(api.PhaseCreateGuestConversionPod))
			Expect(phaseNames).To(HaveKey(api.PhaseConvertGuest))
			Expect(phaseNames).To(HaveKey(api.PhaseCreateVM))
			Expect(phaseNames).To(HaveKey(api.PhaseCompleted))
		})
	})
})

package migrator

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

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

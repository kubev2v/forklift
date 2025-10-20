package ocp

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cnv "kubevirt.io/api/core/v1"
)

func TestOCPLiveMigrator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OCP Live Migrator Suite")
}

var _ = Describe("Builder", func() {
	var (
		builder *Builder
		target  *cnv.VirtualMachine
	)

	Context("mapNetworks for a source network with a separately specified namespace", func() {
		BeforeEach(func() {
			source := api.NetworkPair{
				Source: ref.Ref{
					Namespace: "source",
					Name:      "net-attach-def",
				},
				Destination: api.DestinationNetwork{
					Namespace: "target",
					Name:      "net-attach-target",
				},
			}
			// it shouldn't get confused by an NAD in
			// an unrelated namespace that happens to
			// have the same name.
			unrelated := api.NetworkPair{
				Source: ref.Ref{
					Namespace: "unrelated",
					Name:      "net-attach-def",
				},
				Destination: api.DestinationNetwork{
					Namespace: "target",
					Name:      "unrelated",
				},
			}
			builder = makeBuilder(source, unrelated)
		})

		It("Remaps an unqualified network", func() {
			target = makeVM("net-attach-def")
			builder.mapNetworks("source", target)
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("target/net-attach-target"))
		})

		It("Remaps a namespace-qualified network", func() {
			target = makeVM("source/net-attach-def")
			builder.mapNetworks("source", target)
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("target/net-attach-target"))
		})
	})

	Context("mapNetworks for a source network with a namespaced name", func() {
		BeforeEach(func() {
			source := api.NetworkPair{
				Source: ref.Ref{
					Namespace: "",
					Name:      "source/net-attach-def",
				},
				Destination: api.DestinationNetwork{
					Namespace: "target",
					Name:      "net-attach-target",
				},
			}
			// it shouldn't get confused by an NAD in
			// an unrelated namespace that happens to
			// have the same name.
			unrelated := api.NetworkPair{
				Source: ref.Ref{
					Namespace: "unrelated",
					Name:      "net-attach-def",
				},
				Destination: api.DestinationNetwork{
					Namespace: "target",
					Name:      "unrelated",
				},
			}
			builder = makeBuilder(source, unrelated)
		})

		It("Remaps an unqualified network", func() {
			target = makeVM("net-attach-def")
			builder.mapNetworks("source", target)
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("target/net-attach-target"))
		})

		It("Remaps a namespace-qualified network", func() {
			target = makeVM("source/net-attach-def")
			builder.mapNetworks("source", target)
			Expect(target.Spec.Template.Spec.Networks[0].Multus.NetworkName).To(Equal("target/net-attach-target"))
		})
	})
})

func makeBuilder(networkPairs ...api.NetworkPair) *Builder {
	b := &Builder{}
	b.Context = &plancontext.Context{}
	b.Context.Map.Network = &api.NetworkMap{
		Spec: api.NetworkMapSpec{
			Map: networkPairs,
		},
	}
	return b
}

func makeVM(networkName string) *cnv.VirtualMachine {
	return &cnv.VirtualMachine{
		Spec: cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				Spec: cnv.VirtualMachineInstanceSpec{
					Networks: []cnv.Network{
						// Multus network that should be remapped
						{
							Name: "net-multus",
							NetworkSource: cnv.NetworkSource{
								Multus: &cnv.MultusNetwork{NetworkName: networkName},
							},
						},
					},
				},
			},
		},
	}
}

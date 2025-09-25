package ocp

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFinder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OCP Finder Suite")
}

var _ = Describe("Finder", func() {
	var (
		finder Finder
	)

	Context("resolve()", func() {
		It("Resolves the namespace and name from a fully specified Ref", func() {
			ref := ref.Ref{
				Namespace: "default",
				Name:      "test",
			}
			namespace, name := finder.resolve(ref)
			Expect(namespace).To(Equal("default"))
			Expect(name).To(Equal("test"))
		})

		It("Resolves the namespace and name from a Ref with a namespaced name", func() {
			ref := ref.Ref{
				Namespace: "",
				Name:      "default/test",
			}
			namespace, name := finder.resolve(ref)
			Expect(namespace).To(Equal("default"))
			Expect(name).To(Equal("test"))
		})
	})
})

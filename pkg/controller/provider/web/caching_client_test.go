package web

import (
	"errors"
	"testing"

	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	webvsphere "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	gomega "github.com/onsi/gomega"
)

var errNotImplemented = errors.New("not implemented")

type stubInventory struct {
	findCalls int
	vmCalls   int
	listCalls int
}

func (s *stubInventory) Finder() base.Finder               { return nil }
func (s *stubInventory) Get(_ interface{}, _ string) error { return nil }
func (s *stubInventory) Watch(_ interface{}, _ EventHandler) (*Watch, error) {
	return nil, errNotImplemented
}
func (s *stubInventory) Find(resource interface{}, ref base.Ref) error {
	s.findCalls++
	vm, ok := resource.(*webvsphere.VM)
	if !ok {
		return nil
	}
	vm.ID = ref.ID
	vm.Name = ref.Name
	vm.NICs = []vsphere.NIC{{MAC: "00:11:22:33:44:55"}}
	return nil
}
func (s *stubInventory) VM(ref *base.Ref) (interface{}, error) {
	s.vmCalls++
	vm := &webvsphere.VM{}
	vm.ID = ref.ID
	vm.Name = ref.Name
	return vm, nil
}
func (s *stubInventory) Workload(ref *base.Ref) (interface{}, error) {
	return nil, errNotImplemented
}
func (s *stubInventory) Network(ref *base.Ref) (interface{}, error) {
	return nil, errNotImplemented
}
func (s *stubInventory) Storage(ref *base.Ref) (interface{}, error) {
	return nil, errNotImplemented
}
func (s *stubInventory) Host(ref *base.Ref) (interface{}, error) {
	return nil, errNotImplemented
}
func (s *stubInventory) List(list interface{}, _ ...base.Param) error {
	s.listCalls++
	items, ok := list.(*[]webvsphere.VM)
	if !ok {
		return nil
	}
	dest := webvsphere.VM{}
	dest.ID = "dest-1"
	dest.Name = "dest-vm"
	*items = []webvsphere.VM{dest}
	return nil
}

func TestCachingClientFindAndList(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	inner := &stubInventory{}
	client := NewCachingClient(inner)
	ref := base.Ref{ID: "vm-1", Name: "source-vm"}

	vm := &webvsphere.VM{}
	g.Expect(client.Find(vm, ref)).To(gomega.Succeed())
	g.Expect(client.Find(&webvsphere.VM{}, ref)).To(gomega.Succeed())
	g.Expect(inner.findCalls).To(gomega.Equal(1))

	cached, ok := client.(*CachingClient)
	g.Expect(ok).To(gomega.BeTrue())
	g.Expect(cached.Stats().FindHits).To(gomega.Equal(1))

	var list []webvsphere.VM
	g.Expect(client.List(&list, base.Param{Key: base.DetailParam, Value: "all"})).To(gomega.Succeed())
	g.Expect(client.List(&list, base.Param{Key: base.DetailParam, Value: "all"})).To(gomega.Succeed())
	g.Expect(inner.listCalls).To(gomega.Equal(1))
	g.Expect(cached.Stats().ListHits).To(gomega.Equal(1))
}

func TestCachingClientVMPreservesType(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	inner := &stubInventory{}
	client := NewCachingClient(inner)
	ref := base.Ref{ID: "vm-1", Name: "source-vm"}

	first, err := client.VM(&ref)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	_, ok := first.(*webvsphere.VM)
	g.Expect(ok).To(gomega.BeTrue())

	second, err := client.VM(&ref)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	_, ok = second.(*webvsphere.VM)
	g.Expect(ok).To(gomega.BeTrue())
	g.Expect(inner.vmCalls).To(gomega.Equal(1))
}

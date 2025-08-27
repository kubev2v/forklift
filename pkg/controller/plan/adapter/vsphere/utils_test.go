package vsphere

import (
	"context"
	"fmt"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type failingClient struct {
	client.Client
}

func (c *failingClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return fmt.Errorf("failed to list PVCs")
}

var _ = Describe("vsphere utils tests", func() {

	Describe("getDisksPvc", func() {
		It("should return the correct PVC for a given disk", func() {
			disk := vsphere.Disk{File: "[datastore1] vm/disk1.vmdk"}
			pvc1 := &core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"forklift.konveyor.io/disk-source": "[datastore1] vm/disk1.vmdk"},
				},
			}
			pvc2 := &core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"forklift.konveyor.io/disk-source": "[datastore1] vm/disk2.vmdk"},
				},
			}
			pvcs := []*core.PersistentVolumeClaim{pvc1, pvc2}

			Expect(getDisksPvc(disk, pvcs, false)).To(Equal(pvc1))
		})

		It("should return nil if no matching PVC is found", func() {
			disk := vsphere.Disk{File: "[datastore1] vm/disk3.vmdk"}
			pvc1 := &core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"forklift.konveyor.io/disk-source": "[datastore1] vm/disk1.vmdk"},
				},
			}
			pvcs := []*core.PersistentVolumeClaim{pvc1}

			Expect(getDisksPvc(disk, pvcs, false)).To(BeNil())
		})

		It("should return the correct PVC for a given disk in a warm migration", func() {
			disk := vsphere.Disk{File: "[datastore1] vm/disk1-000001.vmdk"}
			pvc1 := &core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"forklift.konveyor.io/disk-source": "[datastore1] vm/disk1.vmdk"},
				},
			}
			pvcs := []*core.PersistentVolumeClaim{pvc1}

			Expect(getDisksPvc(disk, pvcs, true)).To(Equal(pvc1))
		})
	})

	Describe("trimBackingFileName", func() {
		It("should trim snapshot suffix from disk backing file name", func() {
			fileName := "[datastore13] my-vm/disk-name-000015.vmdk"
			expected := "[datastore13] my-vm/disk-name.vmdk"
			Expect(trimBackingFileName(fileName)).To(Equal(expected))
		})
	})

	Describe("stringifyWithQuotes", func() {
		It("should format a slice of strings with quotes and commas", func() {
			s := []string{"disk1", "disk2", "disk3"}
			expected := "'disk1', 'disk2', 'disk3'"
			Expect(stringifyWithQuotes(s)).To(Equal(expected))
		})
	})

	Describe("listShareablePVCs", func() {
		It("should list all shareable PVCs in the target namespace", func() {
			pvc1 := &core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc1",
					Namespace: "test",
					Labels:    map[string]string{Shareable: "true"},
				},
			}
			pvc2 := &core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc2",
					Namespace: "test",
				},
			}
			scheme := runtime.NewScheme()
			_ = core.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(pvc1, pvc2).Build()

			pvcs, err := listShareablePVCs(client, "test")
			Expect(err).NotTo(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			Expect(pvcs[0].Name).To(Equal("pvc1"))
		})
	})

	Describe("findSharedPVCs", func() {
		It("should find shared PVCs and missing disk PVCs for a VM", func() {
			vm := &model.VM{
				VM1: model.VM1{
					Disks: []vsphere.Disk{
						{File: "[datastore1] vm/shared.vmdk", Shared: true},
						{File: "[datastore1] vm/not-shared.vmdk", Shared: false},
						{File: "[datastore1] vm/missing.vmdk", Shared: true},
					},
				},
			}
			pvc := &core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "shared-pvc",
					Namespace:   "test",
					Labels:      map[string]string{Shareable: "true"},
					Annotations: map[string]string{"forklift.konveyor.io/disk-source": "[datastore1] vm/shared.vmdk"},
				},
			}
			scheme := runtime.NewScheme()
			_ = core.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(pvc).Build()

			pvcs, missing, err := findSharedPVCs(client, vm, "test")
			Expect(err).NotTo(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			Expect(pvcs[0].Name).To(Equal("shared-pvc"))
			Expect(missing).To(HaveLen(1))
			Expect(missing[0].File).To(Equal("[datastore1] vm/missing.vmdk"))
		})

		It("should return an error if listing PVCs fails", func() {
			vm := &model.VM{}
			client := &failingClient{}

			_, _, err := findSharedPVCs(client, vm, "test")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("useCompatibilityModeBus", func() {
		It("should return true if SkipGuestConversion and UseCompatibilityMode are true", func() {
			plan := &v1beta1.Plan{
				Spec: v1beta1.PlanSpec{
					SkipGuestConversion:  true,
					UseCompatibilityMode: true,
				},
			}
			Expect(useCompatibilityModeBus(plan)).To(BeTrue())
		})

		It("should return false if SkipGuestConversion is false", func() {
			plan := &v1beta1.Plan{
				Spec: v1beta1.PlanSpec{
					SkipGuestConversion:  false,
					UseCompatibilityMode: true,
				},
			}
			Expect(useCompatibilityModeBus(plan)).To(BeFalse())
		})

		It("should return false if UseCompatibilityMode is false", func() {
			plan := &v1beta1.Plan{
				Spec: v1beta1.PlanSpec{
					SkipGuestConversion:  true,
					UseCompatibilityMode: false,
				},
			}
			Expect(useCompatibilityModeBus(plan)).To(BeFalse())
		})
	})
})

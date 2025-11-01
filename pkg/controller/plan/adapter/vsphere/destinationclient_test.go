package vsphere

import (
	"context"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var destClientLog = logging.WithName("destination-client-test")

var _ = Describe("DestinationClient", func() {
	Describe("DeletePopulatorDataSource", func() {
		It("should delete all populator CRs successfully", func() {
			// Setup
			populator1 := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: meta.ObjectMeta{
					Name:      "populator-1",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
					},
				},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmdkPath: "/vmdk/path1.vmdk",
				},
			}
			populator2 := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: meta.ObjectMeta{
					Name:      "populator-2",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
					},
				},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmdkPath: "/vmdk/path2.vmdk",
				},
			}

			destClient := createDestinationClient(populator1, populator2)
			vmStatus := &planapi.VMStatus{
				NewName: "test-vm",
			}

			// Execute
			err := destClient.DeletePopulatorDataSource(vmStatus)

			// Assert
			Expect(err).NotTo(HaveOccurred())

			// Verify all populators are deleted
			populatorList := &v1beta1.VSphereXcopyVolumePopulatorList{}
			err = destClient.Destination.Client.List(context.TODO(), populatorList, client.InNamespace("test"))
			Expect(err).NotTo(HaveOccurred())
			Expect(populatorList.Items).To(BeEmpty())
		})

		It("should succeed when no populator CRs exist", func() {
			// Setup
			destClient := createDestinationClient()
			vmStatus := &planapi.VMStatus{
				NewName: "test-vm",
			}

			// Execute
			err := destClient.DeletePopulatorDataSource(vmStatus)

			// Assert
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("getPopulatorCrList", func() {
		It("should return only CRs matching the migration UID", func() {
			// Setup
			populator1 := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: meta.ObjectMeta{
					Name:      "populator-match-1",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
					},
				},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmdkPath: "/vmdk/path1.vmdk",
				},
			}
			populator2 := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: meta.ObjectMeta{
					Name:      "populator-match-2",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
					},
				},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmdkPath: "/vmdk/path2.vmdk",
				},
			}
			populatorDifferent := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: meta.ObjectMeta{
					Name:      "populator-different-migration",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "different-uid",
					},
				},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmdkPath: "/vmdk/path3.vmdk",
				},
			}

			destClient := createDestinationClient(populator1, populator2, populatorDifferent)

			// Execute
			populatorList, err := destClient.getPopulatorCrList()

			// Assert
			Expect(err).NotTo(HaveOccurred())
			Expect(populatorList.Items).To(HaveLen(2))
			for _, pop := range populatorList.Items {
				Expect(pop.Labels["migration"]).To(Equal("123"))
			}
		})

		It("should return empty list when no populator CRs exist", func() {
			// Setup
			destClient := createDestinationClient()

			// Execute
			populatorList, err := destClient.getPopulatorCrList()

			// Assert
			Expect(err).NotTo(HaveOccurred())
			Expect(populatorList.Items).To(BeEmpty())
		})
	})

	Describe("DeleteObject", func() {
		It("should delete the object successfully", func() {
			// Setup
			populator := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-populator",
					Namespace: "test",
				},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmdkPath: "/vmdk/test.vmdk",
				},
			}

			destClient := createDestinationClient(populator)
			vmStatus := &planapi.VMStatus{
				NewName: "test-vm",
			}

			// Execute
			err := destClient.DeleteObject(populator, vmStatus, "Deleted test object", "VSphereXcopyVolumePopulator")

			// Assert
			Expect(err).NotTo(HaveOccurred())

			// Verify object is deleted
			deletedPop := &v1beta1.VSphereXcopyVolumePopulator{}
			err = destClient.Destination.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "test-populator",
				Namespace: "test",
			}, deletedPop)
			Expect(k8serr.IsNotFound(err)).To(BeTrue())
		})

		It("should succeed without error when object does not exist", func() {
			// Setup
			destClient := createDestinationClient()
			populator := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: meta.ObjectMeta{
					Name:      "nonexistent-populator",
					Namespace: "test",
				},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmdkPath: "/vmdk/test.vmdk",
				},
			}
			vmStatus := &planapi.VMStatus{
				NewName: "test-vm",
			}

			// Execute
			err := destClient.DeleteObject(populator, vmStatus, "Deleted test object", "VSphereXcopyVolumePopulator")

			// Assert
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("findPVCByCR", func() {
		It("should return the matching PVC", func() {
			// Setup
			pvc := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
						"vmdkKey":   "disk-1",
					},
					Annotations: map[string]string{
						"copy-offload": "/vmdk/test.vmdk",
					},
				},
				Spec: core.PersistentVolumeClaimSpec{
					AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
				},
			}
			populator := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-populator",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
						"vmdkKey":   "disk-1",
					},
				},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmdkPath: "/vmdk/test.vmdk",
				},
			}

			destClient := createDestinationClient(pvc, populator)

			// Execute
			foundPVC, err := destClient.findPVCByCR(populator)

			// Assert
			Expect(err).NotTo(HaveOccurred())
			Expect(foundPVC).NotTo(BeNil())
			Expect(foundPVC.Name).To(Equal("test-pvc"))
			Expect(foundPVC.Labels["vmdkKey"]).To(Equal("disk-1"))
		})

		It("should return an error when no matching PVC exists", func() {
			// Setup
			populator := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-populator",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
						"vmdkKey":   "disk-1",
					},
				},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmdkPath: "/vmdk/test.vmdk",
				},
			}

			destClient := createDestinationClient(populator)

			// Execute
			foundPVC, err := destClient.findPVCByCR(populator)

			// Assert
			Expect(err).To(HaveOccurred())
			Expect(foundPVC).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("PVC not found"))
		})

		It("should return an error when multiple matching PVCs exist", func() {
			// Setup
			pvc1 := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc-1",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
						"vmdkKey":   "disk-1",
					},
					Annotations: map[string]string{
						"copy-offload": "/vmdk/test.vmdk",
					},
				},
				Spec: core.PersistentVolumeClaimSpec{
					AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
				},
			}
			pvc2 := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc-2",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
						"vmdkKey":   "disk-1",
					},
					Annotations: map[string]string{
						"copy-offload": "/vmdk/test.vmdk",
					},
				},
				Spec: core.PersistentVolumeClaimSpec{
					AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
				},
			}
			populator := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-populator",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
						"vmdkKey":   "disk-1",
					},
				},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmdkPath: "/vmdk/test.vmdk",
				},
			}

			destClient := createDestinationClient(pvc1, pvc2, populator)

			// Execute
			foundPVC, err := destClient.findPVCByCR(populator)

			// Assert
			Expect(err).To(HaveOccurred())
			Expect(foundPVC).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("Multiple PVCs found"))
		})

		It("should not find PVC with different migration UID", func() {
			// Setup
			pvc := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc-different",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "different-migration-uid",
						"vmdkKey":   "disk-1",
					},
				},
				Spec: core.PersistentVolumeClaimSpec{
					AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
				},
			}
			populator := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-populator",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
						"vmdkKey":   "disk-1",
					},
				},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmdkPath: "/vmdk/test.vmdk",
				},
			}

			destClient := createDestinationClient(pvc, populator)

			// Execute
			foundPVC, err := destClient.findPVCByCR(populator)

			// Assert
			Expect(err).To(HaveOccurred())
			Expect(foundPVC).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("PVC not found"))
		})

		It("should not find PVC with different vmdkKey", func() {
			// Setup
			pvc := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc-different-key",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
						"vmdkKey":   "disk-2",
					},
				},
				Spec: core.PersistentVolumeClaimSpec{
					AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
				},
			}
			populator := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-populator",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "123",
						"vmdkKey":   "disk-1",
					},
				},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmdkPath: "/vmdk/test.vmdk",
				},
			}

			destClient := createDestinationClient(pvc, populator)

			// Execute
			foundPVC, err := destClient.findPVCByCR(populator)

			// Assert
			Expect(err).To(HaveOccurred())
			Expect(foundPVC).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("PVC not found"))
		})
	})
})

//nolint:errcheck
func createDestinationClient(objs ...runtime.Object) *DestinationClient {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = core.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	v1beta1.SchemeBuilder.AddToScheme(scheme)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		WithIndex(&core.PersistentVolumeClaim{}, "metadata.annotations.copy-offload", func(obj client.Object) []string {
			pvc := obj.(*core.PersistentVolumeClaim)
			if pvc.Annotations != nil {
				if val, ok := pvc.Annotations["copy-offload"]; ok {
					return []string{val}
				}
			}
			return []string{}
		}).
		Build()

	plan := createPlan()
	migrationUID := k8stypes.UID("123")

	// Set up the migration status with proper snapshot
	migration := &v1beta1.Migration{
		ObjectMeta: meta.ObjectMeta{
			UID: migrationUID,
		},
	}

	// Add migration snapshot to plan status
	plan.Status.Migration.History = []planapi.Snapshot{
		{
			Migration: planapi.SnapshotRef{
				UID: migrationUID,
			},
		},
	}

	return &DestinationClient{
		Context: &plancontext.Context{
			Destination: plancontext.Destination{
				Client: client,
			},
			Plan:      plan,
			Migration: migration,
			Log:       destClientLog,
			Client:    client,
		},
	}
}

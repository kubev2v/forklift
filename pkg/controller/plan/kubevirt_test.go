//nolint:errcheck
package plan

import (
	"context"
	"encoding/json"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var KubeVirtLog = logging.WithName("kubevirt-test")

var _ = ginkgo.Describe("kubevirt tests", func() {
	ginkgo.Describe("getPVCs", func() {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pvc",
				Namespace: "test",
				Labels: map[string]string{
					"migration": "test",
					"vmID":      "test",
				},
			},
		}

		ginkgo.It("should return PVCs", func() {
			kubevirt := createKubeVirt(pvc)
			pvcs, err := kubevirt.getPVCs(ref.Ref{ID: "test"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
		})
	})

	ginkgo.Describe("setTransferNetwork", func() {
		ginkgo.It("should set modern annotation with gateway when route annotation has IP", func() {
			nad := &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nad",
					Namespace: "test-ns",
					Annotations: map[string]string{
						AnnForkliftNetworkRoute: "192.168.1.1",
					},
				},
			}

			kubevirt := createKubeVirtWithTransferNetwork(nad, "test-ns", "test-nad")
			annotations := make(map[string]string)

			err := kubevirt.setTransferNetwork(annotations)
			Expect(err).ToNot(HaveOccurred())

			Expect(annotations).To(HaveKey(AnnTransferNetwork))
			Expect(annotations).ToNot(HaveKey(AnnLegacyTransferNetwork))

			var networks []k8snet.NetworkSelectionElement
			err = json.Unmarshal([]byte(annotations[AnnTransferNetwork]), &networks)
			Expect(err).ToNot(HaveOccurred())
			Expect(networks).To(HaveLen(1))
			Expect(networks[0].Name).To(Equal("test-nad"))
			Expect(networks[0].Namespace).To(Equal("test-ns"))
			Expect(networks[0].GatewayRequest).To(HaveLen(1))
			Expect(networks[0].GatewayRequest[0].String()).To(Equal("192.168.1.1"))
		})

		ginkgo.It("should set modern annotation without gateway when route annotation is 'none'", func() {
			nad := &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nad",
					Namespace: "test-ns",
					Annotations: map[string]string{
						AnnForkliftNetworkRoute: AnnForkliftRouteValueNone,
					},
				},
			}

			kubevirt := createKubeVirtWithTransferNetwork(nad, "test-ns", "test-nad")
			annotations := make(map[string]string)

			err := kubevirt.setTransferNetwork(annotations)
			Expect(err).ToNot(HaveOccurred())

			Expect(annotations).To(HaveKey(AnnTransferNetwork))
			Expect(annotations).ToNot(HaveKey(AnnLegacyTransferNetwork))

			var networks []k8snet.NetworkSelectionElement
			err = json.Unmarshal([]byte(annotations[AnnTransferNetwork]), &networks)
			Expect(err).ToNot(HaveOccurred())
			Expect(networks).To(HaveLen(1))
			Expect(networks[0].Name).To(Equal("test-nad"))
			Expect(networks[0].Namespace).To(Equal("test-ns"))
			Expect(networks[0].GatewayRequest).To(BeEmpty())
		})

		ginkgo.It("should set modern annotation with gateway from IPAM config", func() {
			nadConfig := `{
				"cniVersion": "0.3.1",
				"name": "test-network",
				"type": "ovn-k8s-cni-overlay",
				"ipam": {
					"type": "static",
					"routes": [
						{"dst": "0.0.0.0/0", "gw": "10.0.0.1"}
					]
				}
			}`
			nad := &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nad",
					Namespace: "test-ns",
				},
				Spec: k8snet.NetworkAttachmentDefinitionSpec{
					Config: nadConfig,
				},
			}

			kubevirt := createKubeVirtWithTransferNetwork(nad, "test-ns", "test-nad")
			annotations := make(map[string]string)

			err := kubevirt.setTransferNetwork(annotations)
			Expect(err).ToNot(HaveOccurred())

			Expect(annotations).To(HaveKey(AnnTransferNetwork))
			Expect(annotations).ToNot(HaveKey(AnnLegacyTransferNetwork))

			var networks []k8snet.NetworkSelectionElement
			err = json.Unmarshal([]byte(annotations[AnnTransferNetwork]), &networks)
			Expect(err).ToNot(HaveOccurred())
			Expect(networks).To(HaveLen(1))
			Expect(networks[0].Name).To(Equal("test-nad"))
			Expect(networks[0].Namespace).To(Equal("test-ns"))
			Expect(networks[0].GatewayRequest).To(HaveLen(1))
			Expect(networks[0].GatewayRequest[0].String()).To(Equal("10.0.0.1"))
		})

		ginkgo.It("should prefer gateway from annotation over gateway from IPAM config", func() {
			nadConfig := `{
				"cniVersion": "0.3.1",
				"name": "test-network",
				"type": "ovn-k8s-cni-overlay",
				"ipam": {
					"type": "static",
					"routes": [
						{"dst": "0.0.0.0/0", "gw": "10.0.0.1"}
					]
				}
			}`
			nad := &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nad",
					Namespace: "test-ns",
					Annotations: map[string]string{
						AnnForkliftNetworkRoute: "192.168.1.1",
					},
				},
				Spec: k8snet.NetworkAttachmentDefinitionSpec{
					Config: nadConfig,
				},
			}

			kubevirt := createKubeVirtWithTransferNetwork(nad, "test-ns", "test-nad")
			annotations := make(map[string]string)

			err := kubevirt.setTransferNetwork(annotations)
			Expect(err).ToNot(HaveOccurred())

			Expect(annotations).To(HaveKey(AnnTransferNetwork))
			Expect(annotations).ToNot(HaveKey(AnnLegacyTransferNetwork))

			var networks []k8snet.NetworkSelectionElement
			err = json.Unmarshal([]byte(annotations[AnnTransferNetwork]), &networks)
			Expect(err).ToNot(HaveOccurred())
			Expect(networks).To(HaveLen(1))
			Expect(networks[0].Name).To(Equal("test-nad"))
			Expect(networks[0].Namespace).To(Equal("test-ns"))
			Expect(networks[0].GatewayRequest).To(HaveLen(1))
			Expect(networks[0].GatewayRequest[0].String()).To(Equal("192.168.1.1"))
		})

		ginkgo.It("should fall back to legacy annotation when no route found", func() {
			nad := &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nad",
					Namespace: "test-ns",
				},
			}

			kubevirt := createKubeVirtWithTransferNetwork(nad, "test-ns", "test-nad")
			annotations := make(map[string]string)

			err := kubevirt.setTransferNetwork(annotations)
			Expect(err).ToNot(HaveOccurred())

			Expect(annotations).ToNot(HaveKey(AnnTransferNetwork))
			Expect(annotations).To(HaveKey(AnnLegacyTransferNetwork))
			Expect(annotations[AnnLegacyTransferNetwork]).To(Equal("test-ns/test-nad"))
		})

		ginkgo.It("should return error for invalid IP in route annotation", func() {
			nad := &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nad",
					Namespace: "test-ns",
					Annotations: map[string]string{
						AnnForkliftNetworkRoute: "invalid-ip",
					},
				},
			}

			kubevirt := createKubeVirtWithTransferNetwork(nad, "test-ns", "test-nad")
			annotations := make(map[string]string)

			err := kubevirt.setTransferNetwork(annotations)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not a valid IP address"))
		})
	})

	ginkgo.Describe("Prime PVC cleanup", func() {
		ginkgo.It("should remove finalizers from prime PVC during deletion", func() {
			pvcUID := types.UID("test-pvc-uid-123")
			pvc := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "test",
					UID:       pvcUID,
					Labels: map[string]string{
						"migration": "test-migration",
						"vmID":      "vm-1",
					},
				},
			}
			primePVC := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prime-test-pvc-uid-123",
					Namespace: "test",
					Finalizers: []string{
						"kubernetes.io/pvc-protection",
					},
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				},
			}

			kubevirt := createKubeVirt(primePVC)
			kubevirt.Plan.Spec.TargetNamespace = "test"

			vm := &plan.VMStatus{
				VM: plan.VM{
					Ref: ref.Ref{ID: "vm-1"},
				},
			}

			err := kubevirt.deleteCorrespondingPrimePVC(pvc, vm)
			Expect(err).ToNot(HaveOccurred())

		})

		ginkgo.It("should not error if prime PVC does not exist", func() {
			pvc := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "test",
					UID:       "test-pvc-uid-456",
				},
			}

			kubevirt := createKubeVirt()
			kubevirt.Plan.Spec.TargetNamespace = "test"

			vm := &plan.VMStatus{
				VM: plan.VM{
					Ref: ref.Ref{ID: "test"},
				},
			}

			err := kubevirt.deleteCorrespondingPrimePVC(pvc, vm)
			Expect(err).ToNot(HaveOccurred())
		})

		ginkgo.It("should delete multiple prime PVCs using DeletePrimePVCs", func() {
			// Create multiple PVCs with corresponding prime PVCs
			pvc1 := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vm-disk-0",
					Namespace: "test",
					UID:       "pvc-uid-1",
					Labels: map[string]string{
						"migration": "test-migration",
						"vmID":      "vm-test",
					},
				},
			}
			primePVC1 := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prime-pvc-uid-1",
					Namespace: "test",
					Finalizers: []string{
						"kubernetes.io/pvc-protection",
					},
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				},
			}

			pvc2 := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vm-disk-1",
					Namespace: "test",
					UID:       "pvc-uid-2",
					Labels: map[string]string{
						"migration": "test-migration",
						"vmID":      "vm-test",
					},
				},
			}
			primePVC2 := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prime-pvc-uid-2",
					Namespace: "test",
					Finalizers: []string{
						"kubernetes.io/pvc-protection",
					},
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				},
			}

			kubevirt := createKubeVirt(pvc1, pvc2, primePVC1, primePVC2)
			kubevirt.Plan.Spec.TargetNamespace = "test"

			vm := &plan.VMStatus{
				VM: plan.VM{
					Ref: ref.Ref{ID: "vm-test"},
				},
			}

			err := kubevirt.DeletePrimePVCs(vm)
			Expect(err).ToNot(HaveOccurred())

		})

		ginkgo.It("should preserve target PVCs when deleting prime PVCs", func() {
			// Create target PVC and prime PVC
			targetPVC := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "target-vm-disk-0",
					Namespace: "test",
					UID:       "target-pvc-uid",
					Labels: map[string]string{
						"migration": "test-migration",
						"vmID":      "vm-preserve",
					},
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				},
			}
			primePVC := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prime-target-pvc-uid",
					Namespace: "test",
					Finalizers: []string{
						"kubernetes.io/pvc-protection",
					},
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				},
			}

			kubevirt := createKubeVirt(targetPVC, primePVC)
			kubevirt.Plan.Spec.TargetNamespace = "test"

			vm := &plan.VMStatus{
				VM: plan.VM{
					Ref: ref.Ref{ID: "vm-preserve"},
				},
			}

			err := kubevirt.DeletePrimePVCs(vm)
			Expect(err).ToNot(HaveOccurred())

			// Verify target PVC still exists and is unchanged
			retrievedPVC := &v1.PersistentVolumeClaim{}
			err = kubevirt.Destination.Client.Get(context.TODO(), client.ObjectKey{
				Namespace: "test",
				Name:      "target-vm-disk-0",
			}, retrievedPVC)
			Expect(err).ToNot(HaveOccurred(), "Target PVC should still exist")
			Expect(retrievedPVC.Name).To(Equal("target-vm-disk-0"))
		})
	})
})

func createKubeVirt(objs ...runtime.Object) *KubeVirt {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = k8snet.AddToScheme(scheme)
	v1beta1.SchemeBuilder.AddToScheme(scheme)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
	return &KubeVirt{
		Context: &plancontext.Context{
			Destination: plancontext.Destination{
				Client: client,
			},
			Log:       KubeVirtLog,
			Migration: createMigration(),
			Plan:      createPlanKubevirt(nil),
			Client:    client,
		},
	}
}

func createMigration() *v1beta1.Migration {
	return &v1beta1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			UID:       "test",
		},
	}
}
func createPlanKubevirt(transferNetwork *v1.ObjectReference) *v1beta1.Plan {
	return &v1beta1.Plan{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: v1beta1.PlanSpec{
			Type:            "cold",
			TransferNetwork: transferNetwork,
		},
	}
}

func createKubeVirtWithTransferNetwork(nad *k8snet.NetworkAttachmentDefinition, namespace, name string) *KubeVirt {
	transferNetwork := &v1.ObjectReference{
		Namespace: namespace,
		Name:      name,
	}

	kubevirt := createKubeVirt(nad)
	kubevirt.Plan = createPlanKubevirt(transferNetwork)
	return kubevirt
}

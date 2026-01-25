//nolint:errcheck
package plan

import (
	"encoding/json"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
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

	ginkgo.Describe("ConversionTempStorage plan spec", func() {
		ginkgo.It("should read ConversionTempStorageClass and ConversionTempStorageSize from plan spec", func() {
			plan := createPlanKubevirt(nil)
			plan.Spec.ConversionTempStorageClass = "fast-ssd"
			plan.Spec.ConversionTempStorageSize = "100Gi"

			kubevirt := createKubeVirt()
			kubevirt.Plan = plan

			Expect(kubevirt.Plan.Spec.ConversionTempStorageClass).To(Equal("fast-ssd"))
			Expect(kubevirt.Plan.Spec.ConversionTempStorageSize).To(Equal("100Gi"))
		})

		ginkgo.It("should handle empty ConversionTempStorage fields", func() {
			plan := createPlanKubevirt(nil)
			plan.Spec.ConversionTempStorageClass = ""
			plan.Spec.ConversionTempStorageSize = ""

			kubevirt := createKubeVirt()
			kubevirt.Plan = plan

			Expect(kubevirt.Plan.Spec.ConversionTempStorageClass).To(Equal(""))
			Expect(kubevirt.Plan.Spec.ConversionTempStorageSize).To(Equal(""))
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

var _ = ginkgo.Describe("Duplicate PVC Prevention", func() {
	ginkgo.It("providerStorageName generates deterministic valid DNS1123 names", func() {
		name1 := providerStorageName("ova-store-pvc", "ova-provider", "plan-uid-123", "vm-47")
		name2 := providerStorageName("ova-store-pvc", "ova-provider", "plan-uid-123", "vm-47")
		Expect(name1).To(Equal(name2))

		nameOtherVM := providerStorageName("ova-store-pvc", "ova-provider", "plan-uid-123", "vm-48")
		Expect(name1).NotTo(Equal(nameOtherVM))

		errs := validation.IsDNS1123Label(name1)
		Expect(errs).To(BeEmpty())

		longName := providerStorageName("ova-store-pvc", "very-long-provider-name-exceeds", "very-long-plan-uid-1234567890", "vm-long-id")
		Expect(len(longName)).To(BeNumerically("<=", validation.DNS1123LabelMaxLength))
	})

	ginkgo.It("ResolveAlreadyExists retrieves existing resource on AlreadyExists error", func() {
		existingPVC := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "existing-pvc",
				Namespace: "test",
				Labels:    map[string]string{"app": "forklift"},
			},
		}
		kubevirt := createKubeVirt(existingPVC)

		notFoundErr := k8serr.NewNotFound(schema.GroupResource{Resource: "pvc"}, "test-pvc")
		pvc := &v1.PersistentVolumeClaim{}
		handled, _ := kubevirt.ResolveAlreadyExists(notFoundErr, client.ObjectKey{Name: "test-pvc", Namespace: "test"}, pvc)
		Expect(handled).To(BeFalse())

		alreadyExistsErr := k8serr.NewAlreadyExists(schema.GroupResource{Resource: "persistentvolumeclaims"}, "existing-pvc")
		pvc = &v1.PersistentVolumeClaim{}
		handled, outErr := kubevirt.ResolveAlreadyExists(alreadyExistsErr, client.ObjectKey{Name: "existing-pvc", Namespace: "test"}, pvc)
		Expect(handled).To(BeTrue())
		Expect(outErr).ToNot(HaveOccurred())
		Expect(pvc.Name).To(Equal("existing-pvc"))
	})
})

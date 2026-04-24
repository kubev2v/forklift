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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

	ginkgo.Describe("resolveServiceAccount", func() {
		var savedGlobalSA string

		ginkgo.BeforeEach(func() {
			savedGlobalSA = Settings.Migration.ServiceAccount
		})

		ginkgo.AfterEach(func() {
			Settings.Migration.ServiceAccount = savedGlobalSA
		})

		ginkgo.It("should return plan SA when both plan and global are set", func() {
			Settings.Migration.ServiceAccount = "global-sa"
			p := createPlanKubevirt(nil)
			p.Spec.ServiceAccount = "plan-sa"
			Expect(resolveServiceAccount(p)).To(Equal("plan-sa"))
		})

		ginkgo.It("should return global SA when plan SA is empty", func() {
			Settings.Migration.ServiceAccount = "global-sa"
			p := createPlanKubevirt(nil)
			p.Spec.ServiceAccount = ""
			Expect(resolveServiceAccount(p)).To(Equal("global-sa"))
		})

		ginkgo.It("should return empty string when both are empty", func() {
			Settings.Migration.ServiceAccount = ""
			p := createPlanKubevirt(nil)
			p.Spec.ServiceAccount = ""
			Expect(resolveServiceAccount(p)).To(BeEmpty())
		})

		ginkgo.It("should return plan SA when global is empty", func() {
			Settings.Migration.ServiceAccount = ""
			p := createPlanKubevirt(nil)
			p.Spec.ServiceAccount = "plan-sa"
			Expect(resolveServiceAccount(p)).To(Equal("plan-sa"))
		})
	})

	ginkgo.Describe("dataVolumes CDI SA annotation", func() {
		var savedGlobalSA string

		ginkgo.BeforeEach(func() {
			savedGlobalSA = Settings.Migration.ServiceAccount
		})

		ginkgo.AfterEach(func() {
			Settings.Migration.ServiceAccount = savedGlobalSA
		})

		ginkgo.It("should set CDI SA annotation when plan SA is set", func() {
			Settings.Migration.ServiceAccount = ""
			p := createPlanKubevirt(nil)
			p.Spec.ServiceAccount = "plan-sa"
			annotations := make(map[string]string)
			if sa := resolveServiceAccount(p); sa != "" {
				annotations[AnnCDIPodServiceAccount] = sa
			}
			Expect(annotations).To(HaveKeyWithValue(AnnCDIPodServiceAccount, "plan-sa"))
		})

		ginkgo.It("should set CDI SA annotation when global SA is set", func() {
			Settings.Migration.ServiceAccount = "global-sa"
			p := createPlanKubevirt(nil)
			annotations := make(map[string]string)
			if sa := resolveServiceAccount(p); sa != "" {
				annotations[AnnCDIPodServiceAccount] = sa
			}
			Expect(annotations).To(HaveKeyWithValue(AnnCDIPodServiceAccount, "global-sa"))
		})

		ginkgo.It("should not set CDI SA annotation when both SAs are empty", func() {
			Settings.Migration.ServiceAccount = ""
			p := createPlanKubevirt(nil)
			annotations := make(map[string]string)
			if sa := resolveServiceAccount(p); sa != "" {
				annotations[AnnCDIPodServiceAccount] = sa
			}
			Expect(annotations).ToNot(HaveKey(AnnCDIPodServiceAccount))
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

	ginkgo.Describe("getVirtV2vImage", func() {
		const globalImage = "quay.io/kubev2v/forklift-virt-v2v:latest"
		const xfsImage = "quay.io/kubev2v/forklift-virt-v2v-rhel9:latest"

		ginkgo.BeforeEach(func() {
			Settings.Migration.VirtV2vImage = globalImage
			Settings.Migration.VirtV2vImageXFS = xfsImage
		})

		ginkgo.It("should return the global image when plan has no override", func() {
			p := createPlanKubevirt(nil)
			Expect(getVirtV2vImage(p)).To(Equal(globalImage))
		})

		ginkgo.It("should return the per-plan image when set", func() {
			perPlanImage := "quay.io/kubev2v/forklift-virt-v2v:custom-build"
			p := createPlanKubevirt(nil)
			p.Spec.VirtV2vImage = perPlanImage
			Expect(getVirtV2vImage(p)).To(Equal(perPlanImage))
		})

		ginkgo.It("should fall back to global image when plan override is empty string", func() {
			p := createPlanKubevirt(nil)
			p.Spec.VirtV2vImage = ""
			Expect(getVirtV2vImage(p)).To(Equal(globalImage))
		})

		ginkgo.It("should return the XFS image when XfsCompatibility is enabled", func() {
			p := createPlanKubevirt(nil)
			p.Spec.XfsCompatibility = true
			Expect(getVirtV2vImage(p)).To(Equal(xfsImage))
		})

		ginkgo.It("should return the global image when XfsCompatibility is false", func() {
			p := createPlanKubevirt(nil)
			p.Spec.XfsCompatibility = false
			Expect(getVirtV2vImage(p)).To(Equal(globalImage))
		})

		ginkgo.It("should prioritize VirtV2vImage over XfsCompatibility when both are set", func() {
			perPlanImage := "quay.io/kubev2v/forklift-virt-v2v:custom-build"
			p := createPlanKubevirt(nil)
			p.Spec.VirtV2vImage = perPlanImage
			p.Spec.XfsCompatibility = true
			Expect(getVirtV2vImage(p)).To(Equal(perPlanImage))
		})

		ginkgo.It("should return XFS image when XfsCompatibility is true and VirtV2vImage is empty", func() {
			p := createPlanKubevirt(nil)
			p.Spec.VirtV2vImage = ""
			p.Spec.XfsCompatibility = true
			Expect(getVirtV2vImage(p)).To(Equal(xfsImage))
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

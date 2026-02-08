//nolint:errcheck
package plan

import (
	"context"
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

var _ = ginkgo.Describe("Plan cleanup functions", func() {
	ginkgo.Describe("planOnlyLabels", func() {
		ginkgo.It("should return only plan label", func() {
			kubevirt := createKubeVirtWithPlanUID("test-plan-uid")
			labels := kubevirt.planOnlyLabels()
			Expect(labels).To(HaveLen(1))
			Expect(labels).To(HaveKeyWithValue("plan", "test-plan-uid"))
		})
	})

	ginkgo.Describe("migrationOnlyLabels", func() {
		ginkgo.It("should return plan and migration labels", func() {
			kubevirt := createKubeVirtWithPlanUID("test-plan-uid")
			labels := kubevirt.migrationOnlyLabels("test-migration-uid")
			Expect(labels).To(HaveLen(2))
			Expect(labels).To(HaveKeyWithValue("plan", "test-plan-uid"))
			Expect(labels).To(HaveKeyWithValue("migration", "test-migration-uid"))
		})
	})

	ginkgo.Describe("DeleteAllPlanPods", func() {
		ginkgo.It("should delete pods with plan label", func() {
			pod1 := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "test-ns",
					Labels: map[string]string{
						"plan": "test-plan-uid",
					},
				},
			}
			pod2 := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod2",
					Namespace: "test-ns",
					Labels: map[string]string{
						"plan": "other-plan-uid",
					},
				},
			}

			kubevirt := createKubeVirtWithPlanUIDAndObjects("test-plan-uid", "test-ns", pod1, pod2)
			err := kubevirt.DeleteAllPlanPods()
			Expect(err).ToNot(HaveOccurred())

			// Verify pod1 was deleted, pod2 remains
			podList := &v1.PodList{}
			kubevirt.Destination.Client.List(context.TODO(), podList)
			Expect(podList.Items).To(HaveLen(1))
			Expect(podList.Items[0].Name).To(Equal("pod2"))
		})
	})

	ginkgo.Describe("DeleteAllPlanSecrets", func() {
		ginkgo.It("should delete secrets with plan and resource labels", func() {
			// secret1 has plan label AND resource label - should be deleted
			secret1 := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret1",
					Namespace: "test-ns",
					Labels: map[string]string{
						"plan":     "test-plan-uid",
						"resource": "vm-config",
					},
				},
			}
			// secret2 has different plan - should be preserved
			secret2 := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret2",
					Namespace: "test-ns",
					Labels: map[string]string{
						"plan":     "other-plan-uid",
						"resource": "vm-config",
					},
				},
			}
			// secret3 has plan label but NO resource label (VM-dependency secret) - should be preserved
			secret3 := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret3",
					Namespace: "test-ns",
					Labels: map[string]string{
						"plan": "test-plan-uid",
					},
				},
			}

			kubevirt := createKubeVirtWithPlanUIDAndObjects("test-plan-uid", "test-ns", secret1, secret2, secret3)
			err := kubevirt.DeleteAllPlanSecrets()
			Expect(err).ToNot(HaveOccurred())

			// Verify secret1 was deleted, secret2 and secret3 remain
			secretList := &v1.SecretList{}
			kubevirt.Destination.Client.List(context.TODO(), secretList)
			Expect(secretList.Items).To(HaveLen(2))
			names := []string{secretList.Items[0].Name, secretList.Items[1].Name}
			Expect(names).To(ContainElements("secret2", "secret3"))
		})
	})

	ginkgo.Describe("DeleteMigrationPVCs", func() {
		ginkgo.It("should delete PVCs with specific migration label", func() {
			pvc1 := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc1",
					Namespace: "test-ns",
					Labels: map[string]string{
						"plan":      "test-plan-uid",
						"migration": "migration-uid-1",
					},
				},
			}
			pvc2 := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc2",
					Namespace: "test-ns",
					Labels: map[string]string{
						"plan":      "test-plan-uid",
						"migration": "migration-uid-2",
					},
				},
			}

			kubevirt := createKubeVirtWithPlanUIDAndObjects("test-plan-uid", "test-ns", pvc1, pvc2)
			err := kubevirt.DeleteMigrationPVCs("migration-uid-1")
			Expect(err).ToNot(HaveOccurred())

			// Verify pvc1 was deleted (migration-uid-1), pvc2 remains (migration-uid-2)
			pvcList := &v1.PersistentVolumeClaimList{}
			kubevirt.Destination.Client.List(context.TODO(), pvcList)
			Expect(pvcList.Items).To(HaveLen(1))
			Expect(pvcList.Items[0].Name).To(Equal("pvc2"))
		})
	})
})

func createKubeVirtWithPlanUID(planUID string) *KubeVirt {
	kubevirt := createKubeVirt()
	kubevirt.Plan.UID = "test-plan-uid"
	return kubevirt
}

func createKubeVirtWithPlanUIDAndObjects(planUID string, namespace string, objs ...runtime.Object) *KubeVirt {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = k8snet.AddToScheme(scheme)
	v1beta1.SchemeBuilder.AddToScheme(scheme)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
	plan := createPlanKubevirt(nil)
	plan.UID = "test-plan-uid"
	plan.Spec.TargetNamespace = namespace
	return &KubeVirt{
		Context: &plancontext.Context{
			Destination: plancontext.Destination{
				Client: client,
			},
			Log:       KubeVirtLog,
			Migration: createMigration(),
			Plan:      plan,
			Client:    client,
		},
	}
}

//nolint:errcheck
package plan

import (
	"context"
	"encoding/json"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	convctx "github.com/kubev2v/forklift/pkg/controller/conversion/context"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cnv "kubevirt.io/api/core/v1"
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
				Annotations: map[string]string{
					"forklift.konveyor.io/disk-source": "test-disk",
				},
			},
		}

		ginkgo.It("should return PVCs", func() {
			kubevirt := createKubeVirt(pvc)
			pvcs, err := kubevirt.getPVCs(ref.Ref{ID: "test"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
		})

		ginkgo.It("should exclude prime PVCs created by the volume populator", func() {
			realPVC := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "9ba19d87-0d7f-43be-9ffe-d7dd3a552188-ggcw8",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "test",
						"vmID":      "test",
						"imageID":   "9ba19d87",
					},
					Annotations: map[string]string{
						"forklift.konveyor.io/disk-source": "9ba19d87",
					},
				},
			}
			primePVC := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prime-222bcc84-8e35-4afb-a607-39605ff0397a",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "test",
						"vmID":      "test",
					},
				},
			}
			kubevirt := createKubeVirt(realPVC, primePVC)
			pvcs, err := kubevirt.getPVCs(ref.Ref{ID: "test"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			Expect(pvcs[0].Name).To(Equal(realPVC.Name))
		})

		ginkgo.It("should exclude PVCs without disk-source annotation", func() {
			realPVC := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "real-disk-pvc",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "test",
						"vmID":      "test",
					},
					Annotations: map[string]string{
						"forklift.konveyor.io/disk-source": "disk-001",
					},
				},
			}
			noIdentityPVC := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stray-pvc-no-annotations",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "test",
						"vmID":      "test",
					},
				},
			}
			kubevirt := createKubeVirt(realPVC, noIdentityPVC)
			pvcs, err := kubevirt.getPVCs(ref.Ref{ID: "test"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			Expect(pvcs[0].Name).To(Equal(realPVC.Name))
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

	ginkgo.Describe("DeletePopulatorPods", func() {
		const migrationUID = "test-migration-uid"
		const targetNS = "test-ns"

		var savedRetain bool

		ginkgo.BeforeEach(func() {
			savedRetain = Settings.RetainPopulatorPods
		})
		ginkgo.AfterEach(func() {
			Settings.RetainPopulatorPods = savedRetain
		})

		const testVmID = "vm-123"

		createKubeVirtWithPopulatorPod := func() (*KubeVirt, *v1.Pod) {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "populate-test-pvc",
					Namespace: targetNS,
					Labels: map[string]string{
						"migration": migrationUID,
						"vmID":      testVmID,
					},
				},
			}

			kubevirt := createKubeVirt(pod)
			kubevirt.Plan.Spec.TargetNamespace = targetNS
			kubevirt.Plan.Status.Migration.History = []plan.Snapshot{
				{
					Migration: plan.SnapshotRef{UID: migrationUID},
				},
			}
			return kubevirt, pod
		}

		ginkgo.It("should skip deletion when RetainPopulatorPods is true", func() {
			Settings.RetainPopulatorPods = true
			kubevirt, _ := createKubeVirtWithPopulatorPod()
			vm := &plan.VMStatus{}
			vm.ID = testVmID

			err := kubevirt.DeletePopulatorPods(vm)
			Expect(err).ToNot(HaveOccurred())

			pods, err := kubevirt.getPopulatorPods(testVmID)
			Expect(err).ToNot(HaveOccurred())
			Expect(pods).To(HaveLen(1), "populator pod should still exist")
		})

		ginkgo.It("should delete populator pods when RetainPopulatorPods is false", func() {
			Settings.RetainPopulatorPods = false
			kubevirt, _ := createKubeVirtWithPopulatorPod()
			vm := &plan.VMStatus{}
			vm.ID = testVmID

			err := kubevirt.DeletePopulatorPods(vm)
			Expect(err).ToNot(HaveOccurred())

			pods, err := kubevirt.getPopulatorPods(testVmID)
			Expect(err).ToNot(HaveOccurred())
			Expect(pods).To(BeEmpty(), "populator pod should have been deleted")
		})
	})

	ginkgo.Describe("determineRunStrategy", func() {
		var kubevirt *KubeVirt

		ginkgo.BeforeEach(func() {
			kubevirt = createKubeVirt()
		})

		ginkgo.It("should return RunStrategyAlways when plan TargetPowerState is 'on'", func() {
			kubevirt.Plan.Spec.TargetPowerState = plan.TargetPowerStateOn
			vm := &plan.VMStatus{}

			result := kubevirt.determineRunStrategy(vm)
			Expect(result).To(Equal(cnv.RunStrategyAlways))
		})

		ginkgo.It("should return RunStrategyHalted when plan TargetPowerState is 'off'", func() {
			kubevirt.Plan.Spec.TargetPowerState = plan.TargetPowerStateOff
			vm := &plan.VMStatus{}

			result := kubevirt.determineRunStrategy(vm)
			Expect(result).To(Equal(cnv.RunStrategyHalted))
		})

		ginkgo.It("should return RunStrategyAlways when VM-level TargetPowerState overrides plan to 'on'", func() {
			kubevirt.Plan.Spec.TargetPowerState = plan.TargetPowerStateOff
			vm := &plan.VMStatus{}
			vm.TargetPowerState = plan.TargetPowerStateOn

			result := kubevirt.determineRunStrategy(vm)
			Expect(result).To(Equal(cnv.RunStrategyAlways))
		})

		ginkgo.It("should return RunStrategyHalted when VM-level TargetPowerState overrides plan to 'off'", func() {
			kubevirt.Plan.Spec.TargetPowerState = plan.TargetPowerStateOn
			vm := &plan.VMStatus{}
			vm.TargetPowerState = plan.TargetPowerStateOff

			result := kubevirt.determineRunStrategy(vm)
			Expect(result).To(Equal(cnv.RunStrategyHalted))
		})

		ginkgo.It("should match source power state when no TargetPowerState is set (source On)", func() {
			kubevirt.Plan.Spec.TargetPowerState = ""
			vm := &plan.VMStatus{}
			vm.RestorePowerState = plan.VMPowerStateOn

			result := kubevirt.determineRunStrategy(vm)
			Expect(result).To(Equal(cnv.RunStrategyAlways))
		})

		ginkgo.It("should match source power state when no TargetPowerState is set (source Off)", func() {
			kubevirt.Plan.Spec.TargetPowerState = ""
			vm := &plan.VMStatus{}
			vm.RestorePowerState = plan.VMPowerStateOff

			result := kubevirt.determineRunStrategy(vm)
			Expect(result).To(Equal(cnv.RunStrategyHalted))
		})

		ginkgo.It("should default to RunStrategyHalted when source power state is unknown", func() {
			kubevirt.Plan.Spec.TargetPowerState = ""
			vm := &plan.VMStatus{}
			vm.RestorePowerState = plan.VMPowerStateUnknown

			result := kubevirt.determineRunStrategy(vm)
			Expect(result).To(Equal(cnv.RunStrategyHalted))
		})

		ginkgo.It("should default to RunStrategyHalted when source power state is empty", func() {
			kubevirt.Plan.Spec.TargetPowerState = ""
			vm := &plan.VMStatus{}
			vm.RestorePowerState = ""

			result := kubevirt.determineRunStrategy(vm)
			Expect(result).To(Equal(cnv.RunStrategyHalted))
		})
	})

	ginkgo.Describe("CleanupCopiedConfigMaps", func() {
		ginkgo.It("should delete extra-v2v-conf and customization-scripts ConfigMaps by name", func() {
			extraCM := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-plan-extra-v2v-conf",
					Namespace: "target-ns",
				},
				Data: map[string]string{"key": "val"},
			}
			custCM := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-plan-customization-scripts",
					Namespace: "target-ns",
				},
				Data: map[string]string{"key": "val"},
			}
			kubevirt := createKubeVirtWithProvider(v1beta1.VSphere, extraCM, custCM)

			kubevirt.CleanupCopiedConfigMaps()

			// Both should be gone
			result := &v1.ConfigMap{}
			err := kubevirt.Destination.Get(context.TODO(),
				client.ObjectKey{Name: extraCM.Name, Namespace: "target-ns"}, result)
			Expect(k8serr.IsNotFound(err)).To(BeTrue(), "extra-v2v-conf should be deleted")

			err = kubevirt.Destination.Get(context.TODO(),
				client.ObjectKey{Name: custCM.Name, Namespace: "target-ns"}, result)
			Expect(k8serr.IsNotFound(err)).To(BeTrue(), "customization-scripts should be deleted")
		})

		ginkgo.It("should delete vddk-conf ConfigMaps by label selector", func() {
			vddkCM := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-plan-vddk-conf-abc12",
					Namespace: "target-ns",
					Labels: map[string]string{
						kMigration:     "test",
						kPlan:          "plan-uid",
						kPlanName:      "test-plan",
						kPlanNamespace: "test",
						kUse:           VddkConf,
						kResource:      ResourceVDDKConfig,
					},
				},
				Data: map[string]string{"key": "val"},
			}
			kubevirt := createKubeVirtWithProvider(v1beta1.VSphere, vddkCM)

			kubevirt.CleanupCopiedConfigMaps()

			result := &v1.ConfigMap{}
			err := kubevirt.Destination.Get(context.TODO(),
				client.ObjectKey{Name: vddkCM.Name, Namespace: "target-ns"}, result)
			Expect(k8serr.IsNotFound(err)).To(BeTrue(), "vddk-conf should be deleted")
		})

		ginkgo.It("should delete multiple vddk-conf ConfigMaps from retries", func() {
			labels := map[string]string{
				kMigration:     "test",
				kPlan:          "plan-uid",
				kPlanName:      "test-plan",
				kPlanNamespace: "test",
				kUse:           VddkConf,
				kResource:      ResourceVDDKConfig,
			}
			vddkCM1 := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-plan-vddk-conf-aaa11", Namespace: "target-ns", Labels: labels,
				},
				Data: map[string]string{"key": "val1"},
			}
			vddkCM2 := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-plan-vddk-conf-bbb22", Namespace: "target-ns", Labels: labels,
				},
				Data: map[string]string{"key": "val2"},
			}
			kubevirt := createKubeVirtWithProvider(v1beta1.VSphere, vddkCM1, vddkCM2)

			kubevirt.CleanupCopiedConfigMaps()

			result := &v1.ConfigMap{}
			err := kubevirt.Destination.Get(context.TODO(),
				client.ObjectKey{Name: vddkCM1.Name, Namespace: "target-ns"}, result)
			Expect(k8serr.IsNotFound(err)).To(BeTrue(), "first vddk-conf should be deleted")

			err = kubevirt.Destination.Get(context.TODO(),
				client.ObjectKey{Name: vddkCM2.Name, Namespace: "target-ns"}, result)
			Expect(k8serr.IsNotFound(err)).To(BeTrue(), "second vddk-conf should be deleted")
		})

		ginkgo.It("should not error when no copied ConfigMaps exist", func() {
			kubevirt := createKubeVirtWithProvider(v1beta1.VSphere)
			Expect(func() { kubevirt.CleanupCopiedConfigMaps() }).ToNot(Panic())
		})
	})

	ginkgo.Describe("podVolumeMounts", func() {
		var kubevirt *KubeVirt

		ginkgo.BeforeEach(func() {
			kubevirt = createKubeVirt()
			providerType := v1beta1.VSphere
			kubevirt.Source = plancontext.Source{
				Provider: &v1beta1.Provider{
					ObjectMeta: metav1.ObjectMeta{Name: "test-vsphere", Namespace: "test"},
					Spec: v1beta1.ProviderSpec{
						Type: &providerType,
					},
				},
			}
			kubevirt.Plan.Spec.TargetNamespace = "target-ns"
		})

		ginkgo.It("should include scripts volume in extraVolumes when CustomizationScripts is set", func() {
			cm := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-scripts", Namespace: "target-ns"},
				Data:       map[string]string{"01_linux_run_test.sh": "#!/bin/sh\necho test"},
			}
			kubevirt = createKubeVirtWithProvider(v1beta1.VSphere, cm)
			kubevirt.Plan.Spec.CustomizationScripts = &v1.ObjectReference{
				Name:      "my-scripts",
				Namespace: "target-ns",
			}

			vmVolumes := []cnv.Volume{}
			vm := &plan.VMStatus{}

			volumes, mounts, _, extraVolumes, extraMounts, err := kubevirt.podVolumeMounts(vmVolumes, nil, nil, vm)
			Expect(err).ToNot(HaveOccurred())

			// scripts-volume-mount should be in volumes
			var foundVol bool
			for _, vol := range volumes {
				if vol.Name == DynamicScriptsVolumeName {
					Expect(vol.ConfigMap).ToNot(BeNil())
					Expect(vol.ConfigMap.Name).To(Equal("my-scripts"))
					foundVol = true
				}
			}
			Expect(foundVol).To(BeTrue(), "scripts volume not found in volumes")

			// scripts-volume-mount should be in mounts
			var foundMount bool
			for _, m := range mounts {
				if m.Name == DynamicScriptsVolumeName {
					Expect(m.MountPath).To(Equal(DynamicScriptsMountPath))
					foundMount = true
				}
			}
			Expect(foundMount).To(BeTrue(), "scripts mount not found in mounts")

			// scripts-volume-mount must also be in extraVolumes (for Conversion CR propagation)
			var foundExtraVol bool
			for _, vol := range extraVolumes {
				if vol.Name == DynamicScriptsVolumeName {
					Expect(vol.ConfigMap).ToNot(BeNil())
					Expect(vol.ConfigMap.Name).To(Equal("my-scripts"))
					foundExtraVol = true
				}
			}
			Expect(foundExtraVol).To(BeTrue(), "scripts volume not found in extraVolumes")

			// scripts mount must also be in extraMounts
			var foundExtraMount bool
			for _, m := range extraMounts {
				if m.Name == DynamicScriptsVolumeName {
					Expect(m.MountPath).To(Equal(DynamicScriptsMountPath))
					foundExtraMount = true
				}
			}
			Expect(foundExtraMount).To(BeTrue(), "scripts mount not found in extraMounts")
		})

		ginkgo.It("should use generated name when CustomizationScripts namespace differs from target", func() {
			// Simulate the CM already copied to target-ns with the generated name
			// (EnsureCustomizationScriptsConfigMap would have done this before podVolumeMounts)
			copiedCM := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test-plan-customization-scripts", Namespace: "target-ns"},
				Data:       map[string]string{"01_linux_run_test.sh": "#!/bin/sh\necho test"},
			}
			kubevirt = createKubeVirtWithProvider(v1beta1.VSphere, copiedCM)
			kubevirt.Plan.Spec.CustomizationScripts = &v1.ObjectReference{
				Name:      "my-scripts",
				Namespace: "other-ns",
			}

			vm := &plan.VMStatus{}
			_, _, _, extraVolumes, _, err := kubevirt.podVolumeMounts(nil, nil, nil, vm)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			for _, vol := range extraVolumes {
				if vol.Name == DynamicScriptsVolumeName {
					Expect(vol.ConfigMap).ToNot(BeNil())
					Expect(vol.ConfigMap.Name).To(Equal("test-plan-customization-scripts"))
					found = true
				}
			}
			Expect(found).To(BeTrue(), "scripts volume should be found with generated name")
		})

		ginkgo.It("should return error when CustomizationScripts ConfigMap does not exist", func() {
			kubevirt = createKubeVirtWithProvider(v1beta1.VSphere)
			kubevirt.Plan.Spec.CustomizationScripts = &v1.ObjectReference{
				Name:      "nonexistent-scripts",
				Namespace: "target-ns",
			}

			vm := &plan.VMStatus{}
			_, _, _, _, _, err := kubevirt.podVolumeMounts(nil, nil, nil, vm)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("CustomizationScripts ConfigMap nonexistent-scripts not found"))
		})

		ginkgo.It("should not add scripts volume when CustomizationScripts is nil", func() {
			kubevirt.Plan.Spec.CustomizationScripts = nil

			vm := &plan.VMStatus{}
			volumes, _, _, extraVolumes, _, err := kubevirt.podVolumeMounts(nil, nil, nil, vm)
			Expect(err).ToNot(HaveOccurred())

			for _, vol := range volumes {
				Expect(vol.Name).ToNot(Equal(DynamicScriptsVolumeName))
			}
			for _, vol := range extraVolumes {
				Expect(vol.Name).ToNot(Equal(DynamicScriptsVolumeName))
			}
		})
	})

	ginkgo.Describe("GetDeepInspectionConversion", func() {
		ginkgo.It("returns nil when a Conversion CR for the same VM exists under a different plan UID", func() {
			const (
				vmID           = "vm-abc-123"
				planNamespace  = "test-ns"
				otherPlanUID   = "other-plan-uid"
				currentPlanUID = "current-plan-uid"
			)

			// A DeepInspection CR that belongs to a *different* plan but the same VM.
			existingCR := &v1beta1.Conversion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "di-conversion-other-plan",
					Namespace: planNamespace,
					Labels: map[string]string{
						convctx.LabelVM:             vmID,
						convctx.LabelConversionType: string(v1beta1.DeepInspection),
						convctx.LabelPlan:           otherPlanUID,
					},
				},
			}

			kubevirt := createKubeVirt(existingCR)
			kubevirt.Plan.ObjectMeta = metav1.ObjectMeta{
				Name:      "current-plan",
				Namespace: planNamespace,
				UID:       currentPlanUID,
			}

			vm := &plan.VMStatus{}
			vm.ID = vmID

			cr, err := kubevirt.GetDeepInspectionConversion(vm)
			Expect(err).ToNot(HaveOccurred())
			Expect(cr).To(BeNil())
		})
	})
})

func createKubeVirtWithProvider(providerType v1beta1.ProviderType, objs ...runtime.Object) *KubeVirt {
	kv := createKubeVirt(objs...)
	kv.Source = plancontext.Source{
		Provider: &v1beta1.Provider{
			ObjectMeta: metav1.ObjectMeta{Name: "test-provider", Namespace: "test"},
			Spec: v1beta1.ProviderSpec{
				Type: &providerType,
			},
		},
	}
	kv.Plan.ObjectMeta = metav1.ObjectMeta{Name: "test-plan", Namespace: "test", UID: "plan-uid"}
	kv.Plan.Spec.TargetNamespace = "target-ns"
	return kv
}

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

var _ = ginkgo.Describe("PVC name template", func() {
	ginkgo.Describe("GetPVCNameTemplate", func() {
		ginkgo.It("should return the universal default when plan has no template", func() {
			kv := createKubeVirt()
			kv.Plan.Name = "my-plan"
			template := planbase.GetPVCNameTemplate(kv.Plan, "vm-1")
			Expect(template).To(ContainSubstring("PlanName"))
			Expect(template).To(ContainSubstring("TargetVmName"))
			Expect(template).To(ContainSubstring("DiskIndex"))
		})

		ginkgo.It("should return the plan-level template when set", func() {
			kv := createKubeVirt()
			kv.Plan.Spec.PVCNameTemplate = "{{.PlanName}}-{{.VmId}}-disk-{{.DiskIndex}}"
			template := planbase.GetPVCNameTemplate(kv.Plan, "vm-1")
			Expect(template).To(Equal("{{.PlanName}}-{{.VmId}}-disk-{{.DiskIndex}}"))
		})

		ginkgo.It("should return the VM-level template when set", func() {
			kv := createKubeVirt()
			kv.Plan.Spec.PVCNameTemplate = "plan-level"
			kv.Plan.Spec.VMs = []plan.VM{
				{Ref: ref.Ref{ID: "vm-1"}, PVCNameTemplate: "vm-level-{{.DiskIndex}}"},
			}
			template := planbase.GetPVCNameTemplate(kv.Plan, "vm-1")
			Expect(template).To(Equal("vm-level-{{.DiskIndex}}"))
		})
	})

	ginkgo.Describe("applyPVCNameTemplate", func() {
		ginkgo.It("should set GenerateName with default template and UseGenerateName=true", func() {
			kv := createKubeVirt()
			kv.Plan.Name = "test-plan"
			kv.Plan.Spec.PVCNameTemplateUseGenerateName = true
			vm := &plan.VMStatus{}
			vm.ID = "vm-1"
			vm.Name = "my-vm"

			objectMeta := &metav1.ObjectMeta{}
			err := kv.applyPVCNameTemplate(objectMeta, vm, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(objectMeta.GenerateName).To(Equal("test-plan-my-vm-disk-0-"))
			Expect(objectMeta.Name).To(BeEmpty())
		})

		ginkgo.It("should set Name with default template and UseGenerateName=false", func() {
			kv := createKubeVirt()
			kv.Plan.Name = "test-plan"
			kv.Plan.Spec.PVCNameTemplateUseGenerateName = false
			vm := &plan.VMStatus{}
			vm.ID = "vm-1"
			vm.Name = "my-vm"

			objectMeta := &metav1.ObjectMeta{}
			err := kv.applyPVCNameTemplate(objectMeta, vm, 2)
			Expect(err).ToNot(HaveOccurred())
			Expect(objectMeta.Name).To(Equal("test-plan-my-vm-disk-2"))
			Expect(objectMeta.GenerateName).To(BeEmpty())
		})

		ginkgo.It("should use NewName when set", func() {
			kv := createKubeVirt()
			kv.Plan.Name = "plan"
			kv.Plan.Spec.PVCNameTemplateUseGenerateName = false
			vm := &plan.VMStatus{}
			vm.ID = "vm-1"
			vm.Name = "Original VM!"
			vm.NewName = "safe-vm-name"

			objectMeta := &metav1.ObjectMeta{}
			err := kv.applyPVCNameTemplate(objectMeta, vm, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(objectMeta.Name).To(Equal("plan-safe-vm-name-disk-0"))
		})

		ginkgo.It("should truncate long names", func() {
			kv := createKubeVirt()
			kv.Plan.Name = "very-long-plan-name-exceeding"
			kv.Plan.Spec.PVCNameTemplateUseGenerateName = false
			vm := &plan.VMStatus{}
			vm.ID = "vm-1"
			vm.Name = "very-long-vm-name-exceeding-limit"

			objectMeta := &metav1.ObjectMeta{}
			err := kv.applyPVCNameTemplate(objectMeta, vm, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(objectMeta.Name)).To(BeNumerically("<=", 63))
		})
	})
})

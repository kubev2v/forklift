package vsphere

import (
	"context"
	"fmt"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/vim25/types"
	v1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	cnv "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var builderLog = logging.WithName("vsphere-builder-test")

const ManualOrigin = string(types.NetIpConfigInfoIpAddressOriginManual)

var _ = Describe("vSphere builder", func() {
	Context("PopulatorVolumes", func() {
		It("should created new secret with the provider secret and the storage secret data", func() {
			builder := createBuilder(
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "storage-test-secret", Namespace: "test"},
					Data: map[string][]byte{
						"storagekey": []byte("storageval"),
					},
				},
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "migration-test-secret", Namespace: "test"},
					Data: map[string][]byte{
						"providerkey": []byte("providerval"),
					},
				},
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "offload-ssh-keys-test-vsphere-provider-private", Namespace: "test"},
					Data: map[string][]byte{
						"private-key": []byte("fake-private-key"),
					},
				},
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "offload-ssh-keys-test-vsphere-provider-public", Namespace: "test"},
					Data: map[string][]byte{
						"public-key": []byte("fake-public-key"),
					},
				},
				&core.PersistentVolumeClaim{
					ObjectMeta: meta.ObjectMeta{Name: "test-pvc", Namespace: "test"},
				},
			)

			// Execute
			pvc := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{Name: "test-pvc", Namespace: "test"},
			}
			err := builder.mergeSecrets("migration-test-secret", "test", "storage-test-secret", "test", "merged-test-secret", pvc)
			underTest := core.Secret{}
			errGet := builder.Destination.Get(context.Background(), client.ObjectKey{
				Name:      "merged-test-secret",
				Namespace: "test"}, &underTest)

			// Assert
			Expect(err).NotTo(HaveOccurred())
			Expect(errGet).NotTo(HaveOccurred())
			Expect(underTest.Data).To(HaveLen(5))
			Expect(underTest.Data).To(HaveKeyWithValue("storagekey", []byte("storageval")))
			Expect(underTest.Data).To(HaveKeyWithValue("providerkey", []byte("providerval")))
			Expect(underTest.Data).To(HaveKeyWithValue("GOVMOMI_HOSTNAME", []byte("vcenter.test.example.com")))
			Expect(underTest.Data).To(HaveKey("SSH_PRIVATE_KEY"))
			Expect(underTest.Data).To(HaveKey("SSH_PUBLIC_KEY"))
			Expect(underTest.OwnerReferences).To(HaveLen(1))
			Expect(underTest.OwnerReferences[0].Name).To(Equal("test-pvc"))
		})
		It("should set default access mode to ReadWriteMany for block volumes", func() {
			// Setup
			builder := createBuilder(
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "test-secret", Namespace: "test"},
					Data: map[string][]byte{
						"foo": []byte("bar"),
					},
				},
			)
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{
						ID:   "test-vm-id",
						Name: "test",
					},
					Disks: []vsphere.Disk{
						{
							Datastore: vsphere.Ref{ID: "ds-1"},
							File:      "[datastore1] vm-123/vm-123.vmdk",
							Bus:       vsphere.SCSI,
							Capacity:  1024 * 1024 * 1024, // 1 GiB
							Key:       2000,
						},
					},
				},
			}

			dsMap := []v1beta1.StoragePair{
				{
					Source: ref.Ref{ID: "ds-1"},
					Destination: v1beta1.DestinationStorage{
						StorageClass: "test-sc",
					},
					OffloadPlugin: &v1beta1.OffloadPlugin{
						VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
							StorageVendorProduct: "test-vendor",
							SecretRef:            "test-secret",
						},
					},
				},
			}
			storageMap := v1beta1.StorageMap{
				Spec: v1beta1.StorageMapSpec{
					Map: dsMap,
				},
			}
			annotations := map[string]string{"test-annotation": "true"}
			secretName := "test-secret"

			// Mock inventory
			inventory := &mockInventory{
				ds: model.Datastore{Resource: model.Resource{ID: "ds-1"}},
				vm: vm,
			}
			builder.Source.Inventory = inventory
			builder.Context.Map.Storage = &storageMap

			// Execute
			pvcs, err := builder.PopulatorVolumes(ref.Ref{ID: vm.ID}, annotations, secretName)

			// Assert
			Expect(err).NotTo(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			pvc := pvcs[0]
			Expect(pvc.Spec.AccessModes).To(ContainElement(core.ReadWriteMany))
			Expect(pvc.Spec.VolumeMode).To(Equal(ptr.To(core.PersistentVolumeBlock)))
		})
		It("should set default PVC template name", func() {
			// Setup
			builder := createBuilder(
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "test-secret", Namespace: "test"},
					Data: map[string][]byte{
						"foo": []byte("bar"),
					},
				},
			)
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{
						ID:   "test-vm-id",
						Name: "customer-frontend-server",
					},
					Disks: []vsphere.Disk{
						{
							Datastore: vsphere.Ref{ID: "ds-1"},
							File:      "[datastore1] vm-123/vm-123.vmdk",
							Bus:       vsphere.SCSI,
							Capacity:  1024 * 1024 * 1024, // 1 GiB
							Key:       2000,
						},
					},
				},
			}

			dsMap := []v1beta1.StoragePair{
				{
					Source: ref.Ref{ID: "ds-1"},
					Destination: v1beta1.DestinationStorage{
						StorageClass: "test-sc",
					},
					OffloadPlugin: &v1beta1.OffloadPlugin{
						VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
							StorageVendorProduct: "test-vendor",
							SecretRef:            "test-secret",
						},
					},
				},
			}
			storageMap := v1beta1.StorageMap{
				Spec: v1beta1.StorageMapSpec{
					Map: dsMap,
				},
			}
			annotations := map[string]string{"test-annotation": "true"}
			secretName := "test-secret"

			// Mock inventory
			inventory := &mockInventory{
				ds: model.Datastore{Resource: model.Resource{ID: "ds-1"}},
				vm: vm,
			}
			builder.Source.Inventory = inventory
			builder.Context.Map.Storage = &storageMap

			// Execute
			pvcs, err := builder.PopulatorVolumes(ref.Ref{ID: vm.ID}, annotations, secretName)

			// Assert
			Expect(err).NotTo(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			pvc := pvcs[0]
			// The default template now uses trunc 4 for both plan and VM names
			Expect(pvc.Name).Should(HavePrefix(fmt.Sprintf("%.4s-%.4s-disk-", builder.Plan.Name, vm.Name)))
			Expect(pvc.Spec.DataSourceRef.Kind).To(Equal(v1beta1.VSphereXcopyVolumePopulatorKind))
			Expect(pvc.Spec.DataSourceRef.APIGroup).To(Equal(&v1beta1.SchemeGroupVersion.Group))
			Expect(pvc.Spec.DataSourceRef.Name).To(Equal(pvc.Name))
		})

		It("should honor explicit AccessMode StorageMap and ignore VolumeMode from StorageMap", func() {
			builder := createBuilder(&core.Secret{ObjectMeta: meta.ObjectMeta{Name: "test-secret", Namespace: "test"}})
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-2", Name: "vm"},
					Disks: []vsphere.Disk{
						{
							Datastore: vsphere.Ref{ID: "ds-2"},
							File:      "[datastore2] vm-2/vm-2.vmdk",
							Bus:       vsphere.SCSI, Capacity: 1 << 20, Key: 2000,
						},
					},
				},
			}
			builder.Source.Inventory = &mockInventory{ds: model.Datastore{Resource: model.Resource{ID: "ds-2"}}, vm: vm}
			builder.Context.Map.Storage = &v1beta1.StorageMap{
				Spec: v1beta1.StorageMapSpec{
					Map: []v1beta1.StoragePair{{
						Source: ref.Ref{ID: "ds-2"},
						Destination: v1beta1.DestinationStorage{
							StorageClass: "test-sc",
							AccessMode:   core.ReadWriteOnce,
							VolumeMode:   core.PersistentVolumeFilesystem,
						},
						OffloadPlugin: &v1beta1.OffloadPlugin{
							VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
								StorageVendorProduct: "test-vendor",
								SecretRef:            "test-secret",
							},
						},
					}},
				},
			}
			pvcs, err := builder.PopulatorVolumes(ref.Ref{ID: vm.ID}, nil, "test-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			Expect(pvcs[0].Spec.AccessModes).To(ConsistOf(core.ReadWriteOnce))
			Expect(pvcs[0].Spec.VolumeMode).To(Equal(ptr.To(core.PersistentVolumeBlock)))
		})

		It("should not set migrationHost when dedicatedMigrationHosts is not set", func() {
			builder := createBuilder(
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "test-secret", Namespace: "test"},
				},
			)
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{
						ID:   "test-vm-id",
						Name: "test",
					},
					Disks: []vsphere.Disk{
						{
							Datastore: vsphere.Ref{ID: "ds-1"},
							File:      "[datastore1] vm-123/vm-123.vmdk",
							Bus:       vsphere.SCSI,
							Capacity:  1024 * 1024 * 1024,
							Key:       2000,
						},
					},
				},
			}

			dsMap := []v1beta1.StoragePair{
				{
					Source: ref.Ref{ID: "ds-1"},
					Destination: v1beta1.DestinationStorage{
						StorageClass: "test-sc",
					},
					OffloadPlugin: &v1beta1.OffloadPlugin{
						VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
							StorageVendorProduct: "test-vendor",
							SecretRef:            "test-secret",
						},
					},
				},
			}
			storageMap := v1beta1.StorageMap{
				Spec: v1beta1.StorageMapSpec{
					Map: dsMap,
				},
			}
			builder.Source.Inventory = &mockInventory{
				ds: model.Datastore{Resource: model.Resource{ID: "ds-1"}},
				vm: vm,
			}
			builder.Map.Storage = &storageMap

			pvcs, err := builder.PopulatorVolumes(ref.Ref{ID: vm.ID}, map[string]string{"test-annotation": "true"}, "test-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))

			populator := &v1beta1.VSphereXcopyVolumePopulator{}
			err = builder.Destination.Get(context.TODO(), client.ObjectKey{Name: pvcs[0].Name, Namespace: pvcs[0].Namespace}, populator)
			Expect(err).NotTo(HaveOccurred())
			Expect(populator.Spec.MigrationHost).To(BeEmpty())
		})

		It("should set migrationHost when dedicatedMigrationHosts is set in storage mapping", func() {
			builder := createBuilder(
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "test-secret", Namespace: "test"},
				},
			)
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{
						ID:   "test-vm-id",
						Name: "test",
					},
					Disks: []vsphere.Disk{
						{
							Datastore: vsphere.Ref{ID: "ds-1"},
							File:      "[datastore1] vm-123/vm-123.vmdk",
							Bus:       vsphere.SCSI,
							Capacity:  1024 * 1024 * 1024,
							Key:       2000,
						},
					},
				},
			}

			dsMap := []v1beta1.StoragePair{
				{
					Source: ref.Ref{ID: "ds-1"},
					Destination: v1beta1.DestinationStorage{
						StorageClass: "test-sc",
					},
					OffloadPlugin: &v1beta1.OffloadPlugin{
						VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
							StorageVendorProduct:    "test-vendor",
							SecretRef:               "test-secret",
							DedicatedMigrationHosts: []string{"host1", "host2"},
						},
					},
				},
			}
			storageMap := v1beta1.StorageMap{
				Spec: v1beta1.StorageMapSpec{
					Map: dsMap,
				},
			}
			builder.Source.Inventory = &mockInventory{
				ds: model.Datastore{Resource: model.Resource{ID: "ds-1"}},
				vm: vm,
			}
			builder.Map.Storage = &storageMap

			pvcs, err := builder.PopulatorVolumes(ref.Ref{ID: vm.ID}, map[string]string{"test-annotation": "true"}, "test-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))

			populator := &v1beta1.VSphereXcopyVolumePopulator{}
			err = builder.Destination.Get(context.TODO(), client.ObjectKey{Name: pvcs[0].Name, Namespace: pvcs[0].Namespace}, populator)
			Expect(err).NotTo(HaveOccurred())
			Expect(populator.Spec.MigrationHost).To(BeElementOf("host1", "host2"))
		})
	})

	Context("formatHostAddress", func() {
		DescribeTable("should format addresses correctly", func(address string, expected string) {
			result := formatHostAddress(address)
			Expect(result).To(Equal(expected))
		},
			Entry("IPv4 address (no brackets)",
				"192.168.1.100",
				"192.168.1.100",
			),
			Entry("IPv6 address (add brackets)",
				"2001:db8::1",
				"[2001:db8::1]",
			),
			Entry("Invalid/Hostname (no change)",
				"not-an-ip",
				"not-an-ip",
			),
		)
	})

	Context("SourceVMLabelsAndAnnotations", func() {
		It("should convert all tags to labels when no tagMapping is provided", func() {
			builder := createBuilder()
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				},
				Tags: []vsphere.Tag{
					{Name: "owner", Description: "platform-team"},
					{Name: "environment", Description: "production"},
					{Name: "cost-center", Description: "cc-123"},
				},
			}
			builder.Source.Inventory = &mockInventory{vm: vm}

			labels, annotations, sanitizationReport, err := builder.SourceVMLabelsAndAnnotations(ref.Ref{ID: "vm-1"}, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(labels).To(HaveLen(3))
			Expect(labels["vsphere.forklift.konveyor.io/owner"]).To(Equal("platform-team"))
			Expect(labels["vsphere.forklift.konveyor.io/environment"]).To(Equal("production"))
			Expect(labels["vsphere.forklift.konveyor.io/cost-center"]).To(Equal("cc-123"))
			Expect(annotations).To(BeEmpty())
			Expect(sanitizationReport).To(BeEmpty())
		})

		It("should convert only specified tags to labels when tagMapping is provided", func() {
			builder := createBuilder()
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				},
				Tags: []vsphere.Tag{
					{Name: "owner", Description: "platform-team"},
					{Name: "environment", Description: "production"},
					{Name: "cost-center", Description: "cc-123"},
					{Name: "internal-tag", Description: "should-be-ignored"},
				},
			}
			builder.Source.Inventory = &mockInventory{vm: vm}

			tagMapping := &v1beta1.TagMapping{
				LabelTags: []string{"owner", "cost-center"},
			}
			labels, annotations, _, err := builder.SourceVMLabelsAndAnnotations(ref.Ref{ID: "vm-1"}, tagMapping)

			Expect(err).NotTo(HaveOccurred())
			Expect(labels).To(HaveLen(2))
			Expect(labels["vsphere.forklift.konveyor.io/owner"]).To(Equal("platform-team"))
			Expect(labels["vsphere.forklift.konveyor.io/cost-center"]).To(Equal("cc-123"))
			Expect(labels).NotTo(HaveKey("vsphere.forklift.konveyor.io/environment"))
			Expect(labels).NotTo(HaveKey("vsphere.forklift.konveyor.io/internal-tag"))
			Expect(annotations).To(BeEmpty())
		})

		It("should match tag names case-insensitively", func() {
			builder := createBuilder()
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				},
				Tags: []vsphere.Tag{
					{Name: "Owner", Description: "platform-team"},
					{Name: "ENVIRONMENT", Description: "production"},
				},
			}
			builder.Source.Inventory = &mockInventory{vm: vm}

			tagMapping := &v1beta1.TagMapping{
				LabelTags: []string{"owner", "environment"},
			}
			labels, _, _, err := builder.SourceVMLabelsAndAnnotations(ref.Ref{ID: "vm-1"}, tagMapping)

			Expect(err).NotTo(HaveOccurred())
			Expect(labels).To(HaveLen(2))
			Expect(labels["vsphere.forklift.konveyor.io/Owner"]).To(Equal("platform-team"))
			Expect(labels["vsphere.forklift.konveyor.io/ENVIRONMENT"]).To(Equal("production"))
		})

		It("should convert custom attributes to annotations", func() {
			builder := createBuilder()
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				},
				CustomDef: []vsphere.CustomFieldDef{
					{Key: 100, Name: "app-name"},
					{Key: 101, Name: "app-version"},
				},
				CustomValues: []vsphere.CustomFieldValue{
					{Key: 100, Value: "my-application"},
					{Key: 101, Value: "v1.2.3"},
				},
			}
			builder.Source.Inventory = &mockInventory{vm: vm}

			labels, annotations, _, err := builder.SourceVMLabelsAndAnnotations(ref.Ref{ID: "vm-1"}, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(labels).To(BeEmpty())
			Expect(annotations).To(HaveLen(2))
			Expect(annotations["vsphere.forklift.konveyor.io/app-name"]).To(Equal("my-application"))
			Expect(annotations["vsphere.forklift.konveyor.io/app-version"]).To(Equal("v1.2.3"))
		})

		It("should sanitize invalid tag names and descriptions and report them", func() {
			builder := createBuilder()
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				},
				Tags: []vsphere.Tag{
					{Name: "invalid tag name", Description: "invalid description value"},
					{Name: "valid-tag", Description: "valid-value"},
				},
			}
			builder.Source.Inventory = &mockInventory{vm: vm}

			labels, _, sanitizationReport, err := builder.SourceVMLabelsAndAnnotations(ref.Ref{ID: "vm-1"}, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(labels).To(HaveLen(2))
			Expect(labels["vsphere.forklift.konveyor.io/invalid_tag_name"]).To(Equal("invalid_description_value"))
			Expect(labels["vsphere.forklift.konveyor.io/valid-tag"]).To(Equal("valid-value"))
			Expect(sanitizationReport).To(HaveLen(2))
			Expect(sanitizationReport["tag.name.invalid tag name"]).To(Equal("invalid_tag_name"))
			Expect(sanitizationReport["tag.value.invalid tag name"]).To(Equal("invalid_description_value"))
		})

		It("should skip tags with empty names after sanitization", func() {
			builder := createBuilder()
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				},
				Tags: []vsphere.Tag{
					{Name: "", Description: "no-name"},
					{Name: "valid-tag", Description: "valid-value"},
				},
			}
			builder.Source.Inventory = &mockInventory{vm: vm}

			labels, _, _, err := builder.SourceVMLabelsAndAnnotations(ref.Ref{ID: "vm-1"}, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(labels).To(HaveLen(1))
			Expect(labels["vsphere.forklift.konveyor.io/valid-tag"]).To(Equal("valid-value"))
		})

		It("should convert both tags and custom attributes together", func() {
			builder := createBuilder()
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				},
				Tags: []vsphere.Tag{
					{Name: "owner", Description: "platform-team"},
				},
				CustomDef: []vsphere.CustomFieldDef{
					{Key: 100, Name: "app-name"},
				},
				CustomValues: []vsphere.CustomFieldValue{
					{Key: 100, Value: "my-application"},
				},
			}
			builder.Source.Inventory = &mockInventory{vm: vm}

			labels, annotations, _, err := builder.SourceVMLabelsAndAnnotations(ref.Ref{ID: "vm-1"}, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(labels).To(HaveLen(1))
			Expect(labels["vsphere.forklift.konveyor.io/owner"]).To(Equal("platform-team"))
			Expect(annotations).To(HaveLen(1))
			Expect(annotations["vsphere.forklift.konveyor.io/app-name"]).To(Equal("my-application"))
		})

		It("should convert no tags to labels when tagMapping.Disabled is true", func() {
			builder := createBuilder()
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				},
				Tags: []vsphere.Tag{
					{Name: "owner", Description: "platform-team"},
					{Name: "environment", Description: "production"},
				},
				CustomDef: []vsphere.CustomFieldDef{
					{Key: 100, Name: "app-name"},
				},
				CustomValues: []vsphere.CustomFieldValue{
					{Key: 100, Value: "my-application"},
				},
			}
			builder.Source.Inventory = &mockInventory{vm: vm}

			tagMapping := &v1beta1.TagMapping{
				Disabled: true,
			}
			labels, annotations, _, err := builder.SourceVMLabelsAndAnnotations(ref.Ref{ID: "vm-1"}, tagMapping)

			Expect(err).NotTo(HaveOccurred())
			Expect(labels).To(BeEmpty())
			Expect(annotations).To(HaveLen(1))
			Expect(annotations["vsphere.forklift.konveyor.io/app-name"]).To(Equal("my-application"))
		})

		It("should ignore LabelTags when tagMapping.Disabled is true", func() {
			builder := createBuilder()
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				},
				Tags: []vsphere.Tag{
					{Name: "owner", Description: "platform-team"},
					{Name: "environment", Description: "production"},
				},
			}
			builder.Source.Inventory = &mockInventory{vm: vm}

			tagMapping := &v1beta1.TagMapping{
				Disabled:  true,
				LabelTags: []string{"owner"},
			}
			labels, _, _, err := builder.SourceVMLabelsAndAnnotations(ref.Ref{ID: "vm-1"}, tagMapping)

			Expect(err).NotTo(HaveOccurred())
			Expect(labels).To(BeEmpty())
		})

		It("should convert all tags when tagMapping has empty LabelTags", func() {
			builder := createBuilder()
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				},
				Tags: []vsphere.Tag{
					{Name: "owner", Description: "platform-team"},
					{Name: "environment", Description: "production"},
				},
			}
			builder.Source.Inventory = &mockInventory{vm: vm}

			tagMapping := &v1beta1.TagMapping{
				LabelTags: []string{},
			}
			labels, _, _, err := builder.SourceVMLabelsAndAnnotations(ref.Ref{ID: "vm-1"}, tagMapping)

			Expect(err).NotTo(HaveOccurred())
			Expect(labels).To(HaveLen(2))
			Expect(labels["vsphere.forklift.konveyor.io/owner"]).To(Equal("platform-team"))
			Expect(labels["vsphere.forklift.konveyor.io/environment"]).To(Equal("production"))
		})

		It("should report sanitized custom attribute names", func() {
			builder := createBuilder()
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				},
				CustomDef: []vsphere.CustomFieldDef{
					{Key: 100, Name: "invalid attr name"},
				},
				CustomValues: []vsphere.CustomFieldValue{
					{Key: 100, Value: "some-value"},
				},
			}
			builder.Source.Inventory = &mockInventory{vm: vm}

			_, annotations, sanitizationReport, err := builder.SourceVMLabelsAndAnnotations(ref.Ref{ID: "vm-1"}, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(annotations).To(HaveLen(1))
			Expect(annotations["vsphere.forklift.konveyor.io/invalid_attr_name"]).To(Equal("some-value"))
			Expect(sanitizationReport).To(HaveLen(1))
			Expect(sanitizationReport["customAttribute.name.invalid attr name"]).To(Equal("invalid_attr_name"))
		})

		It("should skip custom attributes whose name sanitizes to empty", func() {
			builder := createBuilder()
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				},
				CustomDef: []vsphere.CustomFieldDef{
					{Key: 100, Name: "!@#$"},
					{Key: 101, Name: "valid-attr"},
				},
				CustomValues: []vsphere.CustomFieldValue{
					{Key: 100, Value: "should-be-skipped"},
					{Key: 101, Value: "kept"},
				},
			}
			builder.Source.Inventory = &mockInventory{vm: vm}

			_, annotations, _, err := builder.SourceVMLabelsAndAnnotations(ref.Ref{ID: "vm-1"}, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(annotations).To(HaveLen(1))
			Expect(annotations["vsphere.forklift.konveyor.io/valid-attr"]).To(Equal("kept"))
			Expect(annotations).NotTo(HaveKey("vsphere.forklift.konveyor.io/"))
		})
	})

	builder := createBuilder()
	DescribeTable("should", func(vm *model.VM, outputMap string) {
		Expect(builder.mapMacStaticIps(vm)).Should(Equal(outputMap))
	},
		Entry("no static ips", &model.VM{GuestID: "windows9Guest"}, ""),
		Entry("single static ip", &model.VM{
			GuestID: "windows9Guest",
			GuestNetworks: []vsphere.GuestNetwork{
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "172.29.3.193",
					Origin:       ManualOrigin,
					PrefixLength: 16,
					DNS:          []string{"8.8.8.8"},
				}},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "172.29.3.1",
					Network: "0.0.0.0",
				}},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.3.1,16,8.8.8.8"),
		Entry("multiple static ips", &model.VM{
			GuestID: "windows9Guest",
			GuestNetworks: []vsphere.GuestNetwork{
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "172.29.3.193",
					Origin:       ManualOrigin,
					PrefixLength: 16,
					DNS:          []string{"8.8.8.8"},
				},
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "fe80::5da:b7a5:e0a2:a097",
					Origin:       ManualOrigin,
					PrefixLength: 64,
					DNS:          []string{"fec0:0:0:ffff::1", "fec0:0:0:ffff::2", "fec0:0:0:ffff::3"},
				},
			},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "172.29.3.1",
					Network: "0.0.0.0",
				},
				{
					Gateway: "fe80::5da:b7a5:e0a2:a095",
					Network: "0.0.0.0",
				},
			},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.3.1,16,8.8.8.8_00:50:56:83:25:47:ip:fe80::5da:b7a5:e0a2:a097,fe80::5da:b7a5:e0a2:a095,64,fec0:0:0:ffff::1,fec0:0:0:ffff::2,fec0:0:0:ffff::3"),
		Entry("non-static ip", &model.VM{GuestID: "windows9Guest", GuestNetworks: []vsphere.GuestNetwork{{MAC: "00:50:56:83:25:47", IP: "172.29.3.193", Origin: string(types.NetIpConfigInfoIpAddressOriginDhcp)}}}, ""),
		Entry("non windows vm", &model.VM{GuestID: "other", GuestNetworks: []vsphere.GuestNetwork{{MAC: "00:50:56:83:25:47", IP: "172.29.3.193", Origin: ManualOrigin}}}, "00:50:56:83:25:47:ip:172.29.3.193,,0"),
		Entry("no OS vm", &model.VM{GuestNetworks: []vsphere.GuestNetwork{{MAC: "00:50:56:83:25:47", IP: "172.29.3.193", Origin: ManualOrigin}}}, "00:50:56:83:25:47:ip:172.29.3.193,,0"),
		Entry("multiple nics static ips", &model.VM{
			GuestID: "windows9Guest",
			GuestNetworks: []vsphere.GuestNetwork{
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "172.29.3.193",
					Origin:       ManualOrigin,
					PrefixLength: 16,
					DNS:          []string{"8.8.8.8"},
				},
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "fe80::5da:b7a5:e0a2:a097",
					Origin:       ManualOrigin,
					PrefixLength: 64,
					DNS:          []string{"fec0:0:0:ffff::1", "fec0:0:0:ffff::2", "fec0:0:0:ffff::3"},
				},
				{
					MAC:          "00:50:56:83:25:48",
					IP:           "172.29.3.192",
					Origin:       ManualOrigin,
					PrefixLength: 24,
					DNS:          []string{"4.4.4.4"},
				},
				{
					MAC:          "00:50:56:83:25:48",
					IP:           "fe80::5da:b7a5:e0a2:a090",
					Origin:       ManualOrigin,
					PrefixLength: 32,
					DNS:          []string{"fec0:0:0:ffff::4", "fec0:0:0:ffff::5", "fec0:0:0:ffff::6"},
				},
			},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "172.29.3.2",
					Network: "0.0.0.0",
				},
				{
					Gateway: "fe80::5da:b7a5:e0a2:a098",
					Network: "0.0.0.0",
				},
				{
					Gateway: "172.29.3.1",
					Network: "0.0.0.0",
				},
				{
					Gateway: "fe80::5da:b7a5:e0a2:a095",
					Network: "0.0.0.0",
				},
			},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.3.1,16,8.8.8.8_00:50:56:83:25:47:ip:fe80::5da:b7a5:e0a2:a097,fe80::5da:b7a5:e0a2:a095,64,fec0:0:0:ffff::1,fec0:0:0:ffff::2,fec0:0:0:ffff::3_00:50:56:83:25:48:ip:172.29.3.192,172.29.3.1,24,4.4.4.4_00:50:56:83:25:48:ip:fe80::5da:b7a5:e0a2:a090,fe80::5da:b7a5:e0a2:a095,32,fec0:0:0:ffff::4,fec0:0:0:ffff::5,fec0:0:0:ffff::6"),
		Entry("single static ip without DNS", &model.VM{
			GuestID: "windows9Guest",
			GuestNetworks: []vsphere.GuestNetwork{
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "172.29.3.193",
					Origin:       ManualOrigin,
					PrefixLength: 16,
				}},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "172.29.3.1",
					Network: "0.0.0.0",
				}},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.3.1,16"),
		Entry("gateway from different subnet", &model.VM{
			GuestID: "windows9Guest",
			GuestNetworks: []vsphere.GuestNetwork{
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "172.29.3.193",
					Origin:       ManualOrigin,
					PrefixLength: 24,
					DNS:          []string{"8.8.8.8"},
				}},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "172.29.4.1",
					Network: "0.0.0.0",
				}},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.4.1,24,8.8.8.8"),
		Entry("multiple gateways with different networks", &model.VM{
			GuestID: "windows9Guest",
			GuestNetworks: []vsphere.GuestNetwork{
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "172.29.3.193",
					Origin:       ManualOrigin,
					PrefixLength: 24,
					DNS:          []string{"8.8.8.8"},
				}},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "10.10.10.2",
					Network: "10.10.10.1",
				},
				{
					Gateway: "172.29.3.1",
					Network: "0.0.0.0",
				},
				{
					Gateway: "10.10.10.1",
					Network: "10.10.10.0",
				}},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.3.1,24,8.8.8.8"),
	)

	DescribeTable("should", func(disks []vsphere.Disk, output []vsphere.Disk) {
		vm := &model.VM1{}
		vm.Disks = disks
		Expect(vm.SortedDisksAsLibvirt()).Should(Equal(output))
	},
		Entry("sort all disks by buses",
			[]vsphere.Disk{
				{Key: 1, Bus: vsphere.IDE},
				{Key: 1, Bus: vsphere.SATA},
				{Key: 1, Bus: vsphere.SCSI},
				{Key: 2, Bus: vsphere.SCSI},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: vsphere.SCSI},
				{Key: 2, Bus: vsphere.SCSI},
				{Key: 1, Bus: vsphere.SATA},
				{Key: 1, Bus: vsphere.IDE},
			},
		),
		Entry("sort IDE and SATA disks by buses",
			[]vsphere.Disk{
				{Key: 1, Bus: vsphere.IDE},
				{Key: 1, Bus: vsphere.SATA},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: vsphere.SATA},
				{Key: 1, Bus: vsphere.IDE},
			},
		),
		Entry("sort multiple SATA disks by buses",
			[]vsphere.Disk{
				{Key: 3, Bus: vsphere.SATA},
				{Key: 1, Bus: vsphere.SATA},
				{Key: 2, Bus: vsphere.SATA},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: vsphere.SATA},
				{Key: 2, Bus: vsphere.SATA},
				{Key: 3, Bus: vsphere.SATA},
			},
		),
		Entry("sort multiple SATA and multiple SCSI disks by buses",
			[]vsphere.Disk{
				{Key: 3, Bus: vsphere.SATA},
				{Key: 3, Bus: vsphere.SCSI},
				{Key: 2, Bus: vsphere.SCSI},
				{Key: 1, Bus: vsphere.SATA},
				{Key: 2, Bus: vsphere.SATA},
				{Key: 1, Bus: vsphere.SCSI},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: vsphere.SCSI},
				{Key: 2, Bus: vsphere.SCSI},
				{Key: 3, Bus: vsphere.SCSI},
				{Key: 1, Bus: vsphere.SATA},
				{Key: 2, Bus: vsphere.SATA},
				{Key: 3, Bus: vsphere.SATA},
			},
		),
		Entry("sort multiple all disks by buses",
			[]vsphere.Disk{
				{Key: 2, Bus: vsphere.IDE},
				{Key: 3, Bus: vsphere.SATA},
				{Key: 3, Bus: vsphere.SCSI},
				{Key: 2, Bus: vsphere.SCSI},
				{Key: 3, Bus: vsphere.IDE},
				{Key: 1, Bus: vsphere.SATA},
				{Key: 2, Bus: vsphere.SATA},
				{Key: 1, Bus: vsphere.SCSI},
				{Key: 1, Bus: vsphere.IDE},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: vsphere.SCSI},
				{Key: 2, Bus: vsphere.SCSI},
				{Key: 3, Bus: vsphere.SCSI},
				{Key: 1, Bus: vsphere.SATA},
				{Key: 2, Bus: vsphere.SATA},
				{Key: 3, Bus: vsphere.SATA},
				{Key: 1, Bus: vsphere.IDE},
				{Key: 2, Bus: vsphere.IDE},
				{Key: 3, Bus: vsphere.IDE},
			},
		),
		Entry("sort NVMe disks with other buses",
			[]vsphere.Disk{
				{Key: 1, Bus: vsphere.NVME},
				{Key: 1, Bus: vsphere.SCSI},
				{Key: 2, Bus: vsphere.NVME},
				{Key: 1, Bus: vsphere.SATA},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: vsphere.SCSI},
				{Key: 1, Bus: vsphere.SATA},
				{Key: 1, Bus: vsphere.NVME},
				{Key: 2, Bus: vsphere.NVME},
			},
		),
		Entry("sort multiple NVMe disks by key",
			[]vsphere.Disk{
				{Key: 3, Bus: vsphere.NVME},
				{Key: 1, Bus: vsphere.NVME},
				{Key: 2, Bus: vsphere.NVME},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: vsphere.NVME},
				{Key: 2, Bus: vsphere.NVME},
				{Key: 3, Bus: vsphere.NVME},
			},
		),
		Entry("sort all disk types including NVMe",
			[]vsphere.Disk{
				{Key: 2, Bus: vsphere.NVME},
				{Key: 2, Bus: vsphere.IDE},
				{Key: 3, Bus: vsphere.SATA},
				{Key: 3, Bus: vsphere.SCSI},
				{Key: 1, Bus: vsphere.NVME},
				{Key: 2, Bus: vsphere.SCSI},
				{Key: 3, Bus: vsphere.IDE},
				{Key: 1, Bus: vsphere.SATA},
				{Key: 2, Bus: vsphere.SATA},
				{Key: 1, Bus: vsphere.SCSI},
				{Key: 1, Bus: vsphere.IDE},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: vsphere.SCSI},
				{Key: 2, Bus: vsphere.SCSI},
				{Key: 3, Bus: vsphere.SCSI},
				{Key: 1, Bus: vsphere.SATA},
				{Key: 2, Bus: vsphere.SATA},
				{Key: 3, Bus: vsphere.SATA},
				{Key: 1, Bus: vsphere.IDE},
				{Key: 2, Bus: vsphere.IDE},
				{Key: 3, Bus: vsphere.IDE},
				{Key: 1, Bus: vsphere.NVME},
				{Key: 2, Bus: vsphere.NVME},
			},
		),
		Entry("sort SCSI disks by controller key then unit when device keys are not monotonic",
			[]vsphere.Disk{
				{Key: 2000, Bus: vsphere.SCSI, ControllerKey: 1001, UnitNumber: 0},
				{Key: 1800, Bus: vsphere.SCSI, ControllerKey: 1000, UnitNumber: 2},
				{Key: 1700, Bus: vsphere.SCSI, ControllerKey: 1000, UnitNumber: 0},
			},
			[]vsphere.Disk{
				{Key: 1700, Bus: vsphere.SCSI, ControllerKey: 1000, UnitNumber: 0},
				{Key: 1800, Bus: vsphere.SCSI, ControllerKey: 1000, UnitNumber: 2},
				{Key: 2000, Bus: vsphere.SCSI, ControllerKey: 1001, UnitNumber: 0},
			},
		),
	)

	Context("mapNetworks Calico annotations", func() {
		const (
			netID     = "net-id-1"
			netKey    = "net-key-1"
			ifname    = "net-0"
			nicMAC    = "aa:bb:cc:dd:ee:01"
			nicIP     = "10.0.0.5"
			nadName   = "calico-l2-nad"
			hwAnnKey  = "cni.projectcalico.org/net-0.hwAddr"
			ipsAnnKey = "cni.projectcalico.org/net-0.ipAddrs"
		)
		buildAndCall := func(nadConfig string, preserveIPs bool) (map[string]string, error) {
			nad := &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: meta.ObjectMeta{Namespace: "test", Name: nadName},
				Spec:       k8snet.NetworkAttachmentDefinitionSpec{Config: nadConfig},
			}
			builder := createBuilder(nad)
			builder.Source.Inventory = &mockInventory{
				networks: map[string]model.Network{
					netID: {Resource: model.Resource{ID: netID}, Variant: vsphere.NetDvPortGroup, Key: netKey},
				},
			}
			builder.Plan.Spec.PreserveStaticIPs = preserveIPs
			builder.Context.Map.Network = &v1beta1.NetworkMap{
				Spec: v1beta1.NetworkMapSpec{
					Map: []v1beta1.NetworkPair{{
						Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: netID}},
						Destination: v1beta1.DestinationNetwork{
							Type: "multus", Namespace: "test", Name: nadName,
						},
					}},
				},
			}
			vm := &model.VM{
				NICs: []vsphere.NIC{{
					Network:   vsphere.Ref{ID: netKey},
					MAC:       nicMAC,
					DeviceKey: 4001,
				}},
				GuestNetworks: []vsphere.GuestNetwork{{
					MAC:            nicMAC,
					IP:             nicIP,
					DeviceConfigId: 4001,
					Origin:         ManualOrigin,
					PrefixLength:   24,
				}},
			}
			spec := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}
			err := builder.mapNetworks(vm, spec)
			return spec.Template.ObjectMeta.Annotations, err
		}

		It("emits MAC and IP annotations for a Calico L2 NAD with PreserveStaticIPs", func() {
			annotations, err := buildAndCall(`{"type":"calico","network":"datacenter-vlans","vlan":100}`, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(annotations).To(HaveKeyWithValue(hwAnnKey, nicMAC))
			Expect(annotations).To(HaveKeyWithValue(ipsAnnKey, fmt.Sprintf(`["%s"]`, nicIP)))
		})

		It("emits MAC only when PreserveStaticIPs is false", func() {
			annotations, err := buildAndCall(`{"type":"calico","network":"datacenter-vlans"}`, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(annotations).To(HaveKeyWithValue(hwAnnKey, nicMAC))
			Expect(annotations).NotTo(HaveKey(ipsAnnKey))
		})

		It("emits nothing for a Calico L3 NAD (no network field)", func() {
			annotations, err := buildAndCall(`{"type":"calico"}`, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(annotations).NotTo(HaveKey(hwAnnKey))
			Expect(annotations).NotTo(HaveKey(ipsAnnKey))
		})

		It("emits nothing for an OVN-K UDN NAD", func() {
			annotations, err := buildAndCall(`{"type":"ovn-k8s-cni-overlay","subnets":"10.0.0.0/24"}`, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(annotations).NotTo(HaveKey(hwAnnKey))
			Expect(annotations).NotTo(HaveKey(ipsAnnKey))
		})

		It("emits nothing for a NAD with empty Spec.Config", func() {
			annotations, err := buildAndCall("", true)
			Expect(err).NotTo(HaveOccurred())
			Expect(annotations).NotTo(HaveKey(hwAnnKey))
			Expect(annotations).NotTo(HaveKey(ipsAnnKey))
		})

		// Multi-NIC support — the builder iterates vm.NICs and emits a per-NIC
		// kInterface (net-0, net-1, …) plus per-interface Calico annotations.
		// runMulti is a more general helper accepting one entry per NIC.
		type nicSpec struct {
			netID     string // source network ID (matched via mockInventory)
			mac       string
			deviceKey int32
			ip        string // empty → no GuestNetworks entry for this NIC
		}
		type nadSpec struct {
			name   string
			config string
		}
		runMulti := func(nics []nicSpec, nads []nadSpec, preserveIPs bool, mapPairs []v1beta1.NetworkPair) (map[string]string, error) {
			objs := []runtime.Object{}
			for _, n := range nads {
				objs = append(objs, &k8snet.NetworkAttachmentDefinition{
					ObjectMeta: meta.ObjectMeta{Namespace: "test", Name: n.name},
					Spec:       k8snet.NetworkAttachmentDefinitionSpec{Config: n.config},
				})
			}
			builder := createBuilder(objs...)
			networks := map[string]model.Network{}
			for _, n := range nics {
				networks[n.netID] = model.Network{
					Resource: model.Resource{ID: n.netID},
					Variant:  vsphere.NetDvPortGroup,
					Key:      n.netID, // match by ID in buildNICResolver
				}
			}
			builder.Source.Inventory = &mockInventory{networks: networks}
			builder.Plan.Spec.PreserveStaticIPs = preserveIPs
			builder.Context.Map.Network = &v1beta1.NetworkMap{
				Spec: v1beta1.NetworkMapSpec{Map: mapPairs},
			}
			vmNICs := make([]vsphere.NIC, 0, len(nics))
			vmGuestNets := []vsphere.GuestNetwork{}
			for _, n := range nics {
				vmNICs = append(vmNICs, vsphere.NIC{
					Network:   vsphere.Ref{ID: n.netID},
					MAC:       n.mac,
					DeviceKey: n.deviceKey,
				})
				if n.ip != "" {
					vmGuestNets = append(vmGuestNets, vsphere.GuestNetwork{
						MAC:            n.mac,
						IP:             n.ip,
						DeviceConfigId: n.deviceKey,
						Origin:         ManualOrigin,
						PrefixLength:   24,
					})
				}
			}
			vm := &model.VM{NICs: vmNICs, GuestNetworks: vmGuestNets}
			spec := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}
			err := builder.mapNetworks(vm, spec)
			return spec.Template.ObjectMeta.Annotations, err
		}
		calicoL2 := func(name string) string {
			return fmt.Sprintf(`{"type":"calico","network":"%s"}`, name)
		}

		It("emits per-interface annotations for 3 NICs on 3 distinct Calico L2 NADs", func() {
			nics := []nicSpec{
				{netID: "src-a", mac: "aa:bb:cc:00:00:01", deviceKey: 4001, ip: "10.0.0.5"},
				{netID: "src-b", mac: "aa:bb:cc:00:00:02", deviceKey: 4002, ip: "10.0.1.5"},
				{netID: "src-c", mac: "aa:bb:cc:00:00:03", deviceKey: 4003, ip: "10.0.2.5"},
			}
			nads := []nadSpec{
				{name: "nad-a", config: calicoL2("net-a")},
				{name: "nad-b", config: calicoL2("net-b")},
				{name: "nad-c", config: calicoL2("net-c")},
			}
			pairs := []v1beta1.NetworkPair{
				{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-a"}}, Destination: v1beta1.DestinationNetwork{Type: "multus", Namespace: "test", Name: "nad-a"}},
				{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-b"}}, Destination: v1beta1.DestinationNetwork{Type: "multus", Namespace: "test", Name: "nad-b"}},
				{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-c"}}, Destination: v1beta1.DestinationNetwork{Type: "multus", Namespace: "test", Name: "nad-c"}},
			}
			ann, err := runMulti(nics, nads, true, pairs)
			Expect(err).NotTo(HaveOccurred())
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/net-0.hwAddr", "aa:bb:cc:00:00:01"))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/net-1.hwAddr", "aa:bb:cc:00:00:02"))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/net-2.hwAddr", "aa:bb:cc:00:00:03"))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/net-0.ipAddrs", `["10.0.0.5"]`))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/net-1.ipAddrs", `["10.0.1.5"]`))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/net-2.ipAddrs", `["10.0.2.5"]`))
		})

		It("emits 3 distinct annotation sets when multiple NICs reference the same Calico Network via distinct NADs", func() {
			// Each NIC gets its own NAD via the NAD pool; all three NADs name
			// the same Calico Network. Invariant under test: per-interface
			// annotation keys (net-0/net-1/net-2) stay distinct even when the
			// underlying Calico Network is shared.
			nics := []nicSpec{
				{netID: "src-a", mac: "aa:bb:cc:00:00:01", deviceKey: 4001},
				{netID: "src-b", mac: "aa:bb:cc:00:00:02", deviceKey: 4002},
				{netID: "src-c", mac: "aa:bb:cc:00:00:03", deviceKey: 4003},
			}
			nads := []nadSpec{
				{name: "calico-nad-a", config: calicoL2("shared-net")},
				{name: "calico-nad-b", config: calicoL2("shared-net")},
				{name: "calico-nad-c", config: calicoL2("shared-net")},
			}
			pairs := []v1beta1.NetworkPair{
				{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-a"}}, Destination: v1beta1.DestinationNetwork{Type: "multus", Namespace: "test", Name: "calico-nad-a"}},
				{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-b"}}, Destination: v1beta1.DestinationNetwork{Type: "multus", Namespace: "test", Name: "calico-nad-b"}},
				{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-c"}}, Destination: v1beta1.DestinationNetwork{Type: "multus", Namespace: "test", Name: "calico-nad-c"}},
			}
			ann, err := runMulti(nics, nads, false, pairs)
			Expect(err).NotTo(HaveOccurred())
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/net-0.hwAddr", "aa:bb:cc:00:00:01"))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/net-1.hwAddr", "aa:bb:cc:00:00:02"))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/net-2.hwAddr", "aa:bb:cc:00:00:03"))
		})

		It("gates per-NIC: emits annotations only for the NICs whose destination NAD is Calico L2", func() {
			// NIC 0: Calico L2 → expect annotations.
			// NIC 1: plain Multus, no CNI config → expect nothing.
			// NIC 2: Calico L3 (type=calico but no `network` field) → expect nothing.
			nics := []nicSpec{
				{netID: "src-calico", mac: "aa:bb:cc:00:00:01", deviceKey: 4001},
				{netID: "src-plain", mac: "aa:bb:cc:00:00:02", deviceKey: 4002},
				{netID: "src-l3", mac: "aa:bb:cc:00:00:03", deviceKey: 4003},
			}
			nads := []nadSpec{
				{name: "calico-nad", config: calicoL2("net-a")},
				{name: "plain-nad", config: ""},
				{name: "calico-l3-nad", config: `{"type":"calico"}`},
			}
			pairs := []v1beta1.NetworkPair{
				{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-calico"}}, Destination: v1beta1.DestinationNetwork{Type: "multus", Namespace: "test", Name: "calico-nad"}},
				{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-plain"}}, Destination: v1beta1.DestinationNetwork{Type: "multus", Namespace: "test", Name: "plain-nad"}},
				{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-l3"}}, Destination: v1beta1.DestinationNetwork{Type: "multus", Namespace: "test", Name: "calico-l3-nad"}},
			}
			ann, err := runMulti(nics, nads, false, pairs)
			Expect(err).NotTo(HaveOccurred())
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/net-0.hwAddr", "aa:bb:cc:00:00:01"))
			Expect(ann).NotTo(HaveKey("cni.projectcalico.org/net-1.hwAddr"))
			Expect(ann).NotTo(HaveKey("cni.projectcalico.org/net-2.hwAddr"))
		})
	})

	Context("mapNetworks Calico primary annotations", func() {
		const (
			netID  = "src-primary"
			nicMAC = "aa:bb:cc:dd:ee:01"
			nicIP  = "10.244.0.5"
		)
		// runPrimary builds a Plan with a single type: calico NetworkMap
		// entry and runs mapNetworks against a one-NIC VM. extraPair is
		// appended to the NetworkMap (for mixed primary+secondary tests).
		runPrimary := func(dest v1beta1.DestinationNetwork, preserveIPs bool, extraPairs []v1beta1.NetworkPair, extraNICs []vsphere.NIC, extraGuestNets []vsphere.GuestNetwork, k8sObjs ...runtime.Object) (*cnv.VirtualMachineSpec, error) {
			builder := createBuilder(k8sObjs...)
			networks := map[string]model.Network{
				netID: {Resource: model.Resource{ID: netID}, Variant: vsphere.NetDvPortGroup, Key: netID},
			}
			for _, p := range extraPairs {
				networks[p.Source.ID] = model.Network{
					Resource: model.Resource{ID: p.Source.ID},
					Variant:  vsphere.NetDvPortGroup,
					Key:      p.Source.ID,
				}
			}
			builder.Source.Inventory = &mockInventory{networks: networks}
			builder.Plan.Spec.PreserveStaticIPs = preserveIPs
			pairs := []v1beta1.NetworkPair{
				{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: netID}}, Destination: dest},
			}
			pairs = append(pairs, extraPairs...)
			builder.Context.Map.Network = &v1beta1.NetworkMap{Spec: v1beta1.NetworkMapSpec{Map: pairs}}
			nics := []vsphere.NIC{{Network: vsphere.Ref{ID: netID}, MAC: nicMAC, DeviceKey: 4001}}
			nics = append(nics, extraNICs...)
			gnets := []vsphere.GuestNetwork{{MAC: nicMAC, IP: nicIP, DeviceConfigId: 4001, Origin: ManualOrigin, PrefixLength: 24}}
			gnets = append(gnets, extraGuestNets...)
			vm := &model.VM{NICs: nics, GuestNetworks: gnets}
			spec := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}
			err := builder.mapNetworks(vm, spec)
			return spec, err
		}

		It("Case A no preserveStaticIPs: MAC only, Bridge binding, no IP/Network", func() {
			spec, err := runPrimary(v1beta1.DestinationNetwork{Type: "pod", Calico: &v1beta1.CalicoDestination{}}, false, nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			ann := spec.Template.ObjectMeta.Annotations
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/hwAddr", nicMAC))
			Expect(ann).NotTo(HaveKey("cni.projectcalico.org/ipAddrs"))
			Expect(ann).NotTo(HaveKey("cni.projectcalico.org/networks"))
			// Bridge on the pod network blocks live migration unless the VM
			// opts in; calico-primary VMs are opted in automatically.
			Expect(ann).To(HaveKeyWithValue("kubevirt.io/allow-pod-bridge-network-live-migration", "true"))
			// Bridge binding always for calico-flagged mappings.
			Expect(spec.Template.Spec.Domain.Devices.Interfaces).To(HaveLen(1))
			Expect(spec.Template.Spec.Domain.Devices.Interfaces[0].Bridge).NotTo(BeNil())
			// Pod network for the cluster-default CNI attach.
			Expect(spec.Template.Spec.Networks).To(HaveLen(1))
			Expect(spec.Template.Spec.Networks[0].Pod).NotTo(BeNil())
		})

		It("plain type: pod (no calico) gets no calico or live-migration annotations", func() {
			spec, err := runPrimary(v1beta1.DestinationNetwork{Type: "pod"}, true, nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			ann := spec.Template.ObjectMeta.Annotations
			Expect(ann).NotTo(HaveKey("cni.projectcalico.org/hwAddr"))
			Expect(ann).NotTo(HaveKey("kubevirt.io/allow-pod-bridge-network-live-migration"))
		})

		It("Case A with preserveStaticIPs: MAC + IPs", func() {
			spec, err := runPrimary(v1beta1.DestinationNetwork{Type: "pod", Calico: &v1beta1.CalicoDestination{}}, true, nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			ann := spec.Template.ObjectMeta.Annotations
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/hwAddr", nicMAC))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/ipAddrs", fmt.Sprintf(`["%s"]`, nicIP)))
			Expect(ann).NotTo(HaveKey("cni.projectcalico.org/networks"))
		})

		It("L2 attach (network + vlan), no preserveStaticIPs: MAC + Network + vlan, no IPs", func() {
			spec, err := runPrimary(
				v1beta1.DestinationNetwork{Type: "pod", Calico: &v1beta1.CalicoDestination{Network: "datacenter-vlans", Vlan: 100}},
				false, nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			ann := spec.Template.ObjectMeta.Annotations
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/hwAddr", nicMAC))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/networks", "datacenter-vlans"))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/vlan", "100"))
			Expect(ann).NotTo(HaveKey("cni.projectcalico.org/ipAddrs"))
		})

		It("L2 attach (network + vlan) with preserveStaticIPs: MAC + IPs + Network + vlan", func() {
			spec, err := runPrimary(
				v1beta1.DestinationNetwork{Type: "pod", Calico: &v1beta1.CalicoDestination{Network: "datacenter-vlans", Vlan: 200}},
				true, nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			ann := spec.Template.ObjectMeta.Annotations
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/hwAddr", nicMAC))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/ipAddrs", fmt.Sprintf(`["%s"]`, nicIP)))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/networks", "datacenter-vlans"))
			// Calico's VLAN selection is annotation-driven; the user's
			// validated choice must reach the pod.
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/vlan", "200"))
		})

		It("Mixed: calico-flagged primary + type:multus secondary coexist with correct scoping", func() {
			// Primary NIC on src-primary, secondary NIC on src-secondary
			// (mapped to a Calico L2 NAD). Annotations: unscoped for primary,
			// VMI-network-name-scoped for secondary (Calico reverse-maps the
			// pod interface name back to the VMI network name).
			secondaryNAD := &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: meta.ObjectMeta{Namespace: "test", Name: "secondary-calico-nad"},
				Spec:       k8snet.NetworkAttachmentDefinitionSpec{Config: `{"type":"calico","network":"sec-vlan","vlan":100}`},
			}
			extraPair := v1beta1.NetworkPair{
				Source:      v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-secondary"}},
				Destination: v1beta1.DestinationNetwork{Type: "multus", Namespace: "test", Name: "secondary-calico-nad"},
			}
			extraNIC := vsphere.NIC{Network: vsphere.Ref{ID: "src-secondary"}, MAC: "aa:bb:cc:00:00:99", DeviceKey: 4002}
			extraGN := vsphere.GuestNetwork{MAC: "aa:bb:cc:00:00:99", IP: "10.100.0.5", DeviceConfigId: 4002, Origin: ManualOrigin, PrefixLength: 24}
			spec, err := runPrimary(
				v1beta1.DestinationNetwork{Type: "pod", Calico: &v1beta1.CalicoDestination{Network: "primary-net", Vlan: 100}},
				true,
				[]v1beta1.NetworkPair{extraPair},
				[]vsphere.NIC{extraNIC},
				[]vsphere.GuestNetwork{extraGN},
				secondaryNAD)
			Expect(err).NotTo(HaveOccurred())
			ann := spec.Template.ObjectMeta.Annotations
			// Primary (unscoped) annotations.
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/hwAddr", nicMAC))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/networks", "primary-net"))
			// Secondary (per-iface) annotations on net-1 (primary NIC is net-0).
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/net-1.hwAddr", "aa:bb:cc:00:00:99"))
			Expect(ann).To(HaveKeyWithValue("cni.projectcalico.org/net-1.ipAddrs", `["10.100.0.5"]`))
		})
	})
})

var _ = Describe("PopulatorXcopyUsed", func() {
	It("should return xcopyUsed when populator CR has the field set", func() {
		populatorCr := &v1beta1.VSphereXcopyVolumePopulator{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-pop",
				Namespace: "test",
				Labels: map[string]string{
					"migration": "123",
					"vmdkKey":   "2000",
					"vmID":      "vm-1",
				},
			},
			Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
				VmId: "vm-1",
			},
			Status: v1beta1.VSphereXcopyVolumePopulatorStatus{
				Progress:  "100",
				XcopyUsed: "1",
			},
		}
		builder := createBuilder(populatorCr)
		pvc := &core.PersistentVolumeClaim{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-pvc",
				Namespace: "test",
				Labels: map[string]string{
					"vmdkKey": "2000",
					"vmID":    "vm-1",
				},
			},
		}

		xcopyUsed, found, err := builder.PopulatorXcopyUsed(pvc)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(xcopyUsed).To(Equal("1"))
	})

	It("should return found=false when xcopyUsed is empty", func() {
		populatorCr := &v1beta1.VSphereXcopyVolumePopulator{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-pop",
				Namespace: "test",
				Labels: map[string]string{
					"migration": "123",
					"vmdkKey":   "2000",
					"vmID":      "vm-1",
				},
			},
			Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
				VmId: "vm-1",
			},
			Status: v1beta1.VSphereXcopyVolumePopulatorStatus{
				Progress: "50",
			},
		}
		builder := createBuilder(populatorCr)
		pvc := &core.PersistentVolumeClaim{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-pvc",
				Namespace: "test",
				Labels: map[string]string{
					"vmdkKey": "2000",
					"vmID":    "vm-1",
				},
			},
		}

		xcopyUsed, found, err := builder.PopulatorXcopyUsed(pvc)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(xcopyUsed).To(BeEmpty())
	})

	It("should return error when populator CR is not found", func() {
		builder := createBuilder()
		pvc := &core.PersistentVolumeClaim{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-pvc",
				Namespace: "test",
				Labels: map[string]string{
					"vmdkKey": "2000",
					"vmID":    "vm-1",
				},
			},
		}

		_, _, err := builder.PopulatorXcopyUsed(pvc)
		Expect(err).To(HaveOccurred())
	})

	It("should return xcopyUsed=0 when xcopy was not used", func() {
		populatorCr := &v1beta1.VSphereXcopyVolumePopulator{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-pop",
				Namespace: "test",
				Labels: map[string]string{
					"migration": "123",
					"vmdkKey":   "2000",
					"vmID":      "vm-1",
				},
			},
			Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
				VmId: "vm-1",
			},
			Status: v1beta1.VSphereXcopyVolumePopulatorStatus{
				Progress:  "100",
				XcopyUsed: "0",
			},
		}
		builder := createBuilder(populatorCr)
		pvc := &core.PersistentVolumeClaim{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-pvc",
				Namespace: "test",
				Labels: map[string]string{
					"vmdkKey": "2000",
					"vmID":    "vm-1",
				},
			},
		}

		xcopyUsed, found, err := builder.PopulatorXcopyUsed(pvc)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(xcopyUsed).To(Equal("0"))
	})
})

var _ = Describe("mapDisks SCSI reservation", func() {
	var builder *Builder

	BeforeEach(func() {
		builder = createBuilder()
	})

	buildPVC := func(diskFile string) *core.PersistentVolumeClaim {
		return &core.PersistentVolumeClaim{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-pvc",
				Namespace: "test",
				Annotations: map[string]string{
					"forklift.konveyor.io/disk-source": diskFile,
				},
			},
		}
	}

	It("should set Reservation and ErrorPolicy on shared RDM LUN disk when scsiReservation is enabled", func() {
		builder.Plan.Spec.RDMAsLun = true
		builder.Plan.Spec.SCSIReservation = true
		vm := &model.VM{
			VM1: model.VM1{
				VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{
					{
						Key:      2000,
						File:     "[ds1] vm/disk.vmdk",
						Capacity: 1 << 30,
						RDM:      true,
						Shared:   true,
						Bus:      vsphere.SCSI,
					},
				},
			},
		}
		pvcs := []*core.PersistentVolumeClaim{buildPVC("[ds1] vm/disk.vmdk")}
		spec := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}

		err := builder.mapDisks(vm, ref.Ref{ID: "vm-1"}, pvcs, spec, false)
		Expect(err).NotTo(HaveOccurred())

		Expect(spec.Template.Spec.Domain.Devices.Disks).To(HaveLen(1))
		disk := spec.Template.Spec.Domain.Devices.Disks[0]
		Expect(disk.LUN).NotTo(BeNil())
		Expect(disk.LUN.Bus).To(Equal(cnv.DiskBusSCSI))
		Expect(disk.LUN.Reservation).To(BeTrue())
		Expect(disk.Shareable).To(Equal(ptr.To(true)))
		Expect(disk.ErrorPolicy).To(Equal(ptr.To(cnv.DiskErrorPolicyReport)))
	})

	It("should NOT set Reservation when disk is RDM+LUN but not shared", func() {
		builder.Plan.Spec.RDMAsLun = true
		vm := &model.VM{
			VM1: model.VM1{
				VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{
					{
						Key:      2000,
						File:     "[ds1] vm/disk.vmdk",
						Capacity: 1 << 30,
						RDM:      true,
						Shared:   false,
						Bus:      vsphere.SCSI,
					},
				},
			},
		}
		pvcs := []*core.PersistentVolumeClaim{buildPVC("[ds1] vm/disk.vmdk")}
		spec := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}

		err := builder.mapDisks(vm, ref.Ref{ID: "vm-1"}, pvcs, spec, false)
		Expect(err).NotTo(HaveOccurred())

		disk := spec.Template.Spec.Domain.Devices.Disks[0]
		Expect(disk.LUN).NotTo(BeNil())
		Expect(disk.LUN.Reservation).To(BeFalse())
		Expect(disk.Shareable).To(BeNil())
		Expect(disk.ErrorPolicy).To(BeNil())
	})

	It("should NOT set LUN or Reservation when disk is shared but not RDM", func() {
		builder.Plan.Spec.RDMAsLun = true
		vm := &model.VM{
			VM1: model.VM1{
				VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{
					{
						Key:      2000,
						File:     "[ds1] vm/disk.vmdk",
						Capacity: 1 << 30,
						RDM:      false,
						Shared:   true,
						Bus:      vsphere.SCSI,
					},
				},
			},
		}
		pvcs := []*core.PersistentVolumeClaim{buildPVC("[ds1] vm/disk.vmdk")}
		spec := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}

		err := builder.mapDisks(vm, ref.Ref{ID: "vm-1"}, pvcs, spec, false)
		Expect(err).NotTo(HaveOccurred())

		disk := spec.Template.Spec.Domain.Devices.Disks[0]
		Expect(disk.LUN).To(BeNil())
		Expect(disk.Disk).NotTo(BeNil())
		Expect(disk.Shareable).To(Equal(ptr.To(true)))
		Expect(disk.ErrorPolicy).To(BeNil())
	})

	It("should NOT set Reservation when rdmAsLun is false", func() {
		builder.Plan.Spec.RDMAsLun = false
		vm := &model.VM{
			VM1: model.VM1{
				VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{
					{
						Key:      2000,
						File:     "[ds1] vm/disk.vmdk",
						Capacity: 1 << 30,
						RDM:      true,
						Shared:   true,
						Bus:      vsphere.SCSI,
					},
				},
			},
		}
		pvcs := []*core.PersistentVolumeClaim{buildPVC("[ds1] vm/disk.vmdk")}
		spec := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}

		err := builder.mapDisks(vm, ref.Ref{ID: "vm-1"}, pvcs, spec, false)
		Expect(err).NotTo(HaveOccurred())

		disk := spec.Template.Spec.Domain.Devices.Disks[0]
		Expect(disk.LUN).To(BeNil())
		Expect(disk.Disk).NotTo(BeNil())
		Expect(disk.Shareable).To(Equal(ptr.To(true)))
		Expect(disk.ErrorPolicy).To(BeNil())
	})

	It("should NOT set Reservation on shared RDM LUN disk when scsiReservation flag is disabled (default)", func() {
		builder.Plan.Spec.RDMAsLun = true
		// SCSIReservation defaults to false — reservation must stay off
		vm := &model.VM{
			VM1: model.VM1{
				VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{
					{
						Key:      2000,
						File:     "[ds1] vm/disk.vmdk",
						Capacity: 1 << 30,
						RDM:      true,
						Shared:   true,
						Bus:      vsphere.SCSI,
					},
				},
			},
		}
		pvcs := []*core.PersistentVolumeClaim{buildPVC("[ds1] vm/disk.vmdk")}
		spec := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}

		err := builder.mapDisks(vm, ref.Ref{ID: "vm-1"}, pvcs, spec, false)
		Expect(err).NotTo(HaveOccurred())

		disk := spec.Template.Spec.Domain.Devices.Disks[0]
		Expect(disk.LUN).NotTo(BeNil())
		Expect(disk.LUN.Reservation).To(BeFalse())
		Expect(disk.Shareable).To(Equal(ptr.To(true)))
	})

	It("should set Reservation when VM-level scsiReservation overrides a disabled plan-level flag", func() {
		builder.Plan.Spec.RDMAsLun = true
		builder.Plan.Spec.SCSIReservation = false
		scsiReservation := true
		builder.Plan.Spec.VMs = []plan.VM{
			{
				Ref:             ref.Ref{ID: "vm-1"},
				SCSIReservation: &scsiReservation,
			},
		}
		vm := &model.VM{
			VM1: model.VM1{
				VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{
					{
						Key:      2000,
						File:     "[ds1] vm/disk.vmdk",
						Capacity: 1 << 30,
						RDM:      true,
						Shared:   true,
						Bus:      vsphere.SCSI,
					},
				},
			},
		}
		pvcs := []*core.PersistentVolumeClaim{buildPVC("[ds1] vm/disk.vmdk")}
		spec := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}

		err := builder.mapDisks(vm, ref.Ref{ID: "vm-1"}, pvcs, spec, false)
		Expect(err).NotTo(HaveOccurred())

		disk := spec.Template.Spec.Domain.Devices.Disks[0]
		Expect(disk.LUN).NotTo(BeNil())
		Expect(disk.LUN.Reservation).To(BeTrue())
		Expect(disk.Shareable).To(Equal(ptr.To(true)))
		Expect(disk.ErrorPolicy).To(Equal(ptr.To(cnv.DiskErrorPolicyReport)))
	})

	It("should NOT set Reservation when VM-level scsiReservation overrides an enabled plan-level flag", func() {
		builder.Plan.Spec.RDMAsLun = true
		builder.Plan.Spec.SCSIReservation = true
		scsiReservation := false
		builder.Plan.Spec.VMs = []plan.VM{
			{
				Ref:             ref.Ref{ID: "vm-1"},
				SCSIReservation: &scsiReservation,
			},
		}
		vm := &model.VM{
			VM1: model.VM1{
				VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{
					{
						Key:      2000,
						File:     "[ds1] vm/disk.vmdk",
						Capacity: 1 << 30,
						RDM:      true,
						Shared:   true,
						Bus:      vsphere.SCSI,
					},
				},
			},
		}
		pvcs := []*core.PersistentVolumeClaim{buildPVC("[ds1] vm/disk.vmdk")}
		spec := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}

		err := builder.mapDisks(vm, ref.Ref{ID: "vm-1"}, pvcs, spec, false)
		Expect(err).NotTo(HaveOccurred())

		disk := spec.Template.Spec.Domain.Devices.Disks[0]
		Expect(disk.LUN).NotTo(BeNil())
		Expect(disk.LUN.Reservation).To(BeFalse())
		Expect(disk.Shareable).To(Equal(ptr.To(true)))
	})
})

//nolint:errcheck
func createBuilder(objs ...runtime.Object) *Builder {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = core.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	_ = k8snet.AddToScheme(scheme)
	v1beta1.SchemeBuilder.AddToScheme(scheme)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
	return &Builder{
		Context: &plancontext.Context{
			Destination: plancontext.Destination{
				Client: client,
			},
			Source: plancontext.Source{
				Provider: &v1beta1.Provider{
					ObjectMeta: meta.ObjectMeta{Name: "test-vsphere-provider", Namespace: "test"},
					Spec: v1beta1.ProviderSpec{
						Type: (*v1beta1.ProviderType)(ptr.To("vsphere")),
						URL:  "https://vcenter.test.example.com/sdk",
					},
				},
				Inventory: nil,
				Secret: &core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "test-provider-secret", Namespace: "test"},
				},
			},
			Plan:      createPlan(),
			Migration: &v1beta1.Migration{ObjectMeta: meta.ObjectMeta{UID: k8stypes.UID("123")}},
			Log:       builderLog,
			Client:    client,
		},
	}
}

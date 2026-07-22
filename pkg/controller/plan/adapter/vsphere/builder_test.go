package vsphere

import (
	"context"
	"fmt"

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
		Expect(builder.mapMacStaticIps(vm, nil)).Should(Equal(outputMap))
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

	Context("mapMacStaticIps with networkIPMode filtering", func() {
		It("should skip NICs with mode 'none'", func() {
			b := createBuilder()
			vm := &model.VM{
				GuestID: "windows9Guest",
				GuestNetworks: []vsphere.GuestNetwork{
					{MAC: "00:50:56:83:25:47", IP: "172.29.3.193", Origin: ManualOrigin, PrefixLength: 16},
					{MAC: "00:50:56:83:25:48", IP: "172.29.3.194", Origin: ManualOrigin, PrefixLength: 16},
				},
				GuestIpStacks: []vsphere.GuestIpStack{{Gateway: "172.29.3.1", Network: "0.0.0.0"}},
			}
			modeByMAC := map[string]string{
				"00:50:56:83:25:47": "preserve",
				"00:50:56:83:25:48": "none",
			}
			result, err := b.mapMacStaticIps(vm, modeByMAC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("00:50:56:83:25:47"))
			Expect(result).NotTo(ContainSubstring("00:50:56:83:25:48"))
		})

		It("should skip NICs with mode 'dhcp'", func() {
			b := createBuilder()
			vm := &model.VM{
				GuestID: "windows9Guest",
				GuestNetworks: []vsphere.GuestNetwork{
					{MAC: "00:50:56:83:25:47", IP: "172.29.3.193", Origin: ManualOrigin, PrefixLength: 16},
				},
			}
			modeByMAC := map[string]string{
				"00:50:56:83:25:47": "dhcp",
			}
			result, err := b.mapMacStaticIps(vm, modeByMAC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("should include NICs not in modeByMAC (backward compat)", func() {
			b := createBuilder()
			vm := &model.VM{
				GuestID: "windows9Guest",
				GuestNetworks: []vsphere.GuestNetwork{
					{MAC: "00:50:56:83:25:47", IP: "172.29.3.193", Origin: ManualOrigin, PrefixLength: 16},
				},
				GuestIpStacks: []vsphere.GuestIpStack{{Gateway: "172.29.3.1", Network: "0.0.0.0"}},
			}
			// NIC not in the map at all — should proceed
			modeByMAC := map[string]string{}
			result, err := b.mapMacStaticIps(vm, modeByMAC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("00:50:56:83:25:47"))
		})

		It("should include all NICs when modeByMAC is nil", func() {
			b := createBuilder()
			vm := &model.VM{
				GuestID: "windows9Guest",
				GuestNetworks: []vsphere.GuestNetwork{
					{MAC: "00:50:56:83:25:47", IP: "172.29.3.193", Origin: ManualOrigin, PrefixLength: 16},
					{MAC: "00:50:56:83:25:48", IP: "172.29.3.194", Origin: ManualOrigin, PrefixLength: 16},
				},
				GuestIpStacks: []vsphere.GuestIpStack{{Gateway: "172.29.3.1", Network: "0.0.0.0"}},
			}
			result, err := b.mapMacStaticIps(vm, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("00:50:56:83:25:47"))
			Expect(result).To(ContainSubstring("00:50:56:83:25:48"))
		})

		It("should preserve only marked NICs in a mixed-mode map", func() {
			b := createBuilder()
			vm := &model.VM{
				GuestID: "windows9Guest",
				GuestNetworks: []vsphere.GuestNetwork{
					{MAC: "00:50:56:83:25:47", IP: "172.29.3.193", Origin: ManualOrigin, PrefixLength: 16},
					{MAC: "00:50:56:83:25:48", IP: "172.29.3.194", Origin: ManualOrigin, PrefixLength: 16},
					{MAC: "00:50:56:83:25:49", IP: "172.29.3.195", Origin: ManualOrigin, PrefixLength: 16},
				},
				GuestIpStacks: []vsphere.GuestIpStack{{Gateway: "172.29.3.1", Network: "0.0.0.0"}},
			}
			modeByMAC := map[string]string{
				"00:50:56:83:25:47": "preserve",
				"00:50:56:83:25:48": "dhcp",
				"00:50:56:83:25:49": "none",
			}
			result, err := b.mapMacStaticIps(vm, modeByMAC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("00:50:56:83:25:47"))
			Expect(result).NotTo(ContainSubstring("00:50:56:83:25:48"))
			Expect(result).NotTo(ContainSubstring("00:50:56:83:25:49"))
		})
	})

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

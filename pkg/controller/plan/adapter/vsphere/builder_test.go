package vsphere

import (
	"context"
	"fmt"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/storage/resolver"
	"github.com/kubev2v/forklift/pkg/storage/resolver/hpe"
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
			builder.Map.Storage = &storageMap

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
			builder.Map.Storage = &storageMap

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
					planbase.AnnDiskSource: diskFile,
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

var _ = Describe("buildCsiImportPVC", func() {
	var (
		csiStoragePair *v1beta1.StoragePair
		vvolDisk       vsphere.Disk
		testVM         *model.VM
	)

	BeforeEach(func() {
		csiStoragePair = &v1beta1.StoragePair{
			Source: ref.Ref{ID: "ds-1"},
			Destination: v1beta1.DestinationStorage{
				StorageClass: "hpe-sc",
			},
			OffloadPlugin: &v1beta1.OffloadPlugin{
				CsiVolumeImport: &v1beta1.CsiVolumeImport{
					SecretRef:            "hpe-creds",
					StorageVendorProduct: v1beta1.StorageVendorProductPrimera3Par,
				},
				VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
					SecretRef:            "hpe-creds",
					StorageVendorProduct: v1beta1.StorageVendorProductPrimera3Par,
				},
			},
		}
		vvolDisk = vsphere.Disk{
			Datastore: vsphere.Ref{ID: "ds-1"},
			File:      "[datastore1] vm/vm.vmdk",
			Bus:       vsphere.SCSI,
			Capacity:  1024 * 1024 * 1024,
			Key:       2000,
		}
		testVM = &model.VM{
			VM1: model.VM1{
				VM0:   model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{vvolDisk},
			},
		}
	})

	It("should return nil for warm migration", func() {
		builder := createBuilder()
		builder.Plan.Spec.Warm = true

		pvc, err := builder.buildCsiImportPVC(
			context.TODO(), ref.Ref{ID: "vm-1"}, testVM, vvolDisk, 0, csiStoragePair, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(pvc).To(BeNil())
	})

	It("should return nil when VVol volume not found on destination array (cross-array)", func() {
		builder := createBuilder()
		builder.diskBackingResolver = &mockDiskBackingResolver{
			backing: &resolver.DiskBacking{VVolID: "vvol:cross-array-uuid"},
		}
		builder.csiPlugin = resolver.CsiImportPluginFunc(func(_ *resolver.DiskBacking) (map[string]string, bool, error) {
			return nil, false, nil
		})

		pvc, err := builder.buildCsiImportPVC(
			context.TODO(), ref.Ref{ID: "vm-1"}, testVM, vvolDisk, 0, csiStoragePair, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(pvc).To(BeNil())
	})

	It("should return PVC with vendor annotations when volume found on same array", func() {
		builder := createBuilder()
		builder.diskBackingResolver = &mockDiskBackingResolver{
			backing: &resolver.DiskBacking{VVolID: "vvol:real-uuid"},
		}
		builder.csiPlugin = resolver.CsiImportPluginFunc(func(_ *resolver.DiskBacking) (map[string]string, bool, error) {
			return map[string]string{hpe.AnnotationKey: "my-hpe-volume"}, true, nil
		})

		pvc, err := builder.buildCsiImportPVC(
			context.TODO(), ref.Ref{ID: "vm-1"}, testVM, vvolDisk, 0, csiStoragePair, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(pvc).NotTo(BeNil())
		Expect(pvc.Annotations[hpe.AnnotationKey]).To(Equal("my-hpe-volume"))
		Expect(pvc.Annotations[planbase.AnnCopyMethod]).To(Equal(planbase.CopyMethodCsiImport))
		Expect(pvc.Annotations[planbase.AnnDiskSource]).NotTo(BeEmpty())
		Expect(*pvc.Spec.StorageClassName).To(Equal("hpe-sc"))
		Expect(*pvc.Spec.VolumeMode).To(Equal(core.PersistentVolumeBlock))
		Expect(pvc.Spec.AccessModes).To(ContainElement(core.ReadWriteMany))
	})

	It("should fail migration when storage API error occurs (not fall through to xcopy)", func() {
		builder := createBuilder()
		builder.diskBackingResolver = &mockDiskBackingResolver{
			backing: &resolver.DiskBacking{VVolID: "vvol:some-uuid"},
		}
		builder.csiPlugin = resolver.CsiImportPluginFunc(func(_ *resolver.DiskBacking) (map[string]string, bool, error) {
			return nil, false, fmt.Errorf("WSAPI connection refused")
		})

		pvc, err := builder.buildCsiImportPVC(
			context.TODO(), ref.Ref{ID: "vm-1"}, testVM, vvolDisk, 0, csiStoragePair, nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("WSAPI connection refused"))
		Expect(pvc).To(BeNil())
	})

	It("should error when storage secret is missing required credential keys", func() {
		builder := createBuilder(
			&core.Secret{
				ObjectMeta: meta.ObjectMeta{Name: "hpe-creds", Namespace: "test"},
				Data: map[string][]byte{
					"STORAGE_HOSTNAME": []byte("https://hpe:8080"),
				},
			},
		)
		builder.diskBackingResolver = &mockDiskBackingResolver{
			backing: &resolver.DiskBacking{VVolID: "vvol:some-uuid"},
		}

		pvc, err := builder.buildCsiImportPVC(
			context.TODO(), ref.Ref{ID: "vm-1"}, testVM, vvolDisk, 0, csiStoragePair, nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("missing required keys"))
		Expect(pvc).To(BeNil())
	})

	It("should return nil when RDM volume not found on destination array (cross-array)", func() {
		builder := createBuilder()
		builder.diskBackingResolver = &mockDiskBackingResolver{
			backing: &resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60002ac0000000000000182d00021f6b"},
		}
		builder.csiPlugin = resolver.CsiImportPluginFunc(func(_ *resolver.DiskBacking) (map[string]string, bool, error) {
			return nil, false, nil
		})

		pvc, err := builder.buildCsiImportPVC(
			context.TODO(), ref.Ref{ID: "vm-1"}, testVM, vvolDisk, 0, csiStoragePair, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(pvc).To(BeNil())
	})

	It("should error when volume not found and no xcopy fallback configured", func() {
		noXcopyPair := &v1beta1.StoragePair{
			Source: ref.Ref{ID: "ds-1"},
			Destination: v1beta1.DestinationStorage{
				StorageClass: "hpe-sc",
			},
			OffloadPlugin: &v1beta1.OffloadPlugin{
				CsiVolumeImport: &v1beta1.CsiVolumeImport{
					SecretRef:            "hpe-creds",
					StorageVendorProduct: v1beta1.StorageVendorProductPrimera3Par,
				},
				// no VSphereXcopyPluginConfig — xcopy fallback not available
			},
		}
		builder := createBuilder()
		builder.diskBackingResolver = &mockDiskBackingResolver{
			backing: &resolver.DiskBacking{VVolID: "vvol:cross-array-uuid"},
		}
		builder.csiPlugin = resolver.CsiImportPluginFunc(func(_ *resolver.DiskBacking) (map[string]string, bool, error) {
			return nil, false, nil
		})

		pvc, err := builder.buildCsiImportPVC(
			context.TODO(), ref.Ref{ID: "vm-1"}, testVM, vvolDisk, 0, noXcopyPair, nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no xcopy fallback configured"))
		Expect(pvc).To(BeNil())
	})

	It("should return nil for VMDK disk (CSI import only supports VVol and RDM)", func() {
		builder := createBuilder()
		builder.diskBackingResolver = &mockDiskBackingResolver{
			backing: &resolver.DiskBacking{DeviceName: "[datastore1] vm/vm.vmdk"},
		}

		pvc, err := builder.buildCsiImportPVC(
			context.TODO(), ref.Ref{ID: "vm-1"}, testVM, vvolDisk, 0, csiStoragePair, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(pvc).To(BeNil())
	})
})

var _ = Describe("CsiImportPVCs and PopulatorVolumes integration", func() {
	It("all VVol disks on different array: CsiImportPVCs empty, PopulatorVolumes creates xcopy PVCs", func() {
		builder := createBuilder(
			&core.Secret{
				ObjectMeta: meta.ObjectMeta{Name: "xcopy-secret", Namespace: "test"},
				Data:       map[string][]byte{"foo": []byte("bar")},
			},
		)
		vm := model.VM{
			VM1: model.VM1{
				VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{
					{Datastore: vsphere.Ref{ID: "ds-1"}, File: "[ds] vm/disk1.vmdk", Bus: vsphere.SCSI, Capacity: 1 << 30, Key: 2000},
					{Datastore: vsphere.Ref{ID: "ds-1"}, File: "[ds] vm/disk2.vmdk", Bus: vsphere.SCSI, Capacity: 1 << 30, Key: 2001},
				},
			},
		}
		storageMap := v1beta1.StorageMap{
			Spec: v1beta1.StorageMapSpec{
				Map: []v1beta1.StoragePair{
					{
						Source:      ref.Ref{ID: "ds-1"},
						Destination: v1beta1.DestinationStorage{StorageClass: "hpe-sc"},
						OffloadPlugin: &v1beta1.OffloadPlugin{
							CsiVolumeImport: &v1beta1.CsiVolumeImport{
								SecretRef:            "hpe-creds",
								StorageVendorProduct: v1beta1.StorageVendorProductPrimera3Par,
							},
							VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
								StorageVendorProduct: "primera3par",
								SecretRef:            "xcopy-secret",
							},
						},
					},
				},
			},
		}
		builder.Source.Inventory = &mockInventory{
			ds: model.Datastore{Resource: model.Resource{ID: "ds-1"}},
			vm: vm,
		}
		builder.Map.Storage = &storageMap
		builder.diskBackingResolver = &mockDiskBackingResolver{
			backing: &resolver.DiskBacking{VVolID: "vvol:cross-array"},
		}
		builder.csiPlugin = resolver.CsiImportPluginFunc(func(_ *resolver.DiskBacking) (map[string]string, bool, error) {
			return nil, false, nil
		})

		csiPVCs, err := builder.CsiImportPVCs(ref.Ref{ID: "vm-1"}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(csiPVCs).To(BeEmpty())

		xcopyPVCs, err := builder.PopulatorVolumes(ref.Ref{ID: "vm-1"}, nil, "xcopy-secret")
		Expect(err).NotTo(HaveOccurred())
		Expect(xcopyPVCs).To(HaveLen(2))
	})

	It("disk 1 on same array (CSI PVC), disk 2 on different array (xcopy PVC)", func() {
		builder := createBuilder(
			&core.Secret{
				ObjectMeta: meta.ObjectMeta{Name: "xcopy-secret", Namespace: "test"},
				Data:       map[string][]byte{"foo": []byte("bar")},
			},
		)
		disk1File := "[ds] vm/disk1.vmdk"
		disk2File := "[ds] vm/disk2.vmdk"
		vm := model.VM{
			VM1: model.VM1{
				VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{
					{Datastore: vsphere.Ref{ID: "ds-1"}, File: disk1File, Bus: vsphere.SCSI, Capacity: 1 << 30, Key: 2000},
					{Datastore: vsphere.Ref{ID: "ds-1"}, File: disk2File, Bus: vsphere.SCSI, Capacity: 1 << 30, Key: 2001},
				},
			},
		}
		storageMap := v1beta1.StorageMap{
			Spec: v1beta1.StorageMapSpec{
				Map: []v1beta1.StoragePair{
					{
						Source:      ref.Ref{ID: "ds-1"},
						Destination: v1beta1.DestinationStorage{StorageClass: "hpe-sc"},
						OffloadPlugin: &v1beta1.OffloadPlugin{
							CsiVolumeImport: &v1beta1.CsiVolumeImport{
								SecretRef:            "hpe-creds",
								StorageVendorProduct: v1beta1.StorageVendorProductPrimera3Par,
							},
							VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
								StorageVendorProduct: "primera3par",
								SecretRef:            "xcopy-secret",
							},
						},
					},
				},
			},
		}
		builder.Source.Inventory = &mockInventory{
			ds: model.Datastore{Resource: model.Resource{ID: "ds-1"}},
			vm: vm,
		}
		builder.Map.Storage = &storageMap

		// vSphere returns different VVolIDs per disk
		builder.diskBackingResolver = &mockDiskBackingResolver{
			backings: map[string]*resolver.DiskBacking{
				disk1File: {VVolID: "vvol:uuid-disk-1"},
				disk2File: {VVolID: "vvol:uuid-disk-2"},
			},
		}

		// Volumes that exist on the destination HPE array — disk 2 is on a different array
		arrayVolumes := map[string]string{
			"vvol:uuid-disk-1": "hpe-vol-disk-1",
		}
		builder.csiPlugin = resolver.CsiImportPluginFunc(func(backing *resolver.DiskBacking) (map[string]string, bool, error) {
			name, found := arrayVolumes[backing.VVolID]
			if !found {
				return nil, false, nil
			}
			return map[string]string{hpe.AnnotationKey: name}, true, nil
		})

		// Phase 1: CsiImportPVCs — only disk 1 resolves
		csiPVCs, err := builder.CsiImportPVCs(ref.Ref{ID: "vm-1"}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(csiPVCs).To(HaveLen(1))
		Expect(csiPVCs[0].Annotations[hpe.AnnotationKey]).To(Equal("hpe-vol-disk-1"))

		// Persist the CSI PVC so diskHandledByCsiImport finds it
		err = builder.Destination.Create(context.TODO(), &csiPVCs[0])
		Expect(err).NotTo(HaveOccurred())

		// Phase 2: PopulatorVolumes — picks up only disk 2
		xcopyPVCs, err := builder.PopulatorVolumes(ref.Ref{ID: "vm-1"}, nil, "xcopy-secret")
		Expect(err).NotTo(HaveOccurred())
		Expect(xcopyPVCs).To(HaveLen(1))
		Expect(xcopyPVCs[0].Annotations[planbase.AnnDiskSource]).To(Equal(disk2File))
	})

	It("should CSI-import the RDM and xcopy the VMDK when disk types are mixed", func() {
		builder := createBuilder(
			&core.Secret{
				ObjectMeta: meta.ObjectMeta{Name: "xcopy-secret", Namespace: "test"},
				Data:       map[string][]byte{"foo": []byte("bar")},
			},
		)
		vmdkFile := "[ds] vm/os.vmdk"
		rdmFile := "[ds] vm/data.vmdk"
		vm := model.VM{
			VM1: model.VM1{
				VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{
					{Datastore: vsphere.Ref{ID: "ds-1"}, File: vmdkFile, Bus: vsphere.SCSI, Capacity: 1 << 30, Key: 2000},
					{Datastore: vsphere.Ref{ID: "ds-1"}, File: rdmFile, Bus: vsphere.SCSI, Capacity: 1 << 30, Key: 2001, RDM: true, DeviceName: "naa.60002ac0000000000000182d00021f6b"},
				},
			},
		}
		storageMap := v1beta1.StorageMap{
			Spec: v1beta1.StorageMapSpec{
				Map: []v1beta1.StoragePair{
					{
						Source:      ref.Ref{ID: "ds-1"},
						Destination: v1beta1.DestinationStorage{StorageClass: "hpe-sc"},
						OffloadPlugin: &v1beta1.OffloadPlugin{
							CsiVolumeImport: &v1beta1.CsiVolumeImport{
								SecretRef:            "hpe-creds",
								StorageVendorProduct: v1beta1.StorageVendorProductPrimera3Par,
							},
							VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
								StorageVendorProduct: "primera3par",
								SecretRef:            "xcopy-secret",
							},
						},
					},
				},
			},
		}
		builder.Source.Inventory = &mockInventory{
			ds: model.Datastore{Resource: model.Resource{ID: "ds-1"}},
			vm: vm,
		}
		builder.Map.Storage = &storageMap

		// vSphere returns VMDK backing for disk 1, RDM backing for disk 2
		builder.diskBackingResolver = &mockDiskBackingResolver{
			backings: map[string]*resolver.DiskBacking{
				vmdkFile: {DeviceName: vmdkFile},
				rdmFile:  {IsRDM: true, DeviceName: "naa.60002ac0000000000000182d00021f6b"},
			},
		}

		// RDM volume exists on the destination array
		arrayVolumes := map[string]string{
			"naa.60002ac0000000000000182d00021f6b": "hpe-data-vol",
		}
		builder.csiPlugin = resolver.CsiImportPluginFunc(func(backing *resolver.DiskBacking) (map[string]string, bool, error) {
			name, found := arrayVolumes[backing.DeviceName]
			if !found {
				return nil, false, nil
			}
			return map[string]string{hpe.AnnotationKey: name}, true, nil
		})

		// Phase 1: CsiImportPVCs — VMDK falls through, RDM resolves
		csiPVCs, err := builder.CsiImportPVCs(ref.Ref{ID: "vm-1"}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(csiPVCs).To(HaveLen(1))
		Expect(csiPVCs[0].Annotations[hpe.AnnotationKey]).To(Equal("hpe-data-vol"))
		Expect(csiPVCs[0].Annotations[planbase.AnnDiskSource]).To(Equal(rdmFile))

		// Persist CSI PVC
		err = builder.Destination.Create(context.TODO(), &csiPVCs[0])
		Expect(err).NotTo(HaveOccurred())

		// Phase 2: PopulatorVolumes — picks up only the VMDK
		xcopyPVCs, err := builder.PopulatorVolumes(ref.Ref{ID: "vm-1"}, nil, "xcopy-secret")
		Expect(err).NotTo(HaveOccurred())
		Expect(xcopyPVCs).To(HaveLen(1))
		Expect(xcopyPVCs[0].Annotations[planbase.AnnDiskSource]).To(Equal(vmdkFile))
	})

	It("should xcopy both disks when VMDK is unsupported and RDM is cross-array", func() {
		builder := createBuilder(
			&core.Secret{
				ObjectMeta: meta.ObjectMeta{Name: "xcopy-secret", Namespace: "test"},
				Data:       map[string][]byte{"foo": []byte("bar")},
			},
		)
		vmdkFile := "[ds] vm/os.vmdk"
		rdmFile := "[ds] vm/data.vmdk"
		vm := model.VM{
			VM1: model.VM1{
				VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{
					{Datastore: vsphere.Ref{ID: "ds-1"}, File: vmdkFile, Bus: vsphere.SCSI, Capacity: 1 << 30, Key: 2000},
					{Datastore: vsphere.Ref{ID: "ds-1"}, File: rdmFile, Bus: vsphere.SCSI, Capacity: 1 << 30, Key: 2001, RDM: true, DeviceName: "naa.60002ac0000000000000182d00021f6b"},
				},
			},
		}
		storageMap := v1beta1.StorageMap{
			Spec: v1beta1.StorageMapSpec{
				Map: []v1beta1.StoragePair{
					{
						Source:      ref.Ref{ID: "ds-1"},
						Destination: v1beta1.DestinationStorage{StorageClass: "hpe-sc"},
						OffloadPlugin: &v1beta1.OffloadPlugin{
							CsiVolumeImport: &v1beta1.CsiVolumeImport{
								SecretRef:            "hpe-creds",
								StorageVendorProduct: v1beta1.StorageVendorProductPrimera3Par,
							},
							VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
								StorageVendorProduct: "primera3par",
								SecretRef:            "xcopy-secret",
							},
						},
					},
				},
			},
		}
		builder.Source.Inventory = &mockInventory{
			ds: model.Datastore{Resource: model.Resource{ID: "ds-1"}},
			vm: vm,
		}
		builder.Map.Storage = &storageMap
		builder.diskBackingResolver = &mockDiskBackingResolver{
			backings: map[string]*resolver.DiskBacking{
				vmdkFile: {DeviceName: vmdkFile},
				rdmFile:  {IsRDM: true, DeviceName: "naa.60002ac0000000000000182d00021f6b"},
			},
		}

		// Empty array — RDM volume is on a different array
		builder.csiPlugin = resolver.CsiImportPluginFunc(func(backing *resolver.DiskBacking) (map[string]string, bool, error) {
			return nil, false, nil
		})

		// CsiImportPVCs: VMDK skipped (disk type), RDM not found (cross-array) → empty
		csiPVCs, err := builder.CsiImportPVCs(ref.Ref{ID: "vm-1"}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(csiPVCs).To(BeEmpty())

		// PopulatorVolumes: picks up both
		xcopyPVCs, err := builder.PopulatorVolumes(ref.Ref{ID: "vm-1"}, nil, "xcopy-secret")
		Expect(err).NotTo(HaveOccurred())
		Expect(xcopyPVCs).To(HaveLen(2))
	})

	It("should send all disks to xcopy when no CsiVolumeImport is configured", func() {
		builder := createBuilder(
			&core.Secret{
				ObjectMeta: meta.ObjectMeta{Name: "xcopy-secret", Namespace: "test"},
				Data:       map[string][]byte{"foo": []byte("bar")},
			},
		)
		vm := model.VM{
			VM1: model.VM1{
				VM0: model.VM0{ID: "vm-1", Name: "test-vm"},
				Disks: []vsphere.Disk{
					{Datastore: vsphere.Ref{ID: "ds-1"}, File: "[ds] vm/os.vmdk", Bus: vsphere.SCSI, Capacity: 1 << 30, Key: 2000},
					{Datastore: vsphere.Ref{ID: "ds-1"}, File: "[ds] vm/data.vmdk", Bus: vsphere.SCSI, Capacity: 1 << 30, Key: 2001, RDM: true, DeviceName: "naa.60002ac0000000000000182d00021f6b"},
				},
			},
		}
		storageMap := v1beta1.StorageMap{
			Spec: v1beta1.StorageMapSpec{
				Map: []v1beta1.StoragePair{
					{
						Source:      ref.Ref{ID: "ds-1"},
						Destination: v1beta1.DestinationStorage{StorageClass: "sc"},
						OffloadPlugin: &v1beta1.OffloadPlugin{
							VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
								StorageVendorProduct: "primera3par",
								SecretRef:            "xcopy-secret",
							},
						},
					},
				},
			},
		}
		builder.Source.Inventory = &mockInventory{
			ds: model.Datastore{Resource: model.Resource{ID: "ds-1"}},
			vm: vm,
		}
		builder.Map.Storage = &storageMap

		csiPVCs, err := builder.CsiImportPVCs(ref.Ref{ID: "vm-1"}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(csiPVCs).To(BeEmpty())

		xcopyPVCs, err := builder.PopulatorVolumes(ref.Ref{ID: "vm-1"}, nil, "xcopy-secret")
		Expect(err).NotTo(HaveOccurred())
		Expect(xcopyPVCs).To(HaveLen(2))
	})
})

var _ = Describe("diskHandledByCsiImport", func() {
	It("should return true when PVC exists with csi-import copy method", func() {
		builder := createBuilder()
		pvcList := &core.PersistentVolumeClaimList{
			Items: []core.PersistentVolumeClaim{
				{
					ObjectMeta: meta.ObjectMeta{
						Annotations: map[string]string{
							planbase.AnnDiskSource: "vm/vm.vmdk",
							planbase.AnnCopyMethod: planbase.CopyMethodCsiImport,
						},
					},
				},
			},
		}

		Expect(builder.diskHandledByCsiImport(pvcList, "vm/vm.vmdk")).To(BeTrue())
	})

	It("should return false when PVC has different copy method", func() {
		builder := createBuilder()
		pvcList := &core.PersistentVolumeClaimList{
			Items: []core.PersistentVolumeClaim{
				{
					ObjectMeta: meta.ObjectMeta{
						Annotations: map[string]string{
							planbase.AnnDiskSource: "vm/vm.vmdk",
							planbase.AnnCopyMethod: "xcopy",
						},
					},
				},
			},
		}

		Expect(builder.diskHandledByCsiImport(pvcList, "vm/vm.vmdk")).To(BeFalse())
	})

	It("should return false when no matching PVC exists", func() {
		builder := createBuilder()
		pvcList := &core.PersistentVolumeClaimList{}

		Expect(builder.diskHandledByCsiImport(pvcList, "vm/vm.vmdk")).To(BeFalse())
	})
})

type mockDiskBackingResolver struct {
	backing  *resolver.DiskBacking
	backings map[string]*resolver.DiskBacking
	err      error
}

func (m *mockDiskBackingResolver) getDiskBacking(_ context.Context, _, diskFile string) (*resolver.DiskBacking, error) {
	if m.backings != nil {
		if b, ok := m.backings[diskFile]; ok {
			return b, m.err
		}
	}
	return m.backing, m.err
}

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

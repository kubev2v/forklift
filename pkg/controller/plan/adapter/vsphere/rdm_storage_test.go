package vsphere

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// dsInventory is a simple mock that satisfies datastoreFinder for tests.
type dsInventory struct {
	datastores map[string]model.Datastore
}

func (m *dsInventory) Find(resource interface{}, r ref.Ref) error {
	ds, ok := m.datastores[r.ID]
	if !ok {
		return fmt.Errorf("datastore %q not found", r.ID)
	}
	if out, ok := resource.(*model.Datastore); ok {
		*out = ds
	}
	return nil
}

var _ = Describe("RDM storage resolution", func() {
	Context("vendorFromNAA", func() {
		DescribeTable("should match known vendor prefixes",
			func(deviceName string, expectedVendor api.StorageVendorProduct) {
				vendor, ok := vendorFromNAA(deviceName, naaVendorPrefixes)
				Expect(ok).To(BeTrue())
				Expect(vendor).To(Equal(expectedVendor))
			},
			Entry("Pure FlashArray", "naa.624a93700123456789abcdef", api.StorageVendorProductPureFlashArray),
			Entry("PowerStore", "naa.68ccf0980065753dad7b5819dc1ae4c6", api.StorageVendorProductPowerStore),
			Entry("Vantara", "naa.60060e800123456789abcdef", api.StorageVendorProductVantara),
			Entry("ONTAP", "naa.600a09800123456789abcdef", api.StorageVendorProductOntap),
			Entry("FlashSystem", "naa.60050760123456789abcdef", api.StorageVendorProductFlashSystem),
			Entry("PowerMax (Symmetrix OUI)", "naa.60000970123456789abcdef", api.StorageVendorProductPowerMax),
			Entry("PowerMax (VMAX OUI)", "naa.60060480123456789abcdef", api.StorageVendorProductPowerMax),
			Entry("Primera3Par", "naa.60002ac0123456789abcdef", api.StorageVendorProductPrimera3Par),
			Entry("Infinibox", "naa.6742b0f0123456789abcdef", api.StorageVendorProductInfinibox),
		)

		It("should handle full device paths", func() {
			vendor, ok := vendorFromNAA("/vmfs/devices/disks/naa.624a93700123456789abcdef", naaVendorPrefixes)
			Expect(ok).To(BeTrue())
			Expect(vendor).To(Equal(api.StorageVendorProductPureFlashArray))
		})

		It("should be case insensitive", func() {
			vendor, ok := vendorFromNAA("naa.624A93700123456789ABCDEF", naaVendorPrefixes)
			Expect(ok).To(BeTrue())
			Expect(vendor).To(Equal(api.StorageVendorProductPureFlashArray))
		})

		It("should return false for unknown prefix", func() {
			_, ok := vendorFromNAA("naa.6999999999999999", naaVendorPrefixes)
			Expect(ok).To(BeFalse())
		})

		It("should return false for non-NAA format", func() {
			_, ok := vendorFromNAA("eui.0123456789abcdef", naaVendorPrefixes)
			Expect(ok).To(BeFalse())
		})

		It("should return false for empty string", func() {
			_, ok := vendorFromNAA("", naaVendorPrefixes)
			Expect(ok).To(BeFalse())
		})

		It("should handle vml prefix with naa", func() {
			vendor, ok := vendorFromNAA("vml.02000400006000097000022000285753303031313453594d4d4554naa.6000097000022000285753303031313454", naaVendorPrefixes)
			Expect(ok).To(BeTrue())
			Expect(vendor).To(Equal(api.StorageVendorProductPowerMax))
		})

		It("should handle vml prefix (ESXi format) for PowerStore", func() {
			vendor, ok := vendorFromNAA("vml.02006a000068ccf0980065753dad7b5819dc1ae4c6506f77657253", naaVendorPrefixes)
			Expect(ok).To(BeTrue())
			Expect(vendor).To(Equal(api.StorageVendorProductPowerStore))
		})

		It("should handle vml prefix for Pure FlashArray", func() {
			vendor, ok := vendorFromNAA("vml.0200010000624a93700abcdef012345678", naaVendorPrefixes)
			Expect(ok).To(BeTrue())
			Expect(vendor).To(Equal(api.StorageVendorProductPureFlashArray))
		})

		It("should return false for unknown vml prefix", func() {
			_, ok := vendorFromNAA("vml.0200010000699999900000000000000000", naaVendorPrefixes)
			Expect(ok).To(BeFalse())
		})

		It("should use custom prefix list", func() {
			custom := []naaVendorEntry{
				{prefix: "6aabbcc", vendor: api.StorageVendorProductPowerFlex},
			}
			vendor, ok := vendorFromNAA("naa.6aabbcc0123456789", custom)
			Expect(ok).To(BeTrue())
			Expect(vendor).To(Equal(api.StorageVendorProductPowerFlex))
		})
	})

	Context("findStorageMapEntriesForVendor", func() {
		pureEntry := api.StoragePair{
			Destination: api.DestinationStorage{StorageClass: "pure-sc"},
			OffloadPlugin: &api.OffloadPlugin{
				VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{
					StorageVendorProduct: api.StorageVendorProductPureFlashArray,
					SecretRef:            "pure-secret",
				},
			},
		}
		powerMaxEntry := api.StoragePair{
			Destination: api.DestinationStorage{StorageClass: "powermax-sc"},
			OffloadPlugin: &api.OffloadPlugin{
				VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{
					StorageVendorProduct: api.StorageVendorProductPowerMax,
					SecretRef:            "powermax-secret",
				},
			},
		}
		noOffloadEntry := api.StoragePair{
			Destination: api.DestinationStorage{StorageClass: "standard-sc"},
		}

		It("should find a single matching entry", func() {
			storageMap := []api.StoragePair{pureEntry, powerMaxEntry}
			result := findStorageMapEntriesForVendor(storageMap, api.StorageVendorProductPowerMax)
			Expect(result).To(HaveLen(1))
			Expect(result[0].OffloadPlugin.VSphereXcopyPluginConfig.SecretRef).To(Equal("powermax-secret"))
			Expect(result[0].Destination.StorageClass).To(Equal("powermax-sc"))
		})

		It("should return empty when no entry matches", func() {
			storageMap := []api.StoragePair{pureEntry}
			result := findStorageMapEntriesForVendor(storageMap, api.StorageVendorProductPowerMax)
			Expect(result).To(BeEmpty())
		})

		It("should return all matching entries when multiple match", func() {
			secondPure := pureEntry
			secondPure.Destination = api.DestinationStorage{StorageClass: "pure-sc-2"}
			storageMap := []api.StoragePair{pureEntry, secondPure, powerMaxEntry}
			result := findStorageMapEntriesForVendor(storageMap, api.StorageVendorProductPureFlashArray)
			Expect(result).To(HaveLen(2))
		})

		It("should skip entries without offload plugin", func() {
			storageMap := []api.StoragePair{noOffloadEntry, powerMaxEntry}
			result := findStorageMapEntriesForVendor(storageMap, api.StorageVendorProductPowerMax)
			Expect(result).To(HaveLen(1))
			Expect(result[0].Destination.StorageClass).To(Equal("powermax-sc"))
		})

		It("should skip entries with nil VSphereXcopyPluginConfig", func() {
			nilConfigEntry := api.StoragePair{
				OffloadPlugin: &api.OffloadPlugin{},
			}
			storageMap := []api.StoragePair{nilConfigEntry, pureEntry}
			result := findStorageMapEntriesForVendor(storageMap, api.StorageVendorProductPureFlashArray)
			Expect(result).To(HaveLen(1))
			Expect(result[0].Destination.StorageClass).To(Equal("pure-sc"))
		})
	})

	Context("extractNAAHex", func() {
		It("should extract hex from naa. prefix", func() {
			Expect(extractNAAHex("naa.624a93700123456789abcdef", naaVendorPrefixes)).To(Equal("624a93700123456789abcdef"))
		})

		It("should extract hex from full device path", func() {
			Expect(extractNAAHex("/vmfs/devices/disks/naa.624a93700123456789abcdef", naaVendorPrefixes)).To(Equal("624a93700123456789abcdef"))
		})

		It("should extract NAA-6 hex from vml. prefix, stripping preamble", func() {
			hex := extractNAAHex("vml.02006a000068ccf0980065753dad7b5819dc1ae4c6506f77657253", naaVendorPrefixes)
			Expect(hex).To(Equal("68ccf0980065753dad7b5819dc1ae4c6"))
		})

		It("should return empty for unknown format", func() {
			Expect(extractNAAHex("eui.0123456789abcdef", naaVendorPrefixes)).To(BeEmpty())
		})

		It("should return empty for empty string", func() {
			Expect(extractNAAHex("", naaVendorPrefixes)).To(BeEmpty())
		})

		It("should lowercase the result", func() {
			Expect(extractNAAHex("naa.624A93700123456789ABCDEF", naaVendorPrefixes)).To(Equal("624a93700123456789abcdef"))
		})

		It("should produce comparable output for NAA and VML formats of same LUN", func() {
			naaHex := extractNAAHex("naa.6000097000022222000000000000001", naaVendorPrefixes)
			vmlHex := extractNAAHex("vml.02000400006000097000022222000000000000001", naaVendorPrefixes)
			// Both should produce the same NAA-6 identifier
			Expect(naaHex).To(Equal("6000097000022222000000000000001"))
			Expect(vmlHex).To(Equal("6000097000022222000000000000001"))
		})
	})

	Context("commonPrefixLen", func() {
		It("should return 0 for no common prefix", func() {
			Expect(commonPrefixLen("abc", "xyz")).To(Equal(0))
		})

		It("should return full length for identical strings", func() {
			Expect(commonPrefixLen("abc", "abc")).To(Equal(3))
		})

		It("should return partial match length", func() {
			Expect(commonPrefixLen("abcdef", "abcxyz")).To(Equal(3))
		})

		It("should handle different lengths", func() {
			Expect(commonPrefixLen("ab", "abcdef")).To(Equal(2))
		})

		It("should handle empty strings", func() {
			Expect(commonPrefixLen("", "abc")).To(Equal(0))
		})
	})

	Context("disambiguateRDMByNAA", func() {
		// Two PowerMax arrays with different serial numbers.
		// NAAs share OUI (6000097) but differ in array serial.
		powerMaxEntry1 := api.StoragePair{
			Source:      ref.Ref{ID: "ds-pm1"},
			Destination: api.DestinationStorage{StorageClass: "powermax-1-sc"},
			OffloadPlugin: &api.OffloadPlugin{
				VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{
					StorageVendorProduct: api.StorageVendorProductPowerMax,
					SecretRef:            "pm1-secret",
				},
			},
		}
		powerMaxEntry2 := api.StoragePair{
			Source:      ref.Ref{ID: "ds-pm2"},
			Destination: api.DestinationStorage{StorageClass: "powermax-2-sc"},
			OffloadPlugin: &api.OffloadPlugin{
				VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{
					StorageVendorProduct: api.StorageVendorProductPowerMax,
					SecretRef:            "pm2-secret",
				},
			},
		}

		It("should pick the entry whose datastore shares the longest NAA prefix", func() {
			inv := &dsInventory{
				datastores: map[string]model.Datastore{
					"ds-pm1": {BackingDevicesNames: []string{"naa.6000097000011111000000000000001"}},
					"ds-pm2": {BackingDevicesNames: []string{"naa.6000097000022222000000000000001"}},
				},
			}
			// RDM is on array 2 (serial 00002222...)
			candidates := []*api.StoragePair{&powerMaxEntry1, &powerMaxEntry2}
			result, err := disambiguateRDMByNAA(inv, candidates, "naa.6000097000022222000000000000099", naaVendorPrefixes)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Destination.StorageClass).To(Equal("powermax-2-sc"))
		})

		It("should skip candidates whose datastore cannot be loaded", func() {
			inv := &dsInventory{
				datastores: map[string]model.Datastore{
					// ds-pm1 is missing — will fail Find()
					"ds-pm2": {BackingDevicesNames: []string{"naa.6000097000022222000000000000001"}},
				},
			}
			candidates := []*api.StoragePair{&powerMaxEntry1, &powerMaxEntry2}
			result, err := disambiguateRDMByNAA(inv, candidates, "naa.6000097000022222000000000000099", naaVendorPrefixes)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Destination.StorageClass).To(Equal("powermax-2-sc"))
		})

		It("should error when no candidate shares an array with the RDM", func() {
			// Use NAAs from different vendors/OUIs so common prefix is short
			inv := &dsInventory{
				datastores: map[string]model.Datastore{
					"ds-pm1": {BackingDevicesNames: []string{"naa.6000097aaa011111000000000000001"}},
					"ds-pm2": {BackingDevicesNames: []string{"naa.6000097bbb022222000000000000001"}},
				},
			}
			// RDM serial diverges right after OUI — common prefix is only ~7 chars
			candidates := []*api.StoragePair{&powerMaxEntry1, &powerMaxEntry2}
			_, err := disambiguateRDMByNAA(inv, candidates, "naa.6000097ccc099999000000000000001", naaVendorPrefixes)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("none share an array"))
		})

		It("should error when RDM device name has no extractable NAA", func() {
			inv := &dsInventory{datastores: map[string]model.Datastore{}}
			candidates := []*api.StoragePair{&powerMaxEntry1}
			_, err := disambiguateRDMByNAA(inv, candidates, "eui.badformat", naaVendorPrefixes)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot extract NAA hex"))
		})

		It("should work with VML-formatted RDM device name", func() {
			inv := &dsInventory{
				datastores: map[string]model.Datastore{
					"ds-pm1": {BackingDevicesNames: []string{"naa.6000097000011111000000000000001"}},
					"ds-pm2": {BackingDevicesNames: []string{"naa.6000097000022222000000000000001"}},
				},
			}
			// RDM in VML format, array 2 serial
			candidates := []*api.StoragePair{&powerMaxEntry1, &powerMaxEntry2}
			result, err := disambiguateRDMByNAA(inv, candidates, "vml.02000400006000097000022222000000000000099", naaVendorPrefixes)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Destination.StorageClass).To(Equal("powermax-2-sc"))
		})

		It("should work with VML-formatted backing device names", func() {
			inv := &dsInventory{
				datastores: map[string]model.Datastore{
					"ds-pm1": {BackingDevicesNames: []string{"vml.02000400006000097000011111000000000000001"}},
					"ds-pm2": {BackingDevicesNames: []string{"vml.02000400006000097000022222000000000000001"}},
				},
			}
			// RDM in NAA format, array 1 serial
			candidates := []*api.StoragePair{&powerMaxEntry1, &powerMaxEntry2}
			result, err := disambiguateRDMByNAA(inv, candidates, "naa.6000097000011111000000000000099", naaVendorPrefixes)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Destination.StorageClass).To(Equal("powermax-1-sc"))
		})

		It("should handle multiple backing devices per datastore", func() {
			inv := &dsInventory{
				datastores: map[string]model.Datastore{
					"ds-pm1": {BackingDevicesNames: []string{
						"naa.6000097000011111000000000000001",
						"naa.6000097000011111000000000000002",
					}},
					"ds-pm2": {BackingDevicesNames: []string{
						"naa.6000097000022222000000000000001",
					}},
				},
			}
			// RDM is on array 1
			candidates := []*api.StoragePair{&powerMaxEntry1, &powerMaxEntry2}
			result, err := disambiguateRDMByNAA(inv, candidates, "naa.6000097000011111000000000000099", naaVendorPrefixes)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Destination.StorageClass).To(Equal("powermax-1-sc"))
		})
	})
})

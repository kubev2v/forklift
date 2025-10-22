package vsphere

import (
	modelVsphere "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("isCBTEnabledForDisks", func() {
	var disks []modelVsphere.Disk
	var ctkMap map[string]bool

	BeforeEach(func() {
		disks = []modelVsphere.Disk{}
		ctkMap = map[string]bool{}
	})

	Context("All disks with CBT enabled across types", func() {
		BeforeEach(func() {
			disks = []modelVsphere.Disk{
				{ControllerKey: 16000, UnitNumber: 0, Bus: "scsi"}, // scsi0:0
				{ControllerKey: 17000, UnitNumber: 1, Bus: "sata"}, // sata0:1
				{ControllerKey: 18000, UnitNumber: 2, Bus: "nvme"}, // nvme0:2
				{ControllerKey: 19000, UnitNumber: 0, Bus: "ide"},  // ide0:0
			}
			ctkMap = map[string]bool{
				"scsi0:0": true,
				"sata0:1": true,
				"nvme0:2": true,
				"ide0:0":  true,
			}
		})

		It("should enable CBT for all disks", func() {
			isCBTEnabledForDisks(ctkMap, disks)
			for _, d := range disks {
				Expect(d.ChangeTrackingEnabled).To(BeTrue())
			}
		})
	})

	Context("Mixed CBT state and missing entries", func() {
		BeforeEach(func() {
			disks = []modelVsphere.Disk{
				{ControllerKey: 16000, UnitNumber: 0, Bus: "scsi"}, // scsi0:0 → true
				{ControllerKey: 17000, UnitNumber: 1, Bus: "sata"}, // sata0:1 → false
				{ControllerKey: 18001, UnitNumber: 0, Bus: "nvme"}, // nvme1:0 → true
				{ControllerKey: 19000, UnitNumber: 1, Bus: "ide"},  // ide0:1 → not in map
			}
			ctkMap = map[string]bool{
				"scsi0:0": true,
				"sata0:1": false,
				"nvme1:0": true,
				// ide0:1 missing
			}
		})

		It("should correctly reflect CBT state per device key", func() {
			isCBTEnabledForDisks(ctkMap, disks)
			Expect(disks[0].ChangeTrackingEnabled).To(BeTrue())  // scsi0:0
			Expect(disks[1].ChangeTrackingEnabled).To(BeFalse()) // sata0:1
			Expect(disks[2].ChangeTrackingEnabled).To(BeTrue())  // nvme1:0
			Expect(disks[3].ChangeTrackingEnabled).To(BeFalse()) // ide0:1 default false
		})
	})

	Context("No entries in the CBT map", func() {
		BeforeEach(func() {
			disks = []modelVsphere.Disk{
				{ControllerKey: 16000, UnitNumber: 1, Bus: "scsi"},
				{ControllerKey: 17000, UnitNumber: 2, Bus: "sata"},
			}
		})

		It("should default all CBT flags to false", func() {
			isCBTEnabledForDisks(ctkMap, disks)
			for _, d := range disks {
				Expect(d.ChangeTrackingEnabled).To(BeFalse())
			}
		})
	})

	Context("CBT enabled for some and missing others", func() {
		BeforeEach(func() {
			disks = []modelVsphere.Disk{
				{ControllerKey: 16000, UnitNumber: 0, Bus: "scsi"}, // scsi0:0
				{ControllerKey: 17000, UnitNumber: 1, Bus: "sata"}, // sata0:1
				{ControllerKey: 18000, UnitNumber: 2, Bus: "nvme"}, // nvme0:2 (missing)
			}
			ctkMap = map[string]bool{
				"scsi0:0": true,
				"sata0:1": false,
			}
		})

		It("should match enabled state and default missing to false", func() {
			isCBTEnabledForDisks(ctkMap, disks)
			Expect(disks[0].ChangeTrackingEnabled).To(BeTrue())  // scsi0:0
			Expect(disks[1].ChangeTrackingEnabled).To(BeFalse()) // sata0:1
			Expect(disks[2].ChangeTrackingEnabled).To(BeFalse()) // nvme0:2
		})
	})

	Context("Empty disks slice", func() {
		It("should not panic or modify anything", func() {
			Expect(func() {
				isCBTEnabledForDisks(ctkMap, disks)
			}).ToNot(Panic())
			Expect(disks).To(BeEmpty())
		})
	})
})

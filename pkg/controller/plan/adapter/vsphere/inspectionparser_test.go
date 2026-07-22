package vsphere

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetBootDiskFromInspectionXML", func() {
	DescribeTable("detects boot disk from inspection XML",
		func(xml string, expected int) {
			Expect(GetBootDiskFromInspectionXML(xml)).To(Equal(expected))
		},
		Entry("prefers /boot/efi on sda", `
<v2v-inspection>
  <operatingsystem>
    <root>btrfsvol:/dev/sda4/root</root>
    <mountpoints>
      <mountpoint dev='btrfsvol:/dev/sda4/root'>/</mountpoint>
      <mountpoint dev='/dev/sda3'>/boot</mountpoint>
      <mountpoint dev='/dev/sda2'>/boot/efi</mountpoint>
    </mountpoints>
  </operatingsystem>
</v2v-inspection>`, 0),

		Entry("prefers /boot/efi on second disk", `
<v2v-inspection>
  <operatingsystem>
    <root>/dev/sda2</root>
    <mountpoints>
      <mountpoint dev='/dev/sda2'>/</mountpoint>
      <mountpoint dev='/dev/sdb1'>/boot/efi</mountpoint>
    </mountpoints>
  </operatingsystem>
</v2v-inspection>`, 1),

		Entry("falls back to /boot when no /boot/efi", `
<v2v-inspection>
  <operatingsystem>
    <root>/dev/sda2</root>
    <mountpoints>
      <mountpoint dev='/dev/sda2'>/</mountpoint>
      <mountpoint dev='/dev/sdb1'>/boot</mountpoint>
    </mountpoints>
  </operatingsystem>
</v2v-inspection>`, 1),

		Entry("falls back to root device when no /boot mounts", `
<v2v-inspection>
  <operatingsystem>
    <root>/dev/sdb2</root>
    <mountpoints>
      <mountpoint dev='/dev/sdb2'>/</mountpoint>
    </mountpoints>
  </operatingsystem>
</v2v-inspection>`, 1),

		Entry("handles btrfs root device", `
<v2v-inspection>
  <operatingsystem>
    <root>btrfsvol:/dev/sdc1/root</root>
    <mountpoints>
      <mountpoint dev='btrfsvol:/dev/sdc1/root'>/</mountpoint>
    </mountpoints>
  </operatingsystem>
</v2v-inspection>`, 2),

		Entry("handles single-disk VM (sda)", `
<v2v-inspection>
  <operatingsystem>
    <root>/dev/sda1</root>
    <mountpoints>
      <mountpoint dev='/dev/sda1'>/</mountpoint>
      <mountpoint dev='/dev/sda2'>/boot</mountpoint>
    </mountpoints>
  </operatingsystem>
</v2v-inspection>`, 0),

		Entry("returns -1 for empty XML", `
<v2v-inspection>
  <operatingsystem>
  </operatingsystem>
</v2v-inspection>`, -1),

		Entry("returns -1 for invalid XML", `not valid xml`, -1),

		Entry("returns -1 for unsupported device naming", `
<v2v-inspection>
  <operatingsystem>
    <root>/dev/nvme0n1p2</root>
    <mountpoints>
      <mountpoint dev='/dev/nvme0n1p2'>/</mountpoint>
    </mountpoints>
  </operatingsystem>
</v2v-inspection>`, -1),

		Entry("handles virtio device vda", `
<v2v-inspection>
  <operatingsystem>
    <root>/dev/vda2</root>
    <mountpoints>
      <mountpoint dev='/dev/vda2'>/</mountpoint>
      <mountpoint dev='/dev/vda1'>/boot</mountpoint>
    </mountpoints>
  </operatingsystem>
</v2v-inspection>`, 0),
	)
})

var _ = Describe("deviceToDiskIndex", func() {
	DescribeTable("maps device paths to disk index",
		func(dev string, expected int) {
			Expect(deviceToDiskIndex(dev)).To(Equal(expected))
		},
		Entry("/dev/sda", "/dev/sda", 0),
		Entry("/dev/sda1", "/dev/sda1", 0),
		Entry("/dev/sdb", "/dev/sdb", 1),
		Entry("/dev/sdb3", "/dev/sdb3", 1),
		Entry("/dev/sdc1", "/dev/sdc1", 2),
		Entry("/dev/sdz", "/dev/sdz", 25),
		Entry("/dev/vda1", "/dev/vda1", 0),
		Entry("/dev/vdb2", "/dev/vdb2", 1),
		Entry("/dev/hda", "/dev/hda", 0),
		Entry("btrfsvol:/dev/sda4/root", "btrfsvol:/dev/sda4/root", 0),
		Entry("btrfsvol:/dev/sdb1/home", "btrfsvol:/dev/sdb1/home", 1),
		Entry("/dev/nvme0n0p1 unsupported", "/dev/nvme0n0p1", -1),
		Entry("unknown device", "/dev/xda1", -1),
		Entry("empty string", "", -1),
	)
})

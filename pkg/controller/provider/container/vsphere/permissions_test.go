package vsphere

import (
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("vSphere permissions", func() {
	Describe("pruneToAvailablePrivileges", func() {
		input := []privilegeGroup{
			{
				Description: "Group A",
				Privileges:  []string{"Priv.A1", "Priv.A2"},
			},
			{
				Description: "Group B",
				Privileges:  []string{"Priv.B1", "Priv.B2", "Priv.B3"},
			},
		}

		It("should keep all privileges when all are available", func() {
			available := map[string]bool{
				"Priv.A1": true, "Priv.A2": true,
				"Priv.B1": true, "Priv.B2": true, "Priv.B3": true,
			}
			result := pruneToAvailablePrivileges(input, available)
			Expect(result).To(HaveLen(2))
			Expect(result[0].Privileges).To(Equal([]string{"Priv.A1", "Priv.A2"}))
			Expect(result[1].Privileges).To(Equal([]string{"Priv.B1", "Priv.B2", "Priv.B3"}))
		})

		It("should remove unavailable privileges from a group", func() {
			available := map[string]bool{
				"Priv.A1": true, "Priv.A2": true,
				"Priv.B1": true,
			}
			result := pruneToAvailablePrivileges(input, available)
			Expect(result).To(HaveLen(2))
			Expect(result[0].Privileges).To(Equal([]string{"Priv.A1", "Priv.A2"}))
			Expect(result[1].Privileges).To(Equal([]string{"Priv.B1"}))
		})

		It("should drop a group entirely when none of its privileges are available", func() {
			available := map[string]bool{
				"Priv.A1": true, "Priv.A2": true,
			}
			result := pruneToAvailablePrivileges(input, available)
			Expect(result).To(HaveLen(1))
			Expect(result[0].Description).To(Equal("Group A"))
		})

		It("should return nil for empty input", func() {
			result := pruneToAvailablePrivileges(nil, map[string]bool{"Priv.X": true})
			Expect(result).To(BeNil())
		})

		It("should return nil when no privileges are available", func() {
			result := pruneToAvailablePrivileges(input, map[string]bool{})
			Expect(result).To(BeNil())
		})
	})

	Describe("comparePrivileges", func() {
		groups := []privilegeGroup{
			{
				Description: "Snapshots",
				Privileges:  []string{"VM.Snap.Create", "VM.Snap.Remove"},
			},
			{
				Description: "Power",
				Privileges:  []string{"VM.PowerOn", "VM.PowerOff"},
			},
		}

		It("should return nil when all privileges are granted", func() {
			granted := map[string]bool{
				"VM.Snap.Create": true, "VM.Snap.Remove": true,
				"VM.PowerOn": true, "VM.PowerOff": true,
			}
			result := comparePrivileges(groups, granted)
			Expect(result).To(BeNil())
		})

		It("should report missing privileges from one group", func() {
			granted := map[string]bool{
				"VM.Snap.Create": true, "VM.Snap.Remove": true,
				"VM.PowerOn": true, "VM.PowerOff": false,
			}
			result := comparePrivileges(groups, granted)
			Expect(result).To(HaveLen(1))
			Expect(result[0].Group).To(Equal("Power"))
			Expect(result[0].Privileges).To(Equal([]string{"VM.PowerOff"}))
		})

		It("should report missing privileges across multiple groups", func() {
			granted := map[string]bool{
				"VM.Snap.Create": true, "VM.Snap.Remove": false,
				"VM.PowerOn": false, "VM.PowerOff": true,
			}
			result := comparePrivileges(groups, granted)
			Expect(result).To(HaveLen(2))
			Expect(result[0].Group).To(Equal("Snapshots"))
			Expect(result[0].Privileges).To(Equal([]string{"VM.Snap.Remove"}))
			Expect(result[1].Group).To(Equal("Power"))
			Expect(result[1].Privileges).To(Equal([]string{"VM.PowerOn"}))
		})

		It("should report all privileges when none are granted", func() {
			granted := map[string]bool{}
			result := comparePrivileges(groups, granted)
			Expect(result).To(HaveLen(2))
			Expect(result[0].Privileges).To(Equal([]string{"VM.Snap.Create", "VM.Snap.Remove"}))
			Expect(result[1].Privileges).To(Equal([]string{"VM.PowerOn", "VM.PowerOff"}))
		})
	})

	Describe("flattenPrivileges", func() {
		It("should flatten all privilege IDs preserving order", func() {
			groups := []privilegeGroup{
				{Description: "A", Privileges: []string{"A.1", "A.2"}},
				{Description: "B", Privileges: []string{"B.1"}},
			}
			result := flattenPrivileges(groups)
			Expect(result).To(Equal([]string{"A.1", "A.2", "B.1"}))
		})

		It("should return nil for empty input", func() {
			result := flattenPrivileges(nil)
			Expect(result).To(BeNil())
		})
	})

	Describe("requiredPrivileges", func() {
		It("should have no duplicate privilege IDs", func() {
			seen := make(map[string]bool)
			for _, group := range requiredPrivileges {
				for _, priv := range group.Privileges {
					Expect(seen[priv]).To(BeFalse(), "duplicate privilege: %s", priv)
					seen[priv] = true
				}
			}
		})

		It("should have non-empty descriptions and privileges", func() {
			for _, group := range requiredPrivileges {
				Expect(group.Description).ToNot(BeEmpty())
				Expect(group.Privileges).ToNot(BeEmpty())
			}
		})
	})

	Describe("FormatMissing", func() {
		It("should produce readable output", func() {
			missing := []MissingPrivileges{
				{Group: "Snapshots", Privileges: []string{"VM.Snap.Create"}},
				{Group: "Power", Privileges: []string{"VM.PowerOn", "VM.PowerOff"}},
			}
			result := FormatMissing(missing)
			Expect(result).To(ContainSubstring("MISSING VSPHERE PRIVILEGES"))
			Expect(result).To(ContainSubstring("Snapshots:"))
			Expect(result).To(ContainSubstring("  - VM.Snap.Create"))
			Expect(result).To(ContainSubstring("Power:"))
			Expect(result).To(ContainSubstring("  - VM.PowerOn"))
			Expect(result).To(ContainSubstring("RESOLUTION"))
		})
	})

	Describe("MissingPrivileges accessor", func() {
		It("should return not-checked before any check runs", func() {
			c := &Collector{missingPrivsMu: &sync.RWMutex{}}
			missing, checked := c.MissingPrivileges()
			Expect(checked).To(BeFalse())
			Expect(missing).To(BeNil())
		})

		It("should return cached results after setting them", func() {
			c := &Collector{missingPrivsMu: &sync.RWMutex{}}
			expected := []MissingPrivileges{
				{Group: "Snapshots", Privileges: []string{"VM.Snap.Create"}},
			}
			c.missingPrivsMu.Lock()
			c.missingPrivs = &privilegeCheckResult{missing: expected}
			c.missingPrivsMu.Unlock()

			missing, checked := c.MissingPrivileges()
			Expect(checked).To(BeTrue())
			Expect(missing).To(HaveLen(1))
			Expect(missing[0].Group).To(Equal("Snapshots"))
		})

		It("should return checked with nil missing when all privileges are granted", func() {
			c := &Collector{missingPrivsMu: &sync.RWMutex{}}
			c.missingPrivsMu.Lock()
			c.missingPrivs = &privilegeCheckResult{}
			c.missingPrivsMu.Unlock()

			missing, checked := c.MissingPrivileges()
			Expect(checked).To(BeTrue())
			Expect(missing).To(BeNil())
		})
	})
})

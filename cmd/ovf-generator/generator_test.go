package main

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGenerator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OVF Generator Suite")
}

const (
	listVMsOutput = "win2019-vm\nrhel9-vm"

	vmInfoWin2019 = `{
  "Name": "win2019-vm",
  "ProcessorCount": 4,
  "MemoryStartup": 8589934592,
  "HardDrives": [{"Path": "C:\\VMs\\win2019-vm\\win2019-vm.vhdx"}],
  "NetworkAdapters": [{"Name": "External Network"}]
}`

	guestOSWin2019 = `{
  "Caption": "Microsoft Windows Server 2019 Standard",
  "Version": "10.0.17763",
  "OSArchitecture": "64-bit"
}`
)

var _ = Describe("OVF Generator", func() {
	var (
		mockExecutor *MockPSExecutor
		generator    *Generator
		ctx          context.Context
	)

	BeforeEach(func() {
		mockExecutor = NewMockPSExecutor()
		generator = NewGenerator(mockExecutor, "")
		ctx = context.Background()
	})

	Describe("listVMs", func() {
		It("parses VM names and handles empty/errors", func() {
			// Success case
			mockExecutor.AddResponse("Get-VM | Select-Object -ExpandProperty Name", listVMsOutput, nil)
			vms, err := generator.listVMs(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(vms).To(Equal([]string{"win2019-vm", "rhel9-vm"}))

			// Empty case
			mockExecutor.Reset()
			mockExecutor.AddResponse("Get-VM | Select-Object -ExpandProperty Name", "", nil)
			vms, err = generator.listVMs(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(vms).To(BeEmpty())
		})
	})

	Describe("getVMInfo", func() {
		It("parses VM JSON correctly", func() {
			mockExecutor.AddResponse("Get-VM -Name 'win2019-vm'", vmInfoWin2019, nil)

			vmInfo, err := generator.getVMInfo(ctx, "win2019-vm")

			Expect(err).ToNot(HaveOccurred())
			Expect(vmInfo["Name"]).To(Equal("win2019-vm"))
			Expect(vmInfo["ProcessorCount"]).To(BeNumerically("==", 4))
		})
	})

	Describe("getGuestOSInfo", func() {
		It("returns OS info or defaults", func() {
			// With KVP data
			mockExecutor.AddResponse("Msvm_ComputerSystem", guestOSWin2019, nil)
			osInfo := generator.getGuestOSInfo(ctx, "win2019-vm")
			Expect(osInfo["Caption"]).To(Equal("Microsoft Windows Server 2019 Standard"))

			// Without KVP data (VM off)
			mockExecutor.Reset()
			mockExecutor.AddResponse("Msvm_ComputerSystem", "null", nil)
			osInfo = generator.getGuestOSInfo(ctx, "stopped-vm")
			Expect(osInfo["Caption"]).To(Equal("Unknown"))
		})
	})

	Describe("extractDiskPaths", func() {
		It("handles array and single-object formats", func() {
			// Array format
			vmInfo := map[string]interface{}{
				"HardDrives": []interface{}{
					map[string]interface{}{"Path": "C:\\disk1.vhdx"},
					map[string]interface{}{"Path": "C:\\disk2.vhdx"},
				},
			}
			Expect(extractDiskPaths(vmInfo)).To(Equal([]string{"C:\\disk1.vhdx", "C:\\disk2.vhdx"}))

			// Single object
			vmInfo = map[string]interface{}{
				"HardDrives": map[string]interface{}{"Path": "C:\\disk1.vhdx"},
			}
			Expect(extractDiskPaths(vmInfo)).To(Equal([]string{"C:\\disk1.vhdx"}))

			// No disks
			Expect(extractDiskPaths(map[string]interface{}{})).To(BeEmpty())
		})
	})
})

var _ = Describe("Validator", func() {
	var (
		mockExecutor *MockPSExecutor
		validator    *Validator
		ctx          context.Context
	)

	BeforeEach(func() {
		mockExecutor = NewMockPSExecutor()
		validator = NewValidator(mockExecutor, "")
		ctx = context.Background()
	})

	Describe("ValidateInput", func() {
		It("rejects invalid paths", func() {
			// Whitespace
			v := NewValidator(mockExecutor, "   ")
			Expect(v.ValidateInput(ctx)).To(HaveOccurred())

			// Invalid chars
			v = NewValidator(mockExecutor, "C:\\<test>")
			Expect(v.ValidateInput(ctx)).To(HaveOccurred())

			// Valid
			v = NewValidator(mockExecutor, "C:\\VMs")
			Expect(v.ValidateInput(ctx)).ToNot(HaveOccurred())
		})
	})

	Describe("ValidateHyperV", func() {
		It("checks module availability and permissions", func() {
			// Available
			mockExecutor.AddResponse("Get-Module -ListAvailable -Name Hyper-V", "AVAILABLE", nil)
			Expect(validator.ValidateHyperVAvailable(ctx)).ToNot(HaveOccurred())

			// Not available
			mockExecutor.Reset()
			mockExecutor.AddResponse("Get-Module -ListAvailable -Name Hyper-V", "NOT_AVAILABLE", nil)
			Expect(validator.ValidateHyperVAvailable(ctx)).To(HaveOccurred())

			// Permissions OK
			mockExecutor.Reset()
			mockExecutor.AddResponse("Get-VM -ErrorAction Stop", "OK", nil)
			Expect(validator.ValidateHyperVPermissions(ctx)).ToNot(HaveOccurred())

			// No permissions
			mockExecutor.Reset()
			mockExecutor.AddResponse("Get-VM -ErrorAction Stop", "NO_PERMISSION", nil)
			Expect(validator.ValidateHyperVPermissions(ctx)).To(HaveOccurred())
		})
	})

	Describe("ValidateVMRequirements", func() {
		It("validates disk, CPU, memory", func() {
			mockExecutor.AddResponse("Test-Path", "EXISTS", nil)

			// Valid VM
			vmInfo := map[string]interface{}{
				"ProcessorCount": float64(2),
				"MemoryStartup":  float64(4294967296),
				"HardDrives":     []interface{}{map[string]interface{}{"Path": "C:\\disk.vhdx"}},
			}
			Expect(validator.ValidateVMRequirements(ctx, "vm1", vmInfo)).ToNot(HaveOccurred())

			// No disks
			vmInfo["HardDrives"] = []interface{}{}
			Expect(validator.ValidateVMRequirements(ctx, "vm1", vmInfo)).To(HaveOccurred())

			// Invalid CPU
			vmInfo["HardDrives"] = []interface{}{map[string]interface{}{"Path": "C:\\disk.vhdx"}}
			vmInfo["ProcessorCount"] = float64(0)
			Expect(validator.ValidateVMRequirements(ctx, "vm1", vmInfo)).To(HaveOccurred())
		})
	})
})

var _ = Describe("Context Cancellation", func() {
	It("stops on cancel", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		mock := NewMockPSExecutor()
		_, err := mock.Execute(ctx, "any")

		Expect(err).To(HaveOccurred())
		psErr, ok := err.(*PSError)
		Expect(ok).To(BeTrue())
		Expect(psErr.IsCancelled()).To(BeTrue())
	})
})

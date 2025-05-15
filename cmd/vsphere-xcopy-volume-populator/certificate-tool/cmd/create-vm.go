package cmd

import (
	"certificate-tool/pkg/vmware"
	"time"

	"github.com/spf13/cobra"
)

var (
	vmName               string
	isoPath              string
	dataStore            string
	guestID              string
	dataCenter           string
	memoryMB             int
	cpus                 int
	network              string
	pool                 string
	cdDeviceKey          string
	guestUser, guestPass string
	dataSizeMB           int
	waitTimeout          time.Duration
	downloadVmdkURL      string
	localVmdkPath        string
)

var createVmCmd = &cobra.Command{
	Use:   "create-vm",
	Short: "Create a VM, attach ISO, and inject data into guest",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := vmware.CreateVM(vmName, vsphereUrl, vsphereUser,
			vspherePassword, dataCenter, dataStore, pool, downloadVmdkURL,
			localVmdkPath, isoPath, waitTimeout)
		return err
	},
}

func init() {
	RootCmd.AddCommand(createVmCmd)

	// Image / source flags
	createVmCmd.Flags().StringVar(&downloadVmdkURL, "download-vmdk-url", "", "URL to download VMDK from (defaults to Ubuntu 20.04)")
	createVmCmd.Flags().StringVar(&localVmdkPath, "local-vmdk-path", "assets/cloudinit/ubuntu-20.04-server-cloudimg-amd64.vmdk", "Path to an already-downloaded VMDK file")
	createVmCmd.Flags().StringVar(&vmName, "vm-name", "", "Name for the new VM")
	createVmCmd.Flags().StringVar(&isoPath, "iso-path", "assets/cloudinit/seed.iso", "ISO path to attach as CD‑ROM")
	createVmCmd.Flags().StringVar(&dataStore, "data-store", "", "Target dataStore name")
	createVmCmd.Flags().StringVar(&dataCenter, "data-center", "", "Target dataStore name")
	createVmCmd.Flags().StringVar(&guestID, "guest-id", "ubuntu64Guest", "VM guest ID")
	createVmCmd.Flags().IntVar(&memoryMB, "memory-mb", 2048, "Memory size (MB)")
	createVmCmd.Flags().IntVar(&cpus, "cpus", 2, "vCPU count")
	createVmCmd.Flags().StringVar(&network, "network", "VM Network", "Network name")
	createVmCmd.Flags().StringVar(&pool, "pool", "Resources", "Resource pool path")
	createVmCmd.Flags().StringVar(&cdDeviceKey, "cd-device-key", "cdrom-3000", "Virtual CD‑ROM device key")
	createVmCmd.Flags().StringVar(&guestUser, "guest-user", "fedora", "Guest OS user")
	createVmCmd.Flags().StringVar(&guestPass, "guest-pass", "password", "Guest OS password")
	createVmCmd.Flags().IntVar(&dataSizeMB, "data-size-mb", 1, "Random data size (MB) to write inside guest")
	createVmCmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "Timeout for vCenter operations")

}

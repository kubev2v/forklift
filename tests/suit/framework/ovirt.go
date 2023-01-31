package framework

import (
	"fmt"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	ovirtsdk "github.com/ovirt/go-ovirt"
	"os"
)

const DefaultStorageClass = "standard"

func (r *OvirtClient) SetupClient() (err error) {
	r.CustomEnv = false
	if env := os.Getenv("OVIRT_CUSTOM_ENV"); env == "true" {
		r.CustomEnv = true
	}

	r.storageClass = DefaultStorageClass
	if sc := os.Getenv("STORAGE_CLASS"); sc != "" {
		r.storageClass = sc
	}

	mandEnvVars := []string{"OVIRT_USERNAME", "OVIRT_PASSWORD", "OVIRT_URL", "OVIRT_CACERT", "OVIRT_VM_ID"}
	for _, field := range mandEnvVars {
		if env := os.Getenv(field); env == "" {
			return fmt.Errorf("%s is not set", field)
		}
	}

	cacertFile := os.Getenv("OVIRT_CACERT")
	fileinput, err := os.ReadFile(cacertFile)
	if err != nil {
		return fmt.Errorf("could not read %s", cacertFile)
	}

	r.Username = os.Getenv("OVIRT_USERNAME")
	r.Password = os.Getenv("OVIRT_PASSWORD")
	r.OvirtURL = os.Getenv("OVIRT_URL")
	r.testVMId = os.Getenv("OVIRT_VM_ID")
	r.Cacert = fileinput

	return
}

// LoadSourceDetails - Load Source VM details from oVirt
func (r *OvirtClient) LoadSourceDetails() (err error) {

	// default test values
	sdomains := []string{"95ef6fee-5773-46a2-9340-a636958a96b8"}
	nics := []string{"6b6b7239-5ea1-4f08-a76e-be150ab8eb89"}

	// if a real custom ovirt environment is used.
	if r.CustomEnv {
		defer r.Close()
		// connect to ovirt instance
		err = r.Connect()

		// get storage domain from the test VM
		sdomains, err = r.getSDFromVM(ref.Ref{ID: r.testVMId})
		if err != nil {
			return fmt.Errorf("error getting storage domains from VM - %v", err)
		}

		// get network interface from the test VM
		nics, err = r.getNicsFromVM(ref.Ref{ID: r.testVMId})
		if err != nil {
			return fmt.Errorf("error getting network interfaces from VM - %v", err)
		}

	}
	r.vmData.sdPairs = sdomains
	r.vmData.nicPairs = nics

	return
}

// Connect - Connect to the oVirt API.
func (r *OvirtClient) Connect() (err error) {
	r.connection, err = ovirtsdk.NewConnectionBuilder().
		URL(r.OvirtURL).
		Username(r.Username).
		Password(r.Password).
		CACert(r.Cacert).
		Insecure(false).
		Build()
	if err != nil {
		return err
	}
	return
}

// Get the VM by ref.
func (r *OvirtClient) getVM(vmRef ref.Ref) (ovirtVm *ovirtsdk.Vm, vmService *ovirtsdk.VmService, err error) {
	vmService = r.connection.SystemService().VmsService().VmService(vmRef.ID)
	vmResponse, err := vmService.Get().Send()
	if err != nil {
		return
	}
	ovirtVm, ok := vmResponse.Vm()
	if !ok {
		err = fmt.Errorf(
			"VM %s source lookup failed",
			vmRef.String())
	}
	return
}

// getNicsFromVM - get network interfaces from specific VM
func (r *OvirtClient) getNicsFromVM(vmRef ref.Ref) (nicIds []string, err error) {
	_, vmService, err := r.getVM(vmRef)
	if err != nil {
		return nil, fmt.Errorf("Failed to get VM - %v", err)
	}

	nicsResponse, err := vmService.NicsService().List().Send()
	nics, ok := nicsResponse.Nics()
	if !ok {
		return nil, fmt.Errorf("Failed to get nics")
	}

	for _, nic := range nics.Slice() {
		vnicService := r.connection.SystemService().VnicProfilesService().ProfileService(nic.MustVnicProfile().MustId())
		vnicResponse, err := vnicService.Get().Send()
		if err != nil {
			return nil, fmt.Errorf("Failed to get vnic service = %v", err)
		}
		profile, ok := vnicResponse.Profile()
		if !ok {
			return nil, fmt.Errorf("Failed to get nic profile")
		}
		network, ok := profile.Network()
		if !ok {
			return nil, fmt.Errorf("Failed to get network")
		}
		networkId, ok := network.Id()
		if !ok {
			return nil, fmt.Errorf("Failed to get network id")
		}
		nicIds = append(nicIds, networkId)
	}
	return
}

// getSDFromVM - get storage domains from specific VM
func (r *OvirtClient) getSDFromVM(vmRef ref.Ref) (storageDomains []string, err error) {
	_, vmService, err := r.getVM(vmRef)
	if err != nil {
		return nil, fmt.Errorf("Failed to get VM - %v", err)
	}

	diskAttachementResponse, err := vmService.DiskAttachmentsService().List().Send()
	if err != nil {
		return nil, fmt.Errorf("Failed to get disk attachment service  %v", err)
	}
	disks, ok := diskAttachementResponse.Attachments()
	if !ok {
		return nil, fmt.Errorf("Failed to get disks")
	}
	for _, da := range disks.Slice() {
		disk, ok := da.Disk()
		if !ok {
			return nil, fmt.Errorf("Failed to get disks")
		}
		diskService := r.connection.SystemService().DisksService().DiskService(disk.MustId())
		diskResponse, err := diskService.Get().Send()
		if err != nil {
			return nil, fmt.Errorf("Failed to get disks - %v", err)
		}
		sds, ok := diskResponse.MustDisk().StorageDomains()
		if !ok {
			return nil, fmt.Errorf("Failed to get storage domains")
		}
		for _, sd := range sds.Slice() {
			sdId, ok := sd.Id()
			if !ok {
				return nil, fmt.Errorf("Failed to get storage domain id")
			}
			storageDomains = append(storageDomains, sdId)
		}
	}
	return
}

// GetVMNics - return the network interface for the VM
func (r *OvirtClient) GetVMNics() []string {
	return r.vmData.nicPairs
}

// GetVMSDs - return storage domain IDs
func (r *OvirtClient) GetVMSDs() []string {
	return r.vmData.sdPairs
}

// GetTestVMId - return the test VM ID
func (r *OvirtClient) GetTestVMId() string {
	return r.testVMId
}

// Close the connection to the oVirt API.
func (r *OvirtClient) Close() {
	if r.connection != nil {
		_ = r.connection.Close()
		r.connection = nil
	}
}

// OvirtClient - oVirt VM Client
type OvirtClient struct {
	connection   *ovirtsdk.Connection
	Cacert       []byte
	Username     string
	OvirtURL     string
	Password     string
	testVMId     string
	storageClass string
	vmData       OvirtVM
	CustomEnv    bool
}

type OvirtVM struct {
	nicPairs []string
	sdPairs  []string
}

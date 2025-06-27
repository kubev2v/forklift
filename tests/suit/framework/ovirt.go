package framework

import (
	"fmt"
	"os"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	ovirtsdk "github.com/ovirt/go-ovirt"
)

const DefaultStorageClass = "nfs-csi"

func (r *OvirtClient) SetupClient(insecure bool) (err error) {
	r.CustomEnv = false
	if env := os.Getenv("OVIRT_CUSTOM_ENV"); env == "true" {
		r.CustomEnv = true
	}

	r.storageClass = DefaultStorageClass
	if sc := os.Getenv("STORAGE_CLASS"); sc != "" {
		r.storageClass = sc
	}

	envVars := []string{"OVIRT_USERNAME", "OVIRT_PASSWORD", "OVIRT_URL"}

	if !insecure {
		envVars = append(envVars, "OVIRT_CACERT", "OVIRT_VM_ID")
	} else {
		envVars = append(envVars, "OVIRT_INSECURE_VM_ID")
	}
	for _, field := range envVars {
		if env := os.Getenv(field); env == "" {
			return fmt.Errorf("%s is not set", field)
		}
	}

	if !insecure {
		cacertFile := os.Getenv("OVIRT_CACERT")
		fileinput, err := os.ReadFile(cacertFile)
		if err != nil {
			return fmt.Errorf("could not read %s", cacertFile)
		}
		r.Cacert = fileinput
		r.Insecure = false
		r.vmData.testVMId = os.Getenv("OVIRT_VM_ID")
	} else {
		r.Insecure = true
		r.vmData.testVMId = os.Getenv("OVIRT_INSECURE_VM_ID")
	}

	r.Username = os.Getenv("OVIRT_USERNAME")
	r.Password = os.Getenv("OVIRT_PASSWORD")
	r.OvirtURL = os.Getenv("OVIRT_URL")
	return
}

// LoadSourceDetails - Load Source VM details from oVirt
func (r *OvirtClient) LoadSourceDetails() (vm *OvirtVM, err error) {

	// default test values
	sdomains := []string{"95ef6fee-5773-46a2-9340-a636958a96b8"}
	nics := []string{"6b6b7239-5ea1-4f08-a76e-be150ab8eb89"}
	// if a real custom ovirt environment is used.
	if r.CustomEnv {
		defer r.Close()
		err = r.Connect()

		if err != nil {
			return nil, fmt.Errorf("error connecting to oVirt engine - %v", err)
		}

		// get storage domain from the test VM
		sdomains, err = r.vmData.getSDFromVM(r.connection, ref.Ref{ID: r.vmData.testVMId})
		if err != nil {
			return nil, fmt.Errorf("error getting storage domains from VM - %v", err)
		}

		// get network interface from the test VM
		nics, err = r.vmData.getNicsFromVM(r.connection, ref.Ref{ID: r.vmData.testVMId})
		if err != nil {
			return nil, fmt.Errorf("error getting network interfaces from VM - %v", err)
		}

	}
	r.vmData.sdPairs = sdomains
	r.vmData.nicPairs = nics

	return &r.vmData, nil
}

// Connect - Connect to the oVirt API.
func (r *OvirtClient) Connect() (err error) {
	builder := ovirtsdk.NewConnectionBuilder().
		URL(r.OvirtURL).
		Username(r.Username).
		Password(r.Password)

	if r.Insecure {
		builder = builder.Insecure(r.Insecure)
	} else {
		builder = builder.CACert(r.Cacert)
	}

	r.connection, err = builder.Build()
	if err != nil {
		return err
	}
	return
}

// Close the connection to the oVirt API.
func (r *OvirtClient) Close() {
	if r.connection != nil {
		_ = r.connection.Close()
		r.connection = nil
	}
}

// Get the VM by ref.
func (r *OvirtVM) getVM(ovirtConn *ovirtsdk.Connection, vmRef ref.Ref) (vmService *ovirtsdk.VmService, err error) {
	vmService = ovirtConn.SystemService().VmsService().VmService(vmRef.ID)
	vmResponse, err := vmService.Get().Send()
	if err != nil {
		return
	}
	_, ok := vmResponse.Vm()
	if !ok {
		err = fmt.Errorf(
			"VM %s source lookup failed",
			vmRef.String())
	}
	return
}

// getNicsFromVM - get network interfaces from specific VM
func (r *OvirtVM) getNicsFromVM(ovirtConn *ovirtsdk.Connection, vmRef ref.Ref) (nicIds []string, err error) {
	vmService, err := r.getVM(ovirtConn, vmRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM - %v", err)
	}

	nicsResponse, err := vmService.NicsService().List().Send()
	nics, ok := nicsResponse.Nics()
	if !ok {
		return nil, fmt.Errorf("failed to get nics")
	}

	for _, nic := range nics.Slice() {
		vnicService := ovirtConn.SystemService().VnicProfilesService().ProfileService(nic.MustVnicProfile().MustId())
		vnicResponse, err := vnicService.Get().Send()
		if err != nil {
			return nil, fmt.Errorf("failed to get vnic service = %v", err)
		}
		profile, ok := vnicResponse.Profile()
		if !ok {
			return nil, fmt.Errorf("failed to get nic profile")
		}
		network, ok := profile.Network()
		if !ok {
			return nil, fmt.Errorf("failed to get network")
		}
		networkId, ok := network.Id()
		if !ok {
			return nil, fmt.Errorf("failed to get network id")
		}
		nicIds = append(nicIds, networkId)
	}
	return
}

// getSDFromVM - get storage domains from specific VM
func (r *OvirtVM) getSDFromVM(ovirtConn *ovirtsdk.Connection, vmRef ref.Ref) (storageDomains []string, err error) {
	vmService, err := r.getVM(ovirtConn, vmRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM - %v", err)
	}

	diskAttachementResponse, err := vmService.DiskAttachmentsService().List().Send()
	if err != nil {
		return nil, fmt.Errorf("failed to get disk attachment service  %v", err)
	}
	disks, ok := diskAttachementResponse.Attachments()
	if !ok {
		return nil, fmt.Errorf("failed to get disks")
	}
	for _, da := range disks.Slice() {
		disk, ok := da.Disk()
		if !ok {
			return nil, fmt.Errorf("failed to get disks")
		}
		diskService := ovirtConn.SystemService().DisksService().DiskService(disk.MustId())
		diskResponse, err := diskService.Get().Send()
		if err != nil {
			return nil, fmt.Errorf("failed to get disks - %v", err)
		}
		sds, ok := diskResponse.MustDisk().StorageDomains()
		if !ok {
			return nil, fmt.Errorf("failed to get storage domains")
		}
		for _, sd := range sds.Slice() {
			sdId, ok := sd.Id()
			if !ok {
				return nil, fmt.Errorf("failed to get storage domain id")
			}
			storageDomains = append(storageDomains, sdId)
		}
	}
	return
}

// GetVMNics - return the network interface for the VM
func (r *OvirtVM) GetVMNics() []string {
	return r.nicPairs
}

// GetVMSDs - return storage domain IDs
func (r *OvirtVM) GetVMSDs() []string {
	return r.sdPairs
}

// GetTestVMId - return the test VM ID
func (r *OvirtVM) GetTestVMId() string {
	return r.testVMId
}

// OvirtClient - oVirt VM Client
type OvirtClient struct {
	connection   *ovirtsdk.Connection
	Cacert       []byte
	Username     string
	OvirtURL     string
	Password     string
	storageClass string
	vmData       OvirtVM
	CustomEnv    bool
	Insecure     bool
}

type OvirtVM struct {
	nicPairs []string
	sdPairs  []string
	testVMId string
}

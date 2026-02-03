package vsphere_offload

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/lib/vsphere_offload/vmware"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"k8s.io/klog/v2"
)

const (
	vibName = "vmkfstools-wrapper"
)

// VibVersion is set by ldflags
var VibVersion = "x.x.x"

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/vib_ensurer_mock.go -package=vsphere_offload_mocks . VIBEnsurer

// VIBEnsurer interface for ensuring VIB installation on ESXi hosts
type VIBEnsurer interface {
	EnsureVib(ctx context.Context, client vmware.Client, esx *object.HostSystem, localVibPath string) error
}

// DefaultVIBEnsurer is the production implementation
type DefaultVIBEnsurer struct{}

// EnsureVib implements VIBEnsurer interface - fetches the vib version and installs it if needed
func (d *DefaultVIBEnsurer) EnsureVib(ctx context.Context, client vmware.Client, esx *object.HostSystem, localVibPath string) error {
	version, err := getVIBVersion(ctx, client, esx)
	if err != nil {
		return fmt.Errorf("failed to get the VIB version from ESXi %s: %w", esx.Name(), err)
	}

	if version == VibVersion {
		return nil
	}
	dc, err := GetHostDC(ctx, esx)
	if err != nil {
		return err
	}
	datastore, err := GetHostDatastore(ctx, esx)
	if err != nil {
		return fmt.Errorf("failed to get datastore for ESXi %s", esx.Name())
	}
	vibPath, err := uploadVib(ctx, client, dc, datastore, localVibPath)
	if err != nil {
		return fmt.Errorf("failed to upload the VIB to ESXi %s: %w", esx.Name(), err)
	}
	klog.Infof("uploaded vib to ESXi %s", esx.Name())

	err = installVib(ctx, client, esx, vibPath)
	if err != nil {
		return fmt.Errorf("failed to install the VIB on ESXi %s: %w", esx.Name(), err)
	}
	klog.Infof("installed vib on ESXi %s version %s", esx.Name(), VibVersion)
	return nil
}

// GetHostDC retrieves the datacenter for a given ESXi host
func GetHostDC(ctx context.Context, esx *object.HostSystem) (*object.Datacenter, error) {
	hostRef := esx.Reference()
	pc := property.DefaultCollector(esx.Client())
	var hostMo mo.HostSystem
	err := pc.RetrieveOne(ctx, hostRef, []string{"parent"}, &hostMo)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve host parent: %w", err)
	}

	parentRef := hostMo.Parent
	var datacenter *object.Datacenter
	currentParentRef := parentRef

	// walk the parents of the host up till the datacenter
	for {
		if currentParentRef.Type == "Datacenter" {
			finder := find.NewFinder(esx.Client(), true)
			datacenter, err = finder.Datacenter(ctx, currentParentRef.String())
			if err != nil {
				return nil, err
			}
			return datacenter, nil
		}

		var genericParentMo mo.ManagedEntity
		err = pc.RetrieveOne(ctx, *currentParentRef, []string{"parent"}, &genericParentMo)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve intermediate parent: %w", err)
		}

		if genericParentMo.Parent == nil {
			break
		}
		currentParentRef = genericParentMo.Parent
	}

	return nil, fmt.Errorf("could not determine datacenter for host '%s'.", esx.Name())
}

// GetHostDatastore retrieves the first available datastore from a given ESXi host
func GetHostDatastore(ctx context.Context, esx *object.HostSystem) (string, error) {
	var hostMo mo.HostSystem
	pc := property.DefaultCollector(esx.Client())
	err := pc.RetrieveOne(ctx, esx.Reference(), []string{"datastore"}, &hostMo)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve host datastores for %s: %w", esx.Name(), err)
	}

	if len(hostMo.Datastore) == 0 {
		return "", fmt.Errorf("no datastores found on host %s", esx.Name())
	}

	// Get the first datastore and load its properties to get the name
	ds := object.NewDatastore(esx.Client(), hostMo.Datastore[0])
	var dsMo mo.Datastore
	err = pc.RetrieveOne(ctx, ds.Reference(), []string{"name"}, &dsMo)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve datastore name for %s: %w", ds.Reference(), err)
	}

	return dsMo.Name, nil
}

func getVIBVersion(ctx context.Context, client vmware.Client, esxi *object.HostSystem) (string, error) {
	r, err := client.RunEsxCommand(ctx, esxi, []string{"software", "vib", "get", "-n", vibName})
	if err != nil {
		vFault, conversionErr := vmware.ErrToFault(err)
		if conversionErr != nil {
			return "", err
		}
		if vFault != nil {
			for _, m := range vFault.ErrMsgs {
				if strings.Contains(m, "[NoMatchError]") {
					// vib is not installed. return empty object
					return "", nil
				}
			}
		}
		return "", err
	}

	return r[0].Value("Version"), err
}

func uploadVib(ctx context.Context, client vmware.Client, dc *object.Datacenter, datastore string, localVibPath string) (string, error) {
	ds, err := client.GetDatastore(ctx, dc, datastore)
	if err != nil {
		return "", fmt.Errorf("failed to get datastore for VIB upload: %w", err)
	}
	destFilename := vibName + ".vib"
	if err = ds.UploadFile(ctx, localVibPath, destFilename, nil); err != nil {
		return "", fmt.Errorf("failed to upload %s: %w", localVibPath, err)
	}
	return fmt.Sprintf("/vmfs/volumes/%s/%s", datastore, destFilename), nil
}

func installVib(ctx context.Context, client vmware.Client, esx *object.HostSystem, vibPath string) error {
	r, err := client.RunEsxCommand(ctx, esx, []string{"software", "vib", "install", "-f", "1", "-v", vibPath})
	if err != nil {
		return err
	}

	if len(r) > 0 {
		if vibsSkipped, ok := r[0]["VIBsSkipped"]; ok && len(vibsSkipped) > 0 {
			message := "unknown reason"
			if msg, ok := r[0]["Message"]; ok && len(msg) > 0 {
				message = msg[0]
			}
			skippedVib := ""
			if len(vibsSkipped) > 0 {
				skippedVib = vibsSkipped[0]
			}
			return fmt.Errorf("VIB installation was skipped by ESXi (host already has '%s' installed, desired version is '%s'). ESXi message: %s", skippedVib, VibVersion, message)
		}

		if vibsInstalled, ok := r[0]["VIBsInstalled"]; ok && len(vibsInstalled) > 0 {
			return nil
		}
	}

	klog.Warningf("Unexpected VIB install response format: %v", r)
	return nil
}

// ShouldSkipVIBCheck checks if VIB validation should be skipped based on cache duration
// Returns true if the condition was last updated less than cacheDuration ago
func ShouldSkipVIBCheck(lastTransitionTime time.Time, cacheDuration time.Duration) bool {
	if lastTransitionTime.IsZero() {
		return false
	}
	return time.Since(lastTransitionTime) < cacheDuration
}

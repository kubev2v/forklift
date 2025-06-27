package populator

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"k8s.io/klog/v2"
)

const (
	vibName     = "vmkfstools-wrapper"
	vibLocation = "/bin/vmkfstools-wrapper.vib"
)

// VibVersion is set by ldflags
var VibVersion = "x.x.x"

// ensure vib will fetch the vib version and in case needed will install it
// on the target ESX
func ensureVib(client vmware.Client, esx *object.HostSystem, datastore string, desiredVibVersion string) error {
	klog.Infof("ensuring vib version on ESXi %s: %s", esx.Name(), VibVersion)

	version, err := getViBVersion(client, esx)
	if err != nil {
		return fmt.Errorf("failed to get the VIB version from ESXi %s: %w", esx.Name(), err)
	}

	klog.Infof("current vib version on ESXi %s: %s", esx.Name(), version)
	if version == desiredVibVersion {
		return nil
	}

	dc, err := getHostDC(esx)
	if err != nil {
		return err
	}
	vibPath, err := uploadVib(client, dc, datastore)
	if err != nil {
		return fmt.Errorf("failed to upload the VIB to ESXi %s: %w", esx.Name(), err)
	}
	klog.Infof("uploaded vib to ESXi %s", esx.Name())

	err = installVib(client, esx, vibPath)
	if err != nil {
		return fmt.Errorf("failed to install the VIB on ESXi %s: %w", esx.Name(), err)
	}
	klog.Infof("installed vib on ESXi %s version %s", esx.Name(), VibVersion)
	return nil
}

func getHostDC(esx *object.HostSystem) (*object.Datacenter, error) {
	ctx := context.Background()
	hostRef := esx.Reference()
	pc := property.DefaultCollector(esx.Client())
	var hostMo mo.HostSystem
	err := pc.RetrieveOne(context.Background(), hostRef, []string{"parent"}, &hostMo)
	if err != nil {
		klog.Fatalf("failed to retrieve host parent: %v", err)
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
		err = pc.RetrieveOne(context.Background(), *currentParentRef, []string{"parent"}, &genericParentMo)
		if err != nil {
			klog.Fatalf("failed to retrieve intermediate parent: %v", err)
		}

		if genericParentMo.Parent == nil {
			break
		}
		currentParentRef = genericParentMo.Parent
	}

	return nil, fmt.Errorf("could not determine datacenter for host '%s'.", esx.Name())
}

func getViBVersion(client vmware.Client, esxi *object.HostSystem) (string, error) {
	r, err := client.RunEsxCommand(context.Background(), esxi, []string{"software", "vib", "get", "-n", vibName})
	if err != nil {
		vFault, conversonErr := vmware.ErrToFault(err)
		if conversonErr != nil {
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

	klog.Infof("reply from get vib %v", r)
	return r[0].Value("Version"), err
}

func uploadVib(client vmware.Client, dc *object.Datacenter, datastore string) (string, error) {
	ds, err := client.GetDatastore(context.Background(), dc, datastore)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	if err = ds.UploadFile(context.Background(), vibLocation, vibName+".vib", nil); err != nil {
		return "", fmt.Errorf("failed to upload %s: %w", vibLocation, err)
	}
	return fmt.Sprintf("/vmfs/volumes/%s/%s", datastore, vibName+".vib"), nil
}

func installVib(client vmware.Client, esx *object.HostSystem, vibPath string) error {
	r, err := client.RunEsxCommand(context.Background(), esx, []string{"software", "vib", "install", "-f", "1", "-v", vibPath})
	if err != nil {
		return err
	}

	klog.Infof("reply from get vib %v", r)
	return nil
}

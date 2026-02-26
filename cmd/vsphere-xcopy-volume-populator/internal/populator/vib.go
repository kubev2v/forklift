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

// ensure vib will fetch the vib version and in case needed will install it
// on the target ESX. Caller should pass a context; vib adds its logger to it for RunEsxCommand.
func ensureVib(ctx context.Context, client vmware.Client, esx *object.HostSystem, datastore string, desiredVibVersion string) error {
	log := klog.Background().WithName("copy-offload").WithName("vib")
	ctx = klog.NewContext(ctx, log)
	log.Info("ensuring VIB version on ESXi", "host", esx.Name(), "desired_version", desiredVibVersion)

	currentVersion, err := getViBVersion(ctx, client, esx)
	if err != nil {
		return fmt.Errorf("failed to get the VIB version from ESXi %s: %w", esx.Name(), err)
	}

	log.Info("current VIB version on ESXi", "host", esx.Name(), "version", currentVersion)
	if currentVersion == desiredVibVersion {
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
	log.Info("uploaded VIB to ESXi", "host", esx.Name())

	err = installVib(ctx, client, esx, vibPath)
	if err != nil {
		return fmt.Errorf("failed to install the VIB on ESXi %s: %w", esx.Name(), err)
	}
	log.Info("installed VIB on ESXi", "host", esx.Name(), "version", desiredVibVersion)
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

func getViBVersion(ctx context.Context, client vmware.Client, esxi *object.HostSystem) (string, error) {
	r, err := client.RunEsxCommand(ctx, esxi, []string{"software", "vib", "get", "-n", vibName})
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

	klog.FromContext(ctx).V(2).Info("VIB get result", "response", r)
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

func installVib(ctx context.Context, client vmware.Client, esx *object.HostSystem, vibPath string) error {
	r, err := client.RunEsxCommand(ctx, esx, []string{"software", "vib", "install", "-f", "1", "-v", vibPath})
	if err != nil {
		return err
	}

	klog.FromContext(ctx).V(2).Info("VIB install result", "response", r)
	return nil
}

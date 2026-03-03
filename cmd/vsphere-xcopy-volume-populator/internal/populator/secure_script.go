package populator

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	vmkfstoolswrapper "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/vmkfstools-wrapper"
	"github.com/vmware/govmomi/object"
	"k8s.io/klog/v2"
)

const (
	scriptName = "secure-vmkfstools-wrapper"
)

// writeSecureScriptToTemp writes the embedded script to a temporary file
func writeSecureScriptToTemp() (string, error) {
	tempFile, err := os.CreateTemp("", "secure-vmkfstools-wrapper-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	_, err = tempFile.Write(vmkfstoolswrapper.Script)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write script content: %w", err)
	}

	return tempFile.Name(), nil
}

// ensureSecureScript ensures the secure script is uploaded and available on the target ESX
// Returns the script path
func ensureSecureScript(ctx context.Context, client vmware.Client, esx *object.HostSystem, datastore string) (string, error) {
	log := klog.Background().WithName("copy-offload").WithName("secure-script")
	log.Info("ensuring secure script on ESXi", "host", esx.Name())
	log.Info("force uploading secure script to ensure latest version")

	dc, err := getHostDC(esx)
	if err != nil {
		return "", err
	}

	scriptPath, err := uploadScript(ctx, client, dc, datastore)
	if err != nil {
		return "", fmt.Errorf("failed to upload the secure script to ESXi %s: %w", esx.Name(), err)
	}
	log.Info("uploaded secure script to ESXi", "host", esx.Name(), "path", scriptPath)

	return scriptPath, nil
}

func uploadScript(ctx context.Context, client vmware.Client, dc *object.Datacenter, datastore string) (string, error) {
	// Lookup datastore with timeout
	dsCtx, dsCancel := context.WithTimeout(ctx, 30*time.Second)
	defer dsCancel()
	ds, err := client.GetDatastore(dsCtx, dc, datastore)
	if err != nil {
		return "", fmt.Errorf("failed to get datastore: %w", err)
	}

	// Write embedded script to temporary file
	tempScriptPath, err := writeSecureScriptToTemp()
	if err != nil {
		return "", fmt.Errorf("failed to write embedded script to temp file: %w", err)
	}
	defer os.Remove(tempScriptPath) // Clean up temp file

	log := klog.Background().WithName("copy-offload").WithName("secure-script")
	log.Info("uploading embedded script to datastore", "script", scriptName)

	// Upload the file with timeout
	upCtx, upCancel := context.WithTimeout(ctx, 30*time.Second)
	defer upCancel()
	if err = ds.UploadFile(upCtx, tempScriptPath, scriptName, nil); err != nil {
		return "", fmt.Errorf("failed to upload embedded script: %w", err)
	}

	datastorePath := fmt.Sprintf("/vmfs/volumes/%s/%s", datastore, scriptName)
	log.Info("uploaded embedded script to datastore", "path", datastorePath)
	return datastorePath, nil
}

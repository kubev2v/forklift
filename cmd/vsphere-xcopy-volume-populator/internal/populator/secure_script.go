package populator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	vmkfstoolswrapper "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/vmkfstools-wrapper"
	"github.com/vmware/govmomi/object"
	"k8s.io/klog/v2"
)

const (
	secureScriptName = "secure-vmkfstools-wrapper"
	contextTimeout   = 5 * time.Minute
)

// writeSecureScriptToTemp writes the embedded script to a temporary file
func writeSecureScriptToTemp() (string, error) {
	tempFile, err := os.CreateTemp("", "secure-vmkfstools-wrapper-*.py")
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
// Returns the script path and UUID separately
func ensureSecureScript(ctx context.Context, client vmware.Client, esx *object.HostSystem, datastore string) (string, uuid.UUID, error) {
	klog.Infof("ensuring secure script on ESXi %s", esx.Name())

	// ALWAYS force re-upload to ensure latest version
	klog.Infof("Force uploading secure script to ensure latest version")

	dc, err := getHostDC(esx)
	if err != nil {
		return "", uuid.Nil, err
	}

	scriptPath, UUID, err := uploadScript(ctx, client, dc, datastore)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("failed to upload the secure script to ESXi %s: %w", esx.Name(), err)
	}
	// Script will execute directly from datastore using UUID filename
	klog.Infof("uploaded secure script to ESXi %s at %s (UUID: %s) - ready for execution", esx.Name(), scriptPath, UUID.String())

	// Return path and UUID separately
	return scriptPath, UUID, nil
}

func uploadScript(ctx context.Context, client vmware.Client, dc *object.Datacenter, datastore string) (string, uuid.UUID, error) {
	// Lookup datastore with timeout
	dsCtx, dsCancel := context.WithTimeout(ctx, contextTimeout)
	defer dsCancel()
	ds, err := client.GetDatastore(dsCtx, dc, datastore)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("failed to get datastore: %w", err)
	}

	// Write embedded script to temporary file
	tempScriptPath, err := writeSecureScriptToTemp()
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("failed to write embedded script to temp file: %w", err)
	}
	defer os.Remove(tempScriptPath) // Clean up temp file

	UUID := uuid.New()
	scriptName := fmt.Sprintf("%s-%s.py", secureScriptName, UUID.String())

	klog.Infof("Uploading embedded script to datastore as %s (with UUID to prevent race conditions)", scriptName)

	// Upload the file with timeout to unique UUID filename
	upCtx, upCancel := context.WithTimeout(ctx, contextTimeout)
	defer upCancel()
	if err = ds.UploadFile(upCtx, tempScriptPath, scriptName, nil); err != nil {
		return "", uuid.Nil, fmt.Errorf("failed to upload embedded script: %w", err)
	}

	datastorePath := fmt.Sprintf("/vmfs/volumes/%s/%s", datastore, scriptName)
	klog.Infof("Successfully uploaded embedded script to datastore path: %s", datastorePath)
	return datastorePath, UUID, nil
}

func cleanupSecureScript(ctx context.Context, client vmware.Client, dc *object.Datacenter, datastore, scriptName string) {
	expectedPrefix := secureScriptName
	if !strings.HasPrefix(scriptName, expectedPrefix) {
		klog.Errorf("Refusing to delete file %s: filename must start with %s", scriptName, expectedPrefix)
		return
	}

	if !strings.HasSuffix(scriptName, ".py") {
		klog.Errorf("Refusing to delete file %s: filename must end with .py", scriptName)
		return
	}

	dsCtx, dsCancel := context.WithTimeout(ctx, contextTimeout)
	defer dsCancel()
	ds, err := client.GetDatastore(dsCtx, dc, datastore)
	if err != nil {
		klog.Warningf("Failed to get datastore for cleanup: %v", err)
		return
	}

	fileManager := ds.NewFileManager(dc, false)

	delCtx, delCancel := context.WithTimeout(ctx, contextTimeout)
	defer delCancel()
	if err := fileManager.DeleteFile(delCtx, scriptName); err != nil {
		klog.Warningf("Failed to cleanup script file %s: %v (non-critical)", scriptName, err)
	} else {
		klog.V(2).Infof("Successfully cleaned up script file %s", scriptName)
	}
}

package populator

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"github.com/vmware/govmomi/object"
	"k8s.io/klog/v2"
)

const (
	secureScriptName = "secure-vmkfstools-wrapper"
)

// embeddedSecureScript contains the Python script content from the embedded file
//
//go:embed secure-vmkfstools-wrapper.py
var embeddedSecureScript []byte

// writeSecureScriptToTemp writes the embedded script to a temporary file
func writeSecureScriptToTemp() (string, error) {
	tempFile, err := os.CreateTemp("", "secure-vmkfstools-wrapper-*.py")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	_, err = tempFile.Write(embeddedSecureScript)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write script content: %w", err)
	}

	return tempFile.Name(), nil
}

// ensureSecureScript ensures the secure script is uploaded and available on the target ESX
func ensureSecureScript(ctx context.Context, client vmware.Client, esx *object.HostSystem, datastore string) (string, error) {
	klog.Infof("ensuring secure script on ESXi %s", esx.Name())

	// ALWAYS force re-upload to ensure latest version
	klog.Infof("Force uploading secure script to ensure latest version")

	dc, err := getHostDC(esx)
	if err != nil {
		return "", err
	}

	scriptPath, err := uploadScript(ctx, client, dc, datastore)
	if err != nil {
		return "", fmt.Errorf("failed to upload the secure script to ESXi %s: %w", esx.Name(), err)
	}
	// Script will execute directly from datastore - no need for shell commands
	klog.Infof("uploaded secure script to ESXi %s at %s - ready for execution", esx.Name(), scriptPath)

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

	scriptName := fmt.Sprintf("%s.py", secureScriptName)
	klog.Infof("Uploading embedded script to datastore as %s", scriptName)

	// Upload the file with timeout
	upCtx, upCancel := context.WithTimeout(ctx, 30*time.Second)
	defer upCancel()
	if err = ds.UploadFile(upCtx, tempScriptPath, scriptName, nil); err != nil {
		return "", fmt.Errorf("failed to upload embedded script: %w", err)
	}

	datastorePath := fmt.Sprintf("/vmfs/volumes/%s/%s", datastore, scriptName)
	klog.Infof("Successfully uploaded embedded script to datastore path: %s", datastorePath)
	return datastorePath, nil
}

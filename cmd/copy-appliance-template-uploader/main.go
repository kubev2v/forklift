package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kubev2v/forklift/pkg/copyappliancetemplate/version"
	templatevsphere "github.com/kubev2v/forklift/pkg/copyappliancetemplate/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	libvsphere "github.com/kubev2v/forklift/pkg/lib/vsphere"
	"github.com/kubev2v/forklift/pkg/settings"
)

var log = logging.WithName("copy-appliance-template-uploader")

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "copy-appliance-template-uploader: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	url := os.Getenv(settings.VCenterURL)
	log.Info("Connecting to vCenter", "url", url)
	gc, err := libvsphere.Connect(
		ctx,
		url,
		os.Getenv(settings.VCenterUser),
		os.Getenv(settings.VCenterPassword),
		os.Getenv(settings.VCenterThumbprint),
		settings.LookupBool(settings.VCenterInsecure, false),
	)
	if err != nil {
		return err
	}
	client, err := templatevsphere.NewClient(gc)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close(ctx) }()

	opts := templatevsphere.ImportOptions{
		FolderPath:         settings.Lookup(settings.CopyApplianceFolder, ""),
		Datastore:          settings.Lookup(settings.CopyApplianceDatastore, ""),
		Network:            settings.Lookup(settings.CopyApplianceNetwork, settings.DefaultCopyApplianceNetwork),
		Name:               os.Getenv(settings.CopyApplianceTemplateName),
		VMDKPath:           settings.Lookup(settings.CopyApplianceTemplateVMDKPath, settings.DefaultBuildPodVMDKPath),
		CPUs:               int32(settings.LookupInt(settings.CopyApplianceTemplateBuildPodCPUs, int(version.DefaultCPU))),
		MemoryMiB:          int32(settings.LookupInt(settings.CopyApplianceTemplateBuildPodMemoryMiB, int(version.DefaultMemoryMiB))),
		TemplateDiskHash:   os.Getenv(settings.CopyApplianceTemplateContentHash),
		TemplateConfigHash: os.Getenv(settings.CopyApplianceTemplateConfigHash),
		BaseContainerImage: os.Getenv(settings.CopyApplianceTemplateBaseContainerImage),
	}
	_ = client.DestroyIfExists(ctx, opts.FolderPath, opts.Name)
	ref, err := client.ImportOVF(ctx, opts)
	if err != nil {
		return err
	}
	log.Info("Upload complete", "moref", ref.Moref)
	return nil
}

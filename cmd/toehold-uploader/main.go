package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kubev2v/forklift/pkg/lib/logging"
	libvsphere "github.com/kubev2v/forklift/pkg/lib/vsphere"
	"github.com/kubev2v/forklift/pkg/settings"
	"github.com/kubev2v/forklift/pkg/toehold/version"
	toeholdvsphere "github.com/kubev2v/forklift/pkg/toehold/vsphere"
)

var log = logging.WithName("toehold-uploader")

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "toehold-uploader: %v\n", err)
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
	client, err := toeholdvsphere.NewClient(gc)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close(ctx) }()

	opts := toeholdvsphere.ImportOptions{
		FolderPath:         settings.Lookup(settings.ToeholdFolder, ""),
		Datastore:          settings.Lookup(settings.ToeholdDatastore, ""),
		Network:            settings.Lookup(settings.ToeholdNetwork, settings.DefaultToeholdNetwork),
		Name:               os.Getenv(settings.ToeholdTemplateName),
		VMDKPath:           settings.Lookup(settings.ToeholdVMDKPath, settings.DefaultBuildPodVMDKPath),
		CPUs:               int32(settings.LookupInt(settings.ToeholdBuildPodCPUs, int(version.DefaultCPU))),
		MemoryMiB:          int32(settings.LookupInt(settings.ToeholdBuildPodMemoryMiB, int(version.DefaultMemoryMiB))),
		TemplateDiskHash:   os.Getenv(settings.ToeholdTemplateContentHash),
		TemplateConfigHash: os.Getenv(settings.ToeholdTemplateConfigHash),
		BaseContainerImage: os.Getenv(settings.ToeholdBaseContainerImage),
	}
	_ = client.DestroyIfExists(ctx, opts.FolderPath, opts.Name)
	ref, err := client.ImportOVF(ctx, opts)
	if err != nil {
		return err
	}
	log.Info("Upload complete", "moref", ref.Moref)
	return nil
}

package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/conversion"
)

func main() {
	env := &config.AppConfig{}
	err := env.Load()
	if err != nil {
		fmt.Println("Failed to load variables", err)
		os.Exit(1)
	}
	if err = linkCertificates(env); err != nil {
		fmt.Println("Failed to link the certificates", err)
		os.Exit(1)
	}
	if err = createV2vOutputDir(env); err != nil {
		fmt.Println("Failed to create v2v output dir", err)
		os.Exit(1)
	}
	convert, err := conversion.NewConversion(env)
	if err != nil {
		fmt.Println("Failed prepare conversion", err)
		os.Exit(1)
	}

	// Remote inspection pod: inspect source disks only, no conversion.
	if env.IsRemoteInspection {
		fatalIfErr(convert.RunRemoteV2vInspection(), "Failed to execute virt-v2v-inspector command")
		return
	}

	// Conversion pod: virt-v2v (or in-place), then inspect and customize if needed.
	fatalIfErr(runConversion(env, convert), "Failed to execute virt-v2v command")

	// In remote migrations we cannot connect to the conversion pod from the controller.
	// This connection is needed to get configuration from virt-v2v or virt-v2v-inspector.
	// We expose those parameters via server in this pod; the controller fetches them and
	// then sends a request to terminate the pod.
	if convert.IsLocalMigration {
		fatalIfErr(startServer(env), "failed to run the server")
	}
}

func runConversion(env *config.AppConfig, convert *conversion.Conversion) error {
	var err error

	switch {
	case convert.IsInPlace && convert.LibvirtUrl != "":
		err = runInPlaceLibvirt(convert)
	case convert.IsInPlace:
		err = runInPlaceDisk(convert)
	case env.IsVsphereMigration():
		err = runVsphereColdConversion(convert)
	default:
		err = convert.RunVirtV2v(nil)
	}
	if err != nil {
		return err
	}
	// vSphere cold handles inspection and customize inline during conversion.
	if convert.IsInPlace || !env.IsVsphereMigration() {
		return runPostConversionInspectAndCustomize(convert)
	}
	return nil
}

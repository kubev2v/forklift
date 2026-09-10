package main

import (
	"fmt"
	"os"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/conversion"
	"github.com/kubev2v/forklift/pkg/virt-v2v/server"
	utils "github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

func runInPlaceLibvirt(convert *conversion.Conversion) error {
	domainXML, err := convert.GetDomainXML()
	if err != nil {
		return fmt.Errorf("failed to get domain XML: %w", err)
	}
	if err := os.WriteFile(convert.LibvirtDomainFile, []byte(domainXML), 0644); err != nil {
		return fmt.Errorf("failed to write domain XML file: %w", err)
	}
	if convert.OverlayEnabled {
		return convert.RunInPlaceWithOverlay(convert.RunVirtV2vInPlace)
	}
	return convert.RunVirtV2vInPlace()
}

func runInPlaceDisk(convert *conversion.Conversion) error {
	if convert.OverlayEnabled {
		return convert.RunInPlaceWithOverlay(convert.RunVirtV2vInPlaceDisk)
	}
	return convert.RunVirtV2vInPlaceDisk()
}

func runVsphereColdConversion(convert *conversion.Conversion) error {
	if err := convert.RunSourceV2vInspection(); err != nil {
		return fmt.Errorf("source inspection: %w", err)
	}
	inspection, err := utils.GetInspectionV2vFromFile(convert.InspectionOutputFile)
	if err != nil {
		return fmt.Errorf("failed to get inspection file: %w", err)
	}
	return convert.RunVirtV2v(&inspection.OS)
}

func runPostConversionInspectAndCustomize(convert *conversion.Conversion) error {
	if err := convert.RunVirtV2VInspection(); err != nil {
		return fmt.Errorf("failed to inspect the disk: %w", err)
	}
	inspection, err := utils.GetInspectionV2vFromFile(convert.InspectionOutputFile)
	if err != nil {
		return fmt.Errorf("failed to get inspection file: %w", err)
	}
	if err := convert.RunCustomize(inspection.OS); err != nil {
		warningMsg := fmt.Sprintf("VM customization failed: %v. Migration will proceed but customization was not applied successfully.", err)
		fmt.Println("WARNING:", warningMsg)
		server.AddWarning(server.Warning{
			Reason:  "CustomizationFailed",
			Message: warningMsg,
		})
	}
	return nil
}

func startServer(env *config.AppConfig) error {
	return server.Server{AppConfig: env}.Start()
}

func fatalIfErr(err error, msg string) {
	if err != nil {
		fmt.Println(msg, err)
		os.Exit(1)
	}
}

// VirtV2VPrepEnvironment used in the cold migration.
// It creates a links between the downloaded guest image from virt-v2v and mounted PVC.
func linkCertificates(env *config.AppConfig) (err error) {
	if env.IsVsphereMigration() {
		if _, err := os.Stat("/etc/secret/cacert"); err == nil {
			// use the specified certificate
			err = os.Symlink("/etc/secret/cacert", "/opt/ca-bundle.crt")
			if err != nil {
				fmt.Println("Error creating ca cert link ", err)
				os.Exit(1)
			}
		} else {
			// otherwise, keep system pool certificates
			err := os.Symlink("/etc/pki/tls/certs/ca-bundle.crt.bak", "/opt/ca-bundle.crt")
			if err != nil {
				fmt.Println("Error creating ca cert link ", err)
				os.Exit(1)
			}
		}
	}
	return nil
}

func createV2vOutputDir(env *config.AppConfig) (err error) {
	if err = os.MkdirAll(env.Workdir, os.ModePerm); err != nil {
		return fmt.Errorf("error creating directory: %v", err)
	}
	return nil
}

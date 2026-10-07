package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/conversion"
	"github.com/kubev2v/forklift/pkg/virt-v2v/server"
	utils "github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

func runColdConversion(convert *conversion.Conversion) error {
	var err error
	switch convert.Source {
	case config.VSPHERE:
		err = convert.RunSourceV2vInspection()
	case config.OVA, config.HYPERV:
		err = convert.RunInputInspection()
	default:
		return fmt.Errorf("unsupported migration source: %s", convert.Source)
	}
	if err != nil {
		return fmt.Errorf("failed to inspect the guest: %w", err)
	}
	inspection, err := utils.GetInspectionV2vFromFile(convert.InspectionOutputFile)
	if err != nil {
		return fmt.Errorf("failed to get inspection file: %w", err)
	}
	return convert.RunVirtV2v(&inspection.OS)
}

func runInPlaceConversion(convert *conversion.Conversion) error {
	if err := convert.RunVirtV2VInspection(); err != nil {
		return fmt.Errorf("failed to inspect the disk: %w", err)
	}
	inspection, err := utils.GetInspectionV2vFromFile(convert.InspectionOutputFile)
	if err != nil {
		return fmt.Errorf("failed to get inspection file: %w", err)
	}
	return runInPlaceCommand(convert, &inspection.OS)
}

func runInPlaceCommand(convert *conversion.Conversion, osinfo *utils.InspectionOS) error {
	diskMode := false
	if convert.LibvirtUrl != "" {
		domainXML, err := convert.GetDomainXML()
		if err != nil {
			return fmt.Errorf("failed to get domain XML: %w", err)
		}
		if err := os.WriteFile(convert.LibvirtDomainFile, []byte(domainXML), 0644); err != nil {
			return fmt.Errorf("failed to write domain XML file: %w", err)
		}
	} else if err := prepareInPlaceDomainFromConfigMap(convert); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		diskMode = true
	}
	switch {
	case diskMode && convert.OverlayEnabled:
		return convert.RunInPlaceWithOverlay(func() error { return convert.RunVirtV2vInPlaceDisk(osinfo) })
	case diskMode:
		return convert.RunVirtV2vInPlaceDisk(osinfo)
	case convert.OverlayEnabled:
		return convert.RunInPlaceWithOverlay(func() error { return convert.RunVirtV2vInPlace(osinfo) })
	default:
		return convert.RunVirtV2vInPlace(osinfo)
	}
}

func prepareInPlaceDomainFromConfigMap(convert *conversion.Conversion) error {
	fileContents, err := os.ReadFile(convert.LibvirtDomainFile)
	if err != nil {
		return err
	}

	// Disk paths in the domain XML point at raw disk files; rewrite them for overlay symlinks.
	// The ConfigMap-mounted file at LibvirtDomainFile is read-only, so write to a temp path.
	modifiedXML, err := convert.UpdateDiskPaths(string(fileContents))
	if err != nil {
		return fmt.Errorf("failed to update disk paths in domain XML: %w", err)
	}

	f, err := os.CreateTemp("", "domain-*-xml")
	if err != nil {
		return fmt.Errorf("failed to create temp domain XML file: %w", err)
	}
	writablePath := f.Name()
	if _, err := f.WriteString(modifiedXML); err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to write domain XML file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close domain XML file after writing: %w", err)
	}
	convert.LibvirtDomainFile = writablePath
	return nil
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

// linkCertificates links the vSphere CA bundle to the path used by libvirt.
func linkCertificates(env *config.AppConfig) error {
	if env.IsVsphereMigration() {
		if _, err := os.Stat("/etc/secret/cacert"); err == nil {
			// use the specified certificate
			if err := os.Symlink("/etc/secret/cacert", "/opt/ca-bundle.crt"); err != nil {
				return fmt.Errorf("error creating ca cert link: %w", err)
			}
		} else {
			// otherwise, keep system pool certificates
			if err := os.Symlink("/etc/pki/tls/certs/ca-bundle.crt.bak", "/opt/ca-bundle.crt"); err != nil {
				return fmt.Errorf("error creating ca cert link: %w", err)
			}
		}
	}
	return nil
}

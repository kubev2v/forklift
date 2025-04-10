package utils

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"

	libvirtxml "libvirt.org/libvirt-go-xml"
)

type InspectionOS struct {
	Name   string `xml:"name"`
	Distro string `xml:"distro"`
	Osinfo string `xml:"osinfo"`
	Arch   string `xml:"arch"`
}

type InspectionV2V struct {
	OS InspectionOS `xml:"operatingsystem"`
}

func GetInspectionV2vFromFile(xmlFilePath string) (*InspectionV2V, error) {
	xmlData, err := os.ReadFile(xmlFilePath)
	if err != nil {
		fmt.Printf("Error read XML: %v\n", err)
		return nil, err
	}

	var xmlConf InspectionV2V
	err = xml.Unmarshal(xmlData, &xmlConf)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling XML: %v\n", err)
	}
	return &xmlConf, nil
}

func ReadDomainFromFile(domainPath string) (*libvirtxml.Domain, error) {
	domcfg := &libvirtxml.Domain{}
	xmlFile, err := os.Open(domainPath)
	if err != nil {
		return nil, err
	}
	defer xmlFile.Close()
	xmlData, err := io.ReadAll(xmlFile)
	if err != nil {
		return nil, err
	}
	err = domcfg.Unmarshal(string(xmlData))
	if err != nil {
		return nil, err
	}
	return domcfg, nil
}

func WriteDomainToFile(domain *libvirtxml.Domain, domainPath string) error {
	marshal, err := domain.Marshal()
	if err != nil {
		return err
	}
	f, err := os.Create(domainPath)
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(marshal))
	if err != nil {
		return err
	}
	return nil
}

func AddDiskToDomain(domain *libvirtxml.Domain, path string, i int) {
	// This is similar to the libvirtDomain in the kubevirt.go
	// TODO: We should stop using the hd device and use the vmwares device and bus from the libvirt domain
	domain.Devices.Disks = append(domain.Devices.Disks, libvirtxml.DomainDisk{
		Device: "disk",
		Driver: &libvirtxml.DomainDiskDriver{
			Name: "qemu",
			Type: "raw",
		},
		Target: &libvirtxml.DomainDiskTarget{
			Dev: "sd" + string(rune('a'+i)),
			Bus: "scsi",
		},
		Source: &libvirtxml.DomainDiskSource{
			File: &libvirtxml.DomainDiskSourceFile{
				File: path,
			},
		},
	})
}

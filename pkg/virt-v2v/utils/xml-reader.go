package utils

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
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

	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	var root string
	for root == "" {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("error reading XML root: %w", err)
		}
		if start, ok := token.(xml.StartElement); ok {
			root = start.Name.Local
		}
	}
	if root != "operatingsystems" && root != "v2v" {
		return nil, fmt.Errorf("unexpected inspection XML root element: %s", root)
	}

	var inspection InspectionV2V
	if err := xml.Unmarshal(xmlData, &inspection); err != nil {
		return nil, fmt.Errorf("error unmarshalling XML: %w", err)
	}
	if inspection.OS.Name == "" && inspection.OS.Distro == "" && inspection.OS.Osinfo == "" && inspection.OS.Arch == "" {
		return nil, fmt.Errorf("no operating system detected in inspection XML")
	}
	return &inspection, nil
}

func (os InspectionOS) IsWindows() bool {
	return strings.Contains(strings.ToLower(os.Osinfo), "win")
}

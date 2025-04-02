package utils

import (
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

	var xmlConf InspectionV2V
	err = xml.Unmarshal(xmlData, &xmlConf)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling XML: %v\n", err)
	}
	return &xmlConf, nil
}

func (os InspectionOS) IsWindows() bool {
	return strings.Contains(strings.ToLower(os.Osinfo), "win")
}

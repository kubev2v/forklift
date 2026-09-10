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

// virtInspectorXML matches the root element produced by virt-inspector.
type virtInspectorXML struct {
	OS InspectionOS `xml:"operatingsystem"`
}

func GetInspectionV2vFromFile(xmlFilePath string) (*InspectionV2V, error) {
	xmlData, err := os.ReadFile(xmlFilePath)
	if err != nil {
		fmt.Printf("Error read XML: %v\n", err)
		return nil, err
	}

	// virt-inspector writes <operatingsystems><operatingsystem>...</operatingsystem></operatingsystems>
	var inspectorXML virtInspectorXML
	if err := xml.Unmarshal(xmlData, &inspectorXML); err == nil && hasInspectionOS(inspectorXML.OS) {
		return &InspectionV2V{OS: inspectorXML.OS}, nil
	}

	// virt-v2v-inspector writes <v2v><operatingsystem>...</operatingsystem></v2v>
	var v2vXML InspectionV2V
	if err := xml.Unmarshal(xmlData, &v2vXML); err != nil {
		return nil, fmt.Errorf("error unmarshalling XML: %v", err)
	}
	return &v2vXML, nil
}

func hasInspectionOS(os InspectionOS) bool {
	return os.Name != "" || os.Distro != "" || os.Osinfo != "" || os.Arch != ""
}

func (os InspectionOS) IsWindows() bool {
	return strings.Contains(strings.ToLower(os.Osinfo), "win")
}

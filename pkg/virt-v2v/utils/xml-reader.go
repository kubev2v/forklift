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

// virtInspectorXML matches the root element produced by virt-inspector.
type virtInspectorXML struct {
	XMLName xml.Name     `xml:"operatingsystems"`
	OS      InspectionOS `xml:"operatingsystem"`
}

type v2vInspectionXML struct {
	XMLName xml.Name     `xml:"v2v"`
	OS      InspectionOS `xml:"operatingsystem"`
}

func GetInspectionV2vFromFile(xmlFilePath string) (*InspectionV2V, error) {
	xmlData, err := os.ReadFile(xmlFilePath)
	if err != nil {
		fmt.Printf("Error read XML: %v\n", err)
		return nil, err
	}

	root, err := xmlRootElement(xmlData)
	if err != nil {
		return nil, fmt.Errorf("error reading XML root: %w", err)
	}

	switch root {
	case "operatingsystems":
		var inspectorXML virtInspectorXML
		if err := xml.Unmarshal(xmlData, &inspectorXML); err != nil {
			return nil, fmt.Errorf("error unmarshalling XML: %w", err)
		}
		if !hasInspectionOS(inspectorXML.OS) {
			return nil, fmt.Errorf("no operating system detected in inspection XML")
		}
		return &InspectionV2V{OS: inspectorXML.OS}, nil
	case "v2v":
		var v2vXML v2vInspectionXML
		if err := xml.Unmarshal(xmlData, &v2vXML); err != nil {
			return nil, fmt.Errorf("error unmarshalling XML: %w", err)
		}
		if !hasInspectionOS(v2vXML.OS) {
			return nil, fmt.Errorf("no operating system detected in inspection XML")
		}
		return &InspectionV2V{OS: v2vXML.OS}, nil
	default:
		return nil, fmt.Errorf("unexpected inspection XML root element: %s", root)
	}
}

func xmlRootElement(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, nil
		}
	}
}

func hasInspectionOS(os InspectionOS) bool {
	return os.Name != "" || os.Distro != "" || os.Osinfo != "" || os.Arch != ""
}

func (os InspectionOS) IsWindows() bool {
	return strings.Contains(strings.ToLower(os.Osinfo), "win")
}

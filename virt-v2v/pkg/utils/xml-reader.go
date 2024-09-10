package utils

import (
	"encoding/xml"
	"fmt"
	"os"
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
	xmlData, err := ReadXMLFile(xmlFilePath)
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

// ReadXMLFile reads the content of an XML []byte from the given file path.
//
// Arguments:
//   - filePath (string): The path to the XML file.
//
// Returns:
//   - []byte: The content of the XML file.
//   - error: An error if the file cannot be read, or nil if successful.
func ReadXMLFile(filePath string) ([]byte, error) {
	if filePath == "" {
		return nil, fmt.Errorf("XML file path is empty")
	}

	xmlData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading XML file: %w", err)
	}

	return xmlData, nil
}

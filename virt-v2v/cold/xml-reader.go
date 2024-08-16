package main

import (
	"encoding/xml"
	"fmt"
	"os"
)

type OvaVmconfig struct {
	XMLName  xml.Name `xml:"domain"`
	Name     string   `xml:"name"`
	OS       OS       `xml:"os"`
	Metadata Metadata `xml:"metadata"`
}

type OS struct {
	Type   OSType `xml:"type"`
	Loader Loader `xml:"loader"`
	Nvram  Nvram  `xml:"nvram"`
}

type Metadata struct {
	LibOsInfo LibOsInfo `xml:"libosinfo"`
}

type LibOsInfo struct {
	V2VOS V2VOS `xml:"os"`
}

type V2VOS struct {
	ID string `xml:"id,attr"`
}

type OSType struct {
	Arch    string `xml:"arch,attr"`
	Machine string `xml:"machine,attr"`
	Content string `xml:",chardata"`
}

type Loader struct {
	Readonly string `xml:"readonly,attr"`
	Type     string `xml:"type,attr"`
	Secure   string `xml:"secure,attr"`
	Path     string `xml:",chardata"`
}

type Nvram struct {
	Template string `xml:"template,attr"`
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

// GetOperationSystemFromConfig extracts the operating system string from the given XML configuration.
//
// This function takes an XML string that represents a virtual machine's configuration
// and extracts the operating system identifier from it.
//
// Arguments:
//   - xmlData ([]byte): The XML []byte representing the VM configuration.
//
// Returns:
//   - string: The operating system ID extracted from the XML configuration.
//   - error: An error if the xml string cannot be parsed, or nil if successful.
func GetOperationSystemFromConfig(xmlData []byte) (string, error) {
	var xmlConf OvaVmconfig

	err := xml.Unmarshal([]byte(xmlData), &xmlConf)
	if err != nil {
		fmt.Printf("Error unmarshalling XML: %v\n", err)
		return "", err
	}

	operatingSystem := xmlConf.Metadata.LibOsInfo.V2VOS.ID
	if operatingSystem == "" {
		fmt.Println("Error unmarshalling XML: missing OS ID")
		return "", fmt.Errorf("missing OS ID")
	}

	return operatingSystem, nil
}

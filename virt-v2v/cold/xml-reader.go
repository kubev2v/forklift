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
	Devices  Devices  `xml:"devices"`
}

type OS struct {
	Type   OSType `xml:"type"`
	Loader Loader `xml:"loader"`
	Nvram  Nvram  `xml:"nvram"`
}

type Metadata struct {
	LibOsInfo LibOsInfo `xml:"libosinfo"`
}

type Devices struct {
	Disks []Disk `xml:"disk"`
}
type Disk struct {
	Source Source `xml:"source"`
}
type Source struct {
	File string `xml:"file,attr"`
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

func GetDomainFromXml(xmlFilePath string) (*OvaVmconfig, error) {
	xmlData, err := ReadXMLFile(xmlFilePath)
	if err != nil {
		fmt.Printf("Error read XML: %v\n", err)
		return nil, err
	}

	var xmlConf OvaVmconfig
	err = xml.Unmarshal(xmlData, &xmlConf)
	if err != nil {
		fmt.Printf("Error unmarshalling XML: %v\n", err)
		return nil, err
	}
	return &xmlConf, nil
}

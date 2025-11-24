package storage

import (
	"fmt"
)

type Storage struct {
	Vendor         string `yaml:"vendor"`
	VendorProduct  string `yaml:"vendorProduct"`
	ProductVersion string `yaml:"productVersion"`
	ConnectionType string `yaml:"connectionType"`
}

type StorageCredentials struct {
	Hostname      string
	Username      string
	Password      string
	SSLSkipVerify bool
	VendorProduct string
}

func StorageInfo(credentials StorageCredentials) (Storage, error) {
	storage := Storage{}

	switch credentials.VendorProduct {
	case "primera3par":
		i, err := getPrimera3ParSystemInfo(credentials.Hostname, credentials.Username, credentials.Password, credentials.SSLSkipVerify)
		if err != nil {
			return Storage{}, err
		}
		fmt.Printf("Storage system info %v\n", i)
		storage.Vendor = "HP"
		storage.VendorProduct = i.Model
		storage.ProductVersion = i.SystemVersion
	case "ontap":
		i, err := getOntapSystemInfo(credentials.Hostname, credentials.Username, credentials.Password, credentials.SSLSkipVerify)
		if err != nil {
			return Storage{}, err
		}
		fmt.Printf("Storage system info %v\n", i)
		storage.Vendor = "NetApp"
		storage.VendorProduct = i.Name
		storage.ProductVersion = i.Version.Full

	default:
		return storage, fmt.Errorf("storage system into retrieval is unsupported for %s", credentials.VendorProduct)
	}

	return storage, nil
}

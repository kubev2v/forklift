package hypervovf

import "strings"

// OsNameToID maps OS type strings to OVF OS IDs
var OsNameToID = map[string]int{
	"otherGuest":             1,
	"macosGuest":             2,
	"attunixGuest":           3,
	"dguxGuest":              4,
	"windowsxpGuest":         5,
	"windows2000Guest":       6,
	"windows2003Guest":       7,
	"vistaGuest":             8,
	"windows7Guest":          9,
	"windows8Guest":          10,
	"windows81Guest":         11,
	"windows10Guest":         103,
	"windows10_64Guest":      103,
	"windows11Guest":         103,
	"windows11_64Guest":      103,
	"windows7srv_guest":      13,
	"windows7srv_64Guest":    13,
	"windows8srv_guest":      112,
	"windows8srv_64Guest":    112,
	"windows2016srv_guest":   15,
	"windows2016srv_64Guest": 15,
	"windows2019srv_guest":   94,
	"windows2019srv_64Guest": 94,
	"windows2022srv_guest":   17,
	"windows2022srv_64Guest": 17,
	"rhel7Guest":             20,
	"rhel8_64Guest":          21,
	"ubuntuGuest":            22,
	"ubuntu64Guest":          22,
	"centosGuest":            24,
	"centos64Guest":          24,
	"debian10Guest":          26,
	"debian10_64Guest":       26,
	"fedoraGuest":            27,
	"fedora64Guest":          27,
	"slesGuest":              29,
	"sles_64Guest":           30,
	"solaris10Guest":         31,
	"solaris11Guest":         32,
	"freebsd11Guest":         33,
	"freebsd12Guest":         34,
	"oracleLinuxGuest":       35,
	"oracleLinux64Guest":     36,
	"otherLinuxGuest":        101,
	"otherLinux64Guest":      101,
}

// GuestOSInfo contains guest operating system information
type GuestOSInfo struct {
	Caption        string `json:"Caption"`
	Version        string `json:"Version"`
	OSArchitecture string `json:"OSArchitecture"`
}

// MapCaptionToOsType converts a Windows OS caption to OVF OS type string
func MapCaptionToOsType(caption, arch string) string {
	caption = strings.ToLower(caption)
	arch = strings.ToLower(arch)

	switch {
	// === Windows Server ===
	case strings.Contains(caption, "windows server 2022"):
		if arch == "64-bit" {
			return "windows2022srv_64Guest"
		}
		return "windows2022srv_guest"
	case strings.Contains(caption, "windows server 2019"):
		if arch == "64-bit" {
			return "windows2019srv_64Guest"
		}
		return "windows2019srv_guest"
	case strings.Contains(caption, "windows server 2016"):
		if arch == "64-bit" {
			return "windows2016srv_64Guest"
		}
		return "windows2016srv_guest"
	case strings.Contains(caption, "windows server 2012 r2"):
		if arch == "64-bit" {
			return "windows8srv_64Guest"
		}
		return "windows8srv_guest"
	case strings.Contains(caption, "windows server 2012"):
		if arch == "64-bit" {
			return "windows8srv_64Guest"
		}
		return "windows8srv_guest"
	case strings.Contains(caption, "windows server 2008 r2"):
		if arch == "64-bit" {
			return "windows7srv_64Guest"
		}
		return "windows7srv_guest"

	// === Windows Desktop ===
	case strings.Contains(caption, "windows 11"):
		if arch == "64-bit" {
			return "windows11_64Guest"
		}
		return "windows11Guest"
	case strings.Contains(caption, "windows 10"):
		if arch == "64-bit" {
			return "windows10_64Guest"
		}
		return "windows10Guest"
	case strings.Contains(caption, "windows 8.1"):
		if arch == "64-bit" {
			return "windows8_64Guest"
		}
		return "windows8Guest"
	case strings.Contains(caption, "windows 8"):
		if arch == "64-bit" {
			return "windows8_64Guest"
		}
		return "windows8Guest"
	case strings.Contains(caption, "windows 7"):
		if arch == "64-bit" {
			return "windows7_64Guest"
		}
		return "windows7Guest"
	case strings.Contains(caption, "windows vista"):
		if arch == "64-bit" {
			return "vista_64Guest"
		}
		return "vistaGuest"

	// === Linux ===
	case strings.Contains(caption, "ubuntu"):
		if arch == "64-bit" {
			return "ubuntu64Guest"
		}
		return "ubuntuGuest"
	case strings.Contains(caption, "debian"):
		if arch == "64-bit" {
			return "debian10_64Guest"
		}
		return "debian10Guest"
	case strings.Contains(caption, "centos"):
		if arch == "64-bit" {
			return "centos64Guest"
		}
		return "centosGuest"
	case strings.Contains(caption, "red hat enterprise linux") || strings.Contains(caption, "rhel"):
		if arch == "64-bit" {
			return "rhel8_64Guest"
		}
		return "rhel7Guest"
	case strings.Contains(caption, "suse"):
		if arch == "64-bit" {
			return "sles_64Guest"
		}
		return "slesGuest"
	case strings.Contains(caption, "fedora"):
		if arch == "64-bit" {
			return "fedora64Guest"
		}
		return "fedoraGuest"
	case strings.Contains(caption, "oracle linux"):
		if arch == "64-bit" {
			return "oracleLinux64Guest"
		}
		return "oracleLinuxGuest"
	case strings.Contains(caption, "linux"):
		if arch == "64-bit" {
			return "otherLinux64Guest"
		}
		return "otherLinuxGuest"

	default:
		return "otherGuest"
	}
}

// GetOVFOperatingSystemID returns the OVF OS ID for a given OS type string
func GetOVFOperatingSystemID(osType string) int {
	// Exact match
	if id, found := OsNameToID[strings.ToLower(osType)]; found {
		return id
	}

	// Partial match fallback
	key := strings.ToLower(osType)
	for known, id := range OsNameToID {
		if strings.Contains(key, strings.ToLower(known)) {
			return id
		}
	}

	return 1 // Other
}

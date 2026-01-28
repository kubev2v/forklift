package inventory

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubev2v/forklift/cmd/provider-common/ovf"
)

// Provider type constants for logging
const (
	ProviderTypeOVA    = "OVA"
	ProviderTypeHyperV = "HyperV"
)

// ScanForAppliances scans for OVA/OVF appliance files in the given path.
// providerType is used for logging (use ProviderTypeOVA or ProviderTypeHyperV).
// Returns parsed OVF envelopes and their file paths.
func ScanForAppliances(path string, providerType string) (envelopes []ovf.Envelope, ovfPaths []string) {
	ovaFiles, ovfFiles, err := findApplianceFiles(path)
	if err != nil {
		log.Printf("[%s] Error finding appliance files: %v", providerType, err)
		return
	}

	var filesPath []string

	// Process .ova archives (OVA provider only - HyperV has no .ova files, only .ovf + .vhdx)
	for _, ovaFile := range ovaFiles {
		log.Printf("[%s] Processing OVA archive: %s", providerType, ovaFile)

		if !isFileComplete(ovaFile) {
			log.Printf("[%s] Skipping %s: file still being copied", providerType, ovaFile)
			continue
		}

		xmlStruct, err := ovf.ExtractEnvelope(ovaFile)
		if err != nil {
			log.Printf("[%s] Error processing OVA %s: %v", providerType, ovaFile, err)
			continue
		}
		envelopes = append(envelopes, *xmlStruct)
		filesPath = append(filesPath, ovaFile)
	}

	// Process standalone .ovf files (both OVA and HyperV providers)
	for _, ovfFile := range ovfFiles {
		log.Printf("[%s] Processing OVF file: %s", providerType, ovfFile)

		if !isFileComplete(ovfFile) {
			log.Printf("[%s] Skipping %s: file still being copied", providerType, ovfFile)
			continue
		}

		xmlStruct, err := ovf.ReadEnvelope(ovfFile)
		if err != nil {
			if strings.Contains(err.Error(), "still being copied") {
				log.Printf("[%s] Skipping %s: %v", providerType, ovfFile, err)
			} else {
				log.Printf("[%s] Error processing OVF %s: %v", providerType, ovfFile, err)
			}
			continue
		}
		envelopes = append(envelopes, *xmlStruct)
		filesPath = append(filesPath, ovfFile)
	}
	return envelopes, filesPath
}

// findApplianceFiles scans directory for .ova and .ovf files.
// Returns ovaFiles (.ova archives) and ovfFiles (standalone .ovf files).
func findApplianceFiles(directory string) (ovaFiles []string, ovfFiles []string, err error) {
	var maxDepth = 2

	err = filepath.WalkDir(directory, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relativePath, _ := filepath.Rel(directory, path)
		depth := len(strings.Split(relativePath, string(filepath.Separator)))

		switch {
		case (depth <= maxDepth) && isOvaFile(info.Name()):
			ovaFiles = append(ovaFiles, path)

		case (depth <= maxDepth+1) && isOvfFile(info.Name()):
			ovfFiles = append(ovfFiles, path)
		}

		return nil
	})

	if err != nil {
		log.Printf("Error scanning appliance files: %v", err)
		return nil, nil, err
	}
	return
}

func isOvaFile(filename string) bool {
	return hasSuffixIgnoreCase(filename, ovf.ExtOVA)
}

func isOvfFile(filename string) bool {
	return hasSuffixIgnoreCase(filename, ovf.ExtOVF)
}

// Checks if the given file has the desired extension
func hasSuffixIgnoreCase(fileName, suffix string) bool {
	return strings.HasSuffix(strings.ToLower(fileName), strings.ToLower(suffix))
}

// isFileComplete checks that the file was not modified in the last 30s
func isFileComplete(filePath string) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	// Exclude zero-byte files (common placeholder pattern)
	age := time.Since(info.ModTime())
	return age > 30*time.Second && info.Size() > 0
}

func GetDiskPath(path string) string {
	if filepath.Ext(path) != ".ovf" {
		return path
	}

	i := strings.LastIndex(path, "/")
	if i > -1 {
		return path[:i+1]
	}
	return path
}

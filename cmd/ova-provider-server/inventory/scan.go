package inventory

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubev2v/forklift/cmd/ova-provider-server/ova"
)

func ScanForAppliances(path string) (envelopes []ova.Envelope, ovaPaths []string) {
	ovaFiles, ovfFiles, err := findOVAFiles(path)
	if err != nil {
		fmt.Println("Error finding OVA anf OVF files:", err)
		return
	}

	var filesPath []string

	for _, ovaFile := range ovaFiles {
		fmt.Println("Processing OVA file:", ovaFile)

		if !isFileComplete(ovaFile) {
			log.Printf("Skipping %s: file still being copied\n", ovaFile)
			continue
		}

		xmlStruct, err := ova.ExtractEnvelope(ovaFile)
		if err != nil {
			log.Printf("Error processing OVF from OVA %s: %v\n", ovaFile, err)
			continue
		}
		envelopes = append(envelopes, *xmlStruct)
		filesPath = append(filesPath, ovaFile)
	}

	for _, ovfFile := range ovfFiles {
		fmt.Println("Processing OVF file:", ovfFile)

		if !isFileComplete(ovfFile) {
			log.Printf("Skipping %s: file still being copied\n", ovfFile)
			continue
		}

		xmlStruct, err := ova.ReadEnvelope(ovfFile)
		if err != nil {
			if strings.Contains(err.Error(), "still being copied") {
				log.Printf("Skipping %s: %v\n", ovfFile, err)
			} else {
				log.Printf("Error processing OVF %s: %v\n", ovfFile, err)
			}
			continue
		}
		envelopes = append(envelopes, *xmlStruct)
		filesPath = append(filesPath, ovfFile)
	}
	return envelopes, filesPath
}

func findOVAFiles(directory string) (ovaFiles []string, ovfFiles []string, err error) {
	var ovaMaxDepth = 2

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
		case (depth <= ovaMaxDepth) && isOva(info.Name()):
			ovaFiles = append(ovaFiles, path)

		case (depth <= ovaMaxDepth+1) && isOvf(info.Name()):
			ovfFiles = append(ovfFiles, path)
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error scanning OVA and OVF files:  ", err)
		return nil, nil, err
	}
	return
}

func isOva(filename string) bool {
	return hasSuffixIgnoreCase(filename, ova.ExtOVA)
}

func isOvf(filename string) bool {
	return hasSuffixIgnoreCase(filename, ova.ExtOVF)
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

func getDiskPath(path string) string {
	if filepath.Ext(path) != ".ovf" {
		return path
	}

	i := strings.LastIndex(path, "/")
	if i > -1 {
		return path[:i+1]
	}
	return path
}

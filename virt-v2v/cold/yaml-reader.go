package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type VmConfig struct {
	OSInfo    string
	DiskPaths []string
}

// ReadYAMLFile reads the YAML file and extracts the HostDisk paths and osinfo label.
func GetVmConfigYaml(filePath string) (*VmConfig, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening YAML file: %w", err)
	}
	defer file.Close()

	config := &VmConfig{}
	scanner := bufio.NewScanner(file)
	var inLabels, inVolumes bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "labels:") {
			inLabels = true
			continue
		}

		if inLabels {
			if strings.HasPrefix(line, "libguestfs.org/osinfo:") {
				config.OSInfo = strings.TrimSpace(strings.TrimPrefix(line, "libguestfs.org/osinfo:"))
			}
			if !strings.HasPrefix(line, "libguestfs.org/") {
				inLabels = false
			}
		}

		if strings.Contains(line, "volumes:") {
			inVolumes = true
			continue
		}
		if inVolumes {
			if strings.Contains(line, "hostDisk:") {
				scanner.Scan()
				pathLine := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(pathLine, "path:") {
					pathValue := strings.TrimSpace(strings.TrimPrefix(pathLine, "path:"))
					if pathValue != "" {
						config.DiskPaths = append(config.DiskPaths, pathValue)
					}
				}
			}
			if strings.Contains(line, "- name:") {
				inVolumes = false
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading YAML file: %w", err)
	}

	return config, nil
}

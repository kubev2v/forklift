/*
Copyright 2026 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubev2v/vm-migration-detective/pkg/vmdetect"
	"github.com/sirupsen/logrus"
)

const (
	// secretDir is where the connection Secret is mounted in the deep-inspection pod.
	secretDir = "/etc/secret"
)

func main() {
	// Initialize logger
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	log.SetOutput(os.Stdout)

	creds, err := loadProviderCredentials()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create detector
	detector, err := vmdetect.NewDetector(vmdetect.DetectorConfig{
		Credentials: creds,
		VDDKLibDir:  "/opt/vmware-vix-disklib-distrib",
		Logger:      log,
		DB:          nil,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	vmMoref, snapshotMoref, err := vmAndSnapshotFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Run detection
	result, err := detector.Detect(vmdetect.DetectParams{
		Ctx:           context.Background(),
		VMMoref:       vmMoref,
		SnapshotMoref: snapshotMoref,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	prettyJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling result: %v\n", err)
		return
	}

	fmt.Print(string(prettyJSON))

	// Check results
	if !result.Passed {
		for _, concern := range result.AllConcerns {
			fmt.Printf("[%s] %s: %s\n", concern.Category, concern.Label, concern.Message)
		}
	}

	os.Exit(0)
}

func loadProviderCredentials() (vmdetect.Credentials, error) {
	url, err := readSecretDataFile("url")
	if err != nil {
		return vmdetect.Credentials{}, err
	}
	user, err := readSecretDataFile("user")
	if err != nil {
		return vmdetect.Credentials{}, err
	}
	password, err := readSecretDataFile("password")
	if err != nil {
		return vmdetect.Credentials{}, err
	}
	return vmdetect.Credentials{
		VCenterURL: url,
		Username:   user,
		Password:   password,
	}, nil
}

func readSecretDataFile(basename string) (string, error) {
	p := filepath.Join(secretDir, basename)
	data, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("read provider credential file %q: %w", p, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func vmAndSnapshotFromEnv() (vmMoref, snapshotMoref string, err error) {
	// VM_ID is set on deep-inspection pods built by the conversion controller.
	vmMoref = strings.TrimSpace(os.Getenv("VM_MOREF"))
	snapshotMoref = strings.TrimSpace(os.Getenv("SNAPSHOT_MOREF"))
	if vmMoref == "" || snapshotMoref == "" {
		return "", "", fmt.Errorf("VM_MOREF and SNAPSHOT_MOREF must be set to VMware managed object references")
	}
	return vmMoref, snapshotMoref, nil
}

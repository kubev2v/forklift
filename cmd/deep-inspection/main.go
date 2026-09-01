/*
Copyright 2026 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubev2v/vm-migration-detective/pkg/vmdetect"
	"github.com/sirupsen/logrus"
)

const (
	secretDir = "/etc/secret"
)

func main() {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	log.SetOutput(os.Stdout)

	srv := newResultServer()

	source := strings.TrimSpace(os.Getenv("V2V_SOURCE"))

	go func() {
		var result *vmdetect.DetectResult
		var detectErr error

		switch source {
		case "hyperv":
			result, detectErr = detectLocalDisks(log)
		default:
			result, detectErr = detectVSphere(log)
		}
		srv.setResult(result, detectErr)
	}()

	os.Exit(srv.run())
}

func detectVSphere(log *logrus.Logger) (*vmdetect.DetectResult, error) {
	creds, err := loadProviderCredentials()
	if err != nil {
		return nil, err
	}

	detector, err := vmdetect.NewDetector(vmdetect.DetectorConfig{
		Credentials: creds,
		VDDKLibDir:  "/opt/vmware-vix-disklib-distrib",
		Logger:      log,
		DB:          nil,
	})
	if err != nil {
		return nil, err
	}

	vmMoref, snapshotMoref, err := vmAndSnapshotFromEnv()
	if err != nil {
		return nil, err
	}

	return detector.Detect(vmdetect.DetectParams{
		Ctx:           context.Background(),
		VMMoref:       vmMoref,
		SnapshotMoref: snapshotMoref,
	})
}

// detectLocalDisks runs virt-inspector directly on locally-mounted disk files
// (Hyper-V VHD/VHDX over SMB mount at /hyperv).
func detectLocalDisks(log *logrus.Logger) (*vmdetect.DetectResult, error) {
	diskPath := strings.TrimSpace(os.Getenv("V2V_DISK_PATH"))
	if diskPath == "" {
		return nil, fmt.Errorf("V2V_DISK_PATH must be set for hyperv deep inspection")
	}

	format := "vhdx"
	if strings.HasSuffix(strings.ToLower(diskPath), ".vhd") {
		format = "vpc"
	}

	log.Infof("Running local deep inspection on %s (format=%s)", diskPath, format)

	detector, err := vmdetect.NewDetector(vmdetect.DetectorConfig{
		Logger: log,
		DB:     nil,
	})
	if err != nil {
		return nil, err
	}

	return detector.DetectLocal(vmdetect.DetectLocalParams{
		Ctx:       context.Background(),
		DiskPaths: []string{diskPath},
		Formats:   []string{format},
	})
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
	vmMoref = strings.TrimSpace(os.Getenv("VM_MOREF"))
	snapshotMoref = strings.TrimSpace(os.Getenv("SNAPSHOT_MOREF"))
	if vmMoref == "" || snapshotMoref == "" {
		return "", "", fmt.Errorf("VM_MOREF and SNAPSHOT_MOREF must be set to VMware managed object references")
	}
	return vmMoref, snapshotMoref, nil
}

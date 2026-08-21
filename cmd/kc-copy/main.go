// kc-copy runs standalone vSphere NFC disk copy.
// Copied from github.com/yaacov/kc-utils v0.1.2 (cmd/kc-copy/main.go).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/yaacov/kc-utils/pkg/common/logger"
	kccopy "github.com/yaacov/kc-utils/pkg/copy"
)

func main() {
	inputFile := flag.String("input", "", "input JSON file (CopyInput)")
	outputFile := flag.String("output", "copy-progress.json", "output JSON file")
	host := flag.String("host", "", "vCenter/ESXi hostname (e.g. vcenter.example.com)")
	datacenter := flag.String("datacenter", "", "vSphere datacenter name")
	insecure := flag.Bool("insecure", false, "skip TLS certificate verification")
	caCert := flag.String("ca-cert", "", "PEM CA cert path for secure TLS")
	vmName := flag.String("vm-name", "", "VM name")
	fingerprint := flag.String("fingerprint", "", "vCenter SSL thumbprint")
	diskPath := flag.String("disk-path", "", "comma-separated source vmdk paths to copy (omit to copy all disks)")
	targetDir := flag.String("target-dir", "", "write diskN.img files here instead of PVC targets")
	workdir := flag.String("work-dir", kccopy.DefaultWorkdir, "working directory")
	copyConcurrency := flag.Int("copy-concurrency", kccopy.DefaultCopyConcurrency, "max parallel disk copies")
	logLevel := flag.String("log-level", "info", "log level (debug, info, warn, error)")
	flag.Parse()

	logger.Init(*logLevel)

	input, err := loadInput(*inputFile, *host, *datacenter, *insecure, *caCert, *vmName, *fingerprint, *diskPath, *targetDir, *workdir, *outputFile, *copyConcurrency)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(input.Workdir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "failed to create workdir:", err)
		os.Exit(1)
	}

	if err := kccopy.Run(&input); err != nil {
		fmt.Fprintln(os.Stderr, "copy failed:", err)
		os.Exit(1)
	}
}

func loadInput(inputFile, host, datacenter string, insecure bool, caCert, vmName, fingerprint, diskPath, targetDir, workdir, outputPath string, copyConcurrency int) (kccopy.CopyInput, error) {
	if inputFile != "" {
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return kccopy.CopyInput{}, fmt.Errorf("read input: %w", err)
		}
		var input kccopy.CopyInput
		if err := json.Unmarshal(data, &input); err != nil {
			return kccopy.CopyInput{}, fmt.Errorf("parse input JSON: %w", err)
		}
		if input.Workdir == "" {
			input.Workdir = workdir
		}
		if input.CopyConcurrency == 0 {
			input.CopyConcurrency = copyConcurrency
		}
		if input.OutputPath == "" {
			input.OutputPath = outputPath
		}
		if input.TargetDir == "" {
			input.TargetDir = targetDir
		}
		if err := validateInput(&input); err != nil {
			return kccopy.CopyInput{}, err
		}
		return input, nil
	}

	input := kccopy.CopyInput{
		Host:            host,
		Datacenter:      datacenter,
		Insecure:        insecure,
		CaCert:          caCert,
		VMName:          vmName,
		Fingerprint:     fingerprint,
		SourceDisks:     kccopy.SplitDiskPath(diskPath),
		TargetDir:       targetDir,
		Workdir:         workdir,
		OutputPath:      outputPath,
		CopyConcurrency: copyConcurrency,
	}
	if err := validateInput(&input); err != nil {
		return kccopy.CopyInput{}, err
	}
	return input, nil
}

func validateInput(input *kccopy.CopyInput) error {
	if input.Host == "" {
		return fmt.Errorf("--host is required (or use --input)")
	}
	if input.VMName == "" {
		return fmt.Errorf("--vm-name is required (or use --input)")
	}
	if input.Fingerprint == "" {
		return fmt.Errorf("--fingerprint is required (or use --input)")
	}
	return nil
}

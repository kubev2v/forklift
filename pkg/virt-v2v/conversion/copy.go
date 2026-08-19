package conversion

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	kccopy "github.com/yaacov/kc-utils/pkg/copy"
)

const (
	kcCopyBin         = "/usr/local/bin/kc-copy"
	copyInputFileName = "copy-input.json"
	copyOutputFile    = "copy-progress.json"
	kcCopyCaCert      = "/etc/secret/cacert"
)

// Convert runs guest conversion.
// InPlace: virt-v2v-in-place only.
// vSphere (local or remote): kc-copy then virt-v2v-in-place.
// OVA / HyperV: virt-v2v (copy+convert).
func (c *Conversion) Convert() error {
	if c.IsInPlace {
		return c.RunInPlaceConversion()
	}
	if !c.IsVsphereMigration() {
		return c.RunVirtV2v()
	}
	if err := c.RunKcCopy(); err != nil {
		return err
	}
	return c.RunInPlaceConversion()
}

// RunKcCopy writes copy-input.json from the conversion env and execs kc-copy.
func (c *Conversion) RunKcCopy() error {
	if c.LibvirtUrl == "" {
		return fmt.Errorf("kc-copy requires %s", config.EnvLibvirtUrlName)
	}
	if c.VmName == "" {
		return fmt.Errorf("kc-copy requires %s", config.EnvVmNameName)
	}
	if c.Fingerprint == "" {
		return fmt.Errorf("kc-copy requires %s", config.EnvFingerprintName)
	}

	caCert := ""
	if _, err := c.fileSystem.Stat(kcCopyCaCert); err == nil {
		caCert = kcCopyCaCert
	}

	input, err := BuildCopyInput(c.AppConfig, kccopy.SplitDiskPath(c.DiskPath), caCert)
	if err != nil {
		return fmt.Errorf("parse libvirt URL: %w", err)
	}
	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal kc-copy input: %w", err)
	}

	inputPath := filepath.Join(input.Workdir, copyInputFileName)
	outputPath := filepath.Join(input.Workdir, copyOutputFile)
	if err := c.fileSystem.WriteFile(inputPath, data, 0644); err != nil {
		return fmt.Errorf("write kc-copy input: %w", err)
	}

	cmd := c.CommandBuilder.New(kcCopyBin).
		AddArg("--input", inputPath).
		AddArg("--output", outputPath).
		Build()
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kc-copy: %w", err)
	}
	return nil
}

// RunInPlaceConversion runs virt-v2v-in-place on already populated disks.
// With a libvirt URL it rewrites domain XML to local disk links; otherwise it
// uses -i disk (e.g. EC2).
func (c *Conversion) RunInPlaceConversion() error {
	if c.LibvirtUrl != "" {
		domainXML, err := c.GetDomainXML()
		if err != nil {
			return fmt.Errorf("failed to get domain XML: %v", err)
		}
		if err := c.fileSystem.WriteFile(c.LibvirtDomainFile, []byte(domainXML), 0644); err != nil {
			return fmt.Errorf("failed to write domain XML file: %v", err)
		}
		if c.OverlayEnabled {
			return c.RunInPlaceWithOverlay(c.RunVirtV2vInPlace)
		}
		return c.RunVirtV2vInPlace()
	}
	if c.OverlayEnabled {
		return c.RunInPlaceWithOverlay(c.RunVirtV2vInPlaceDisk)
	}
	return c.RunVirtV2vInPlaceDisk()
}

// BuildCopyInput maps Forklift V2V_* env to kc-copy JSON.
// Empty sourceDisks is marshaled as source_disks:null; kc-copy copies every NFC disk.
// caCert is /etc/secret/cacert when that file is mounted, otherwise empty.
func BuildCopyInput(cfg *config.AppConfig, sourceDisks []string, caCert string) (*kccopy.CopyInput, error) {
	host, datacenter, insecure, err := parseLibvirtURL(cfg.LibvirtUrl)
	if err != nil {
		return nil, err
	}
	in := &kccopy.CopyInput{
		Host:        host,
		Datacenter:  datacenter,
		Insecure:    insecure,
		VMName:      cfg.VmName,
		Fingerprint: cfg.Fingerprint,
		SourceDisks: sourceDisks,
		Workdir:     cfg.Workdir,
	}
	if in.Workdir == "" {
		in.Workdir = config.V2vOutputDir
	}
	if caCert != "" {
		in.CaCert = caCert
	}
	return in, nil
}

func parseLibvirtURL(libvirtURL string) (host, datacenter string, insecure bool, err error) {
	u, err := url.Parse(libvirtURL)
	if err != nil {
		return "", "", false, err
	}
	host = u.Hostname()
	if p := u.Port(); p != "" {
		host = net.JoinHostPort(host, p)
	}
	insecure = insecureFromQuery(u.RawQuery)
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) > 0 && parts[0] != "" {
		datacenter = parts[0]
	}
	return host, datacenter, insecure, nil
}

func insecureFromQuery(rawQuery string) bool {
	if rawQuery == "" {
		return false
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return false
	}
	return values.Get("no_verify") == "1"
}

package inspection

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kubev2v/vm-migration-detective/internal/cmdbuilder"
	"github.com/kubev2v/vm-migration-detective/internal/vddk"
	"github.com/kubev2v/vm-migration-detective/pkg/types"
	"github.com/sirupsen/logrus"
)

// VirtV2vInspector handles VM inspection operations using virt-v2v-inspector
type VirtV2vInspector struct {
	virtV2vInspectorPath string
	timeout              time.Duration
	logger               *logrus.Logger
}

// NewVirtV2vInspector creates a new VirtV2vInspector instance
func NewVirtV2vInspector(virtV2vInspectorPath string, timeout time.Duration, logger *logrus.Logger) *VirtV2vInspector {
	if virtV2vInspectorPath == "" {
		virtV2vInspectorPath = "virt-v2v-inspector" // Use system PATH
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &VirtV2vInspector{
		virtV2vInspectorPath: virtV2vInspectorPath,
		timeout:              timeout,
		logger:               logger,
	}
}

// Inspect uses virt-v2v-inspector to inspect a VM snapshot directly via VDDK
func (i *VirtV2vInspector) Inspect(
	ctx context.Context,
	vmMoref string,
	snapshotMoref string,
	vcenterURL string,
	username string,
	password string,
	diskInfo *types.SnapshotDiskInfo, // Snapshot disk info from vm_service
	sslVerify string, // SSL verification option for vpx:// URL (e.g., "no_verify=1" or "cacert=/path/to/ca-bundle.crt")
) (*types.VirtV2VInspectorXML, error) {
	i.logger.WithFields(logrus.Fields{
		"vm_moref":       vmMoref,
		"snapshot_moref": snapshotMoref,
		"vcenter_url":    vcenterURL,
	}).Info("Running virt-v2v-inspector on snapshot")

	// Build libvirt connection URL for vSphere
	// Format: vpx://username@vcenter/compute-resource-path?ssl-verify
	// The path must point to a compute resource (host/cluster), not the datacenter or VM
	// The VM name is specified as a positional argument after "--"
	// Username is in URL (needed by virt-v2v-inspector to pass to VDDK)
	// Password is provided via -ip file (secure)
	// Extract hostname from vCenter URL
	vcenterHost := extractHostname(vcenterURL)

	// URL-encode username to handle special characters like @
	// The @ symbol in the username needs to be percent-encoded as %40
	// because @ is used as a delimiter between username and hostname in URLs
	encodedUsername := url.QueryEscape(username)

	// Use the compute resource path from diskInfo (e.g., "/Datacenter/Cluster/host.example.com")
	// This is required for vpx:// URLs - they need a compute resource, not just a datacenter
	computeResourcePath := diskInfo.ComputeResourcePath
	if computeResourcePath == "" {
		return nil, fmt.Errorf("compute resource path is required for vpx:// URL")
	}

	// Build vpx:// URL with username
	// virt-v2v-inspector extracts the username from this URL to pass to VDDK internally
	// Password is kept secure in separate file via -ip parameter
	// Add SSL verification parameter (provided by caller)
	libvirtURL := fmt.Sprintf("vpx://%s@%s%s?%s",
		encodedUsername, vcenterHost, computeResourcePath, sslVerify)

	// Create context with timeout
	inspectCtx, cancel := context.WithTimeout(ctx, i.timeout)
	defer cancel()

	// Create a password file for VDDK authentication
	// VDDK uses -io vddk-password=+file to read password securely
	passwordFile, err := i.createPasswordFile(password)
	if err != nil {
		return nil, fmt.Errorf("failed to create password file: %w", err)
	}
	defer func() { _ = os.Remove(passwordFile) }()

	// Strip VDDK paths from LD_LIBRARY_PATH so libguestfs/supermin doesn't pick them up.
	thumbprint, err := getVCenterThumbprint(vcenterHost)
	if err != nil {
		i.logger.WithError(err).Warn("Failed to get thumbprint, proceeding without SSL verification")
	}
	vddkLibDir := vddk.GetLibDir()
	vddkLibPath := vddk.GetLibPath()

	diskUnlock := resolveDiskUnlock(i.logger)

	cmdArgs := cmdbuilder.New().
		WithLogger(i.logger).
		FilterEnv("LD_LIBRARY_PATH", func(val string) string {
			var kept []string
			for _, p := range strings.Split(val, ":") {
				if p != vddkLibPath && !strings.Contains(p, "vmware-vix-disklib") {
					kept = append(kept, p)
				}
			}
			return strings.Join(kept, ":")
		}).
		SetEnv("LIBGUESTFS_DEBUG", "1").
		Add("-v", "-x").
		Flag("-i", "libvirt").
		Flag("-ic", libvirtURL).
		Flag("-ip", passwordFile).
		Flag("-it", "vddk").
		FlagIf(thumbprint != "", "-io", fmt.Sprintf("vddk-thumbprint=%s", thumbprint)).
		FlagIf(vddkLibDir != "", "-io", fmt.Sprintf("vddk-libdir=%s", vddkLibDir))

	for _, baseDiskPath := range diskInfo.BaseDiskPaths {
		if baseDiskPath != "" {
			cmdArgs.Flag("-io", fmt.Sprintf("vddk-file=%s", baseDiskPath))
		}
	}

	if args := diskUnlock.Args(); len(args) > 0 {
		cmdArgs.Add(args...)
	}

	cmdArgs.Add("--", vmMoref)

	output, err := cmdArgs.RunCombined(inspectCtx, i.virtV2vInspectorPath)
	if inspectCtx.Err() != nil {
		if inspectCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("virt-v2v-inspector command timed out after %v", i.timeout)
		}
		return nil, fmt.Errorf("virt-v2v-inspector command was cancelled: %w", inspectCtx.Err())
	}

	outputStr := string(output)
	if err != nil {
		exitCode := cmdbuilder.ExitCode(err)

		// Check if this is likely an encrypted disk error
		if encrypted, reason := isEncryptedDiskError(outputStr); encrypted {
			i.logger.WithFields(logrus.Fields{
				"output":          outputStr,
				"exit_code":       exitCode,
				"executable":      i.virtV2vInspectorPath,
				"args":            cmdArgs.MaskedArgs(),
				"matched_pattern": reason,
			}).Error("virt-v2v-inspector failed - disk appears to be encrypted")

			switch diskUnlock.method {
			case unlockClevis:
				return nil, fmt.Errorf("disk encryption detected: virt-v2v-inspector could not unlock disk using clevis/NBDE. Exit code: %d", exitCode)
			case unlockKeyFiles:
				return nil, fmt.Errorf("disk encryption detected: virt-v2v-inspector could not unlock disk using %d LUKS key file(s) from %s. Exit code: %d", len(diskUnlock.keys), defaultLUKSKeyDir, exitCode)
			default:
				return nil, fmt.Errorf("disk encryption detected: virt-v2v-inspector cannot access encrypted disks. The VM disk appears to be encrypted and cannot be inspected without decryption. Exit code: %d", exitCode)
			}
		}

		i.logger.WithFields(logrus.Fields{
			"output":     outputStr,
			"exit_code":  exitCode,
			"executable": i.virtV2vInspectorPath,
			"args":       cmdArgs.MaskedArgs(),
		}).Error("virt-v2v-inspector failed")

		// Include output in error message for better debugging
		if outputStr != "" {
			return nil, fmt.Errorf("virt-v2v-inspector failed (exit code %d): %w\nOutput: %s", exitCode, err, outputStr)
		}
		return nil, fmt.Errorf("virt-v2v-inspector failed (exit code %d): %w", exitCode, err)
	}

	// Extract XML from output (virt-v2v-inspector with -v -x may output debug messages)
	// Look for XML content - it should start with <?xml or <v2v-inspection>
	xmlStart := strings.Index(outputStr, "<?xml")
	if xmlStart == -1 {
		xmlStart = strings.Index(outputStr, "<v2v-inspection")
	}
	if xmlStart == -1 {
		xmlStart = strings.Index(outputStr, "<operatingsystem")
	}
	if xmlStart == -1 {
		xmlStart = strings.Index(outputStr, "<inspection")
	}

	var xmlData []byte
	if xmlStart >= 0 {
		// Extract XML portion from output
		xmlData = []byte(outputStr[xmlStart:])
		// Find the end of XML (look for closing </v2v-inspection> tag first, then fallback to </operatingsystem>)
		xmlEnd := strings.LastIndex(string(xmlData), "</v2v-inspection>")
		if xmlEnd > 0 {
			xmlEnd += len("</v2v-inspection>")
			xmlData = xmlData[:xmlEnd]
		} else {
			// Fallback: look for </operatingsystem> if </v2v-inspection> not found
			xmlEnd = strings.LastIndex(string(xmlData), "</operatingsystem>")
			if xmlEnd > 0 {
				xmlEnd += len("</operatingsystem>")
				xmlData = xmlData[:xmlEnd]
			}
		}
		if i.logger != nil {
			xmlPreview := string(xmlData)
			if len(xmlPreview) > 1000 {
				xmlPreview = xmlPreview[:1000] + "... (truncated)"
			}
			i.logger.WithField("xml_extracted", xmlPreview).Debug("Extracted XML from output")
		}
	} else {
		// No XML found, try parsing the whole output
		xmlData = output
		if i.logger != nil {
			i.logger.Warn("No XML markers found in output, attempting to parse entire output")
		}
	}

	inspectionData, err := parseV2VInspectionXML(xmlData)
	if err != nil {
		if i.logger != nil {
			i.logger.WithFields(logrus.Fields{
				"error":  err,
				"output": outputStr,
			}).Error("Failed to parse virt-v2v-inspector XML output")
		}
		return nil, fmt.Errorf("failed to parse virt-v2v-inspector output: %w", err)
	}

	i.logger.Info("virt-v2v-inspector snapshot inspection completed successfully")
	return inspectionData, nil
}

// extractHostname extracts hostname from a URL
func extractHostname(urlStr string) string {
	if urlStr == "" {
		return ""
	}

	// Try parsing as URL
	parsedURL, err := url.Parse(urlStr)
	if err == nil && parsedURL.Hostname() != "" {
		return parsedURL.Hostname()
	}

	// If parsing fails, assume it's already a hostname
	return urlStr
}

// createPasswordFile creates a temporary file with the password
// virt-v2v-inspector expects -ip to be a file path, not the password directly
func (i *VirtV2vInspector) createPasswordFile(password string) (string, error) {
	tmpFile, err := os.CreateTemp("", "v2v-password-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary password file: %w", err)
	}

	// Write password to file
	if _, err := tmpFile.WriteString(password); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write password to file: %w", err)
	}

	// Close the file
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to close password file: %w", err)
	}

	// Set restrictive permissions (read-only for owner)
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to set password file permissions: %w", err)
	}

	return tmpFile.Name(), nil
}

// parseV2VInspectionXML parses virt-v2v-inspector XML output and returns the native XML structure
func parseV2VInspectionXML(xmlData []byte) (*types.VirtV2VInspectorXML, error) {
	var xmlRoot types.VirtV2VInspectorXML
	err := xml.Unmarshal(xmlData, &xmlRoot)
	if err != nil {
		return nil, fmt.Errorf("XML parsing error: %w", err)
	}

	return &xmlRoot, nil
}

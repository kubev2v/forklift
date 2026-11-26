package inspection

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/kubev2v/vm-migration-detective/internal/cmdbuilder"
	"github.com/kubev2v/vm-migration-detective/internal/tlsconfig"
	"github.com/kubev2v/vm-migration-detective/internal/vddk"
	"github.com/kubev2v/vm-migration-detective/internal/vsphere"
	"github.com/kubev2v/vm-migration-detective/pkg/types"
	"github.com/sirupsen/logrus"
)

// virtV2vProgressLine matches virt-v2v-inspector's own high-level phase markers,
// e.g. "[   0.0] Setting up the source: ..." — while excluding guest kernel
// boot log lines that use the same bracket style with microsecond precision.
var virtV2vProgressLine = regexp.MustCompile(`^\[\s*\d+\.\d\]`)

// shouldLogVirtV2vLine also preserves supermin diagnostics in the live log.
func shouldLogVirtV2vLine(line string) bool {
	return virtV2vProgressLine.MatchString(line) || strings.Contains(strings.ToLower(line), "supermin:")
}

// VirtV2vInspector handles VM inspection operations using virt-v2v-inspector
type VirtV2vInspector struct {
	virtV2vInspectorPath string
	logger               *logrus.Logger
}

// NewVirtV2vInspector creates a new VirtV2vInspector instance. The process runs
// until its caller's context is cancelled.
func NewVirtV2vInspector(virtV2vInspectorPath string, logger *logrus.Logger) *VirtV2vInspector {
	if virtV2vInspectorPath == "" {
		virtV2vInspectorPath = "virt-v2v-inspector" // Use system PATH
	}
	return &VirtV2vInspector{
		virtV2vInspectorPath: virtV2vInspectorPath,
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
	tlsConfig *tlsconfig.Config,
	diskInfo *types.SnapshotDiskInfo, // Snapshot disk info from vm_service
) (*types.VirtV2VInspectorXML, error) {
	if tlsConfig == nil {
		return nil, fmt.Errorf("TLS configuration is required")
	}
	sslVerify := tlsConfig.ForVirtV2V(tlsConfig.RootCAPath)

	i.logger.WithFields(logrus.Fields{
		"vm_moref":       vmMoref,
		"snapshot_moref": snapshotMoref,
		"vcenter_url":    vcenterURL,
	}).Info("Running virt-v2v-inspector on snapshot")

	if diskInfo.VMName == "" {
		return nil, fmt.Errorf("VM name is required for virt-v2v-inspector (vmMoref %s has no name)", vmMoref)
	}

	vcenterHost := extractHostname(vcenterURL)
	vddkLibDir := vddk.GetLibDir()
	useVDDK := false
	if info, err := os.Stat(vddkLibDir); err == nil && info.IsDir() {
		useVDDK = true
	}

	var vsphereClient *vsphere.Client
	if !useVDDK {
		client, err := vsphere.NewClient(ctx, vcenterURL, username, password, tlsConfig, i.logger)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to vSphere for NFC disk inputs: %w", err)
		}
		vsphereClient = client
		defer vsphereClient.Close()
	}

	cmdArgs := cmdbuilder.New().
		WithLogger(i.logger).
		FilterEnv("LD_LIBRARY_PATH", func(val string) string {
			var kept []string
			for _, p := range strings.Split(val, ":") {
				if p != vddk.GetLibPath() && !strings.Contains(p, "vmware-vix-disklib") {
					kept = append(kept, p)
				}
			}
			return strings.Join(kept, ":")
		}).
		SetEnv("LIBGUESTFS_DEBUG", "1").
		Add("-v", "-x")

	baseDiskPaths, err := resolveBaseDiskPaths(diskInfo.BaseDiskPaths, func() ([]string, error) {
		if vsphereClient != nil {
			return vsphereClient.GetBaseDiskPaths(ctx, vmMoref)
		}
		return queryBaseDiskPathsFromVSphere(ctx, vcenterURL, username, password, vmMoref, tlsConfig, i.logger)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to determine base disk paths for virt-v2v-inspector: %w", err)
	}

	if useVDDK {
		// Preserve the native vpx/libvirt input path when VDDK is installed.
		computeResourcePath := diskInfo.ComputeResourcePath
		if computeResourcePath == "" {
			return nil, fmt.Errorf("compute resource path is required for vpx:// URL")
		}
		encodedUsername := url.QueryEscape(username)
		libvirtURL := fmt.Sprintf("vpx://%s@%s%s?%s",
			encodedUsername, bracketIPv6(vcenterHost), computeResourcePath, sslVerify)

		passwordFile, err := i.createPasswordFile(password)
		if err != nil {
			return nil, fmt.Errorf("failed to create password file: %w", err)
		}
		defer func() { _ = os.Remove(passwordFile) }()

		thumbprint := tlsConfig.ForNBDKit()
		if thumbprint == "" && !tlsConfig.Insecure {
			thumbprint, err = tlsconfig.GetVCenterThumbprint(vcenterHost, tlsConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to get vCenter certificate thumbprint: %w", err)
			}
		}

		cmdArgs.Flag("-i", "libvirt").
			Flag("-ic", libvirtURL).
			Flag("-ip", passwordFile).
			Flag("-it", "vddk").
			FlagIf(thumbprint != "", "-io", fmt.Sprintf("vddk-thumbprint=%s", thumbprint)).
			FlagIf(vddkLibDir != "", "-io", fmt.Sprintf("vddk-libdir=%s", vddkLibDir))
		cmdArgs.Add("--no-selinux-relabel", "--no-fstrim")

		for _, baseDiskPath := range baseDiskPaths {
			if baseDiskPath != "" {
				cmdArgs.Flag("-io", fmt.Sprintf("vddk-file=%s", baseDiskPath))
			}
		}

		// libvirt's vpx:// driver looks up domains by VM display name, not moref.
		cmdArgs.Add("--", diskInfo.VMName)
	} else {
		// The installed virt-v2v-inspector rejects "-it nfc". Expose each NFC
		// session as a raw NBD disk instead, which its disk input supports.
		i.logger.WithFields(logrus.Fields{
			"vm_moref":       vmMoref,
			"snapshot_moref": snapshotMoref,
			"disk_count":     len(baseDiskPaths),
		}).Info("Opening NFC NBD inputs for virt-v2v-inspector")

		cmdArgs.Add("-i", "disk", "-if", "raw")
		for _, baseDiskPath := range baseDiskPaths {
			if baseDiskPath == "" {
				continue
			}

			session, err := openWithNBDKit(ctx, vmMoref, snapshotMoref, baseDiskPath,
				vcenterURL, username, password, tlsConfig, vsphereClient, i.logger)
			if err != nil {
				return nil, fmt.Errorf("failed to open NFC NBD input for disk %q: %w", baseDiskPath, err)
			}
			defer session.Close()

			if err := session.WaitForReady(30 * time.Second); err != nil {
				return nil, fmt.Errorf("NFC NBD input for disk %q did not become ready: %w", baseDiskPath, err)
			}
			cmdArgs.Add(session.NBDURL)
		}
		cmdArgs.Add("--no-selinux-relabel", "--no-fstrim")
	}

	diskUnlock := resolveDiskUnlock(i.logger)
	if args := diskUnlock.Args(); len(args) > 0 {
		cmdArgs.Add(args...)
	}

	// XML goes to stdout, debug/error output to stderr. Stderr is streamed
	// line-by-line so phase markers appear in real time; stdout is buffered.
	stdout, stderr, err := cmdArgs.RunSeparate(ctx, i.virtV2vInspectorPath, func(line string) {
		if shouldLogVirtV2vLine(line) {
			i.logger.WithField("vm_moref", vmMoref).Info(line)
		}
	})
	if ctx.Err() != nil {
		return nil, fmt.Errorf("virt-v2v-inspector command was cancelled: %w", ctx.Err())
	}

	stdoutStr := string(stdout)
	stderrStr := string(stderr)
	if len(stderr) > 0 && i.logger != nil {
		i.logger.WithField("stderr", stderrStr).Debug("virt-v2v-inspector stderr output")
	}

	if err != nil {
		exitCode := cmdbuilder.ExitCode(err)

		// Check if this is likely an encrypted disk error (check both stdout and stderr)
		combinedOutput := stdoutStr + stderrStr
		if encrypted, reason := isEncryptedDiskError(combinedOutput); encrypted {
			i.logger.WithFields(logrus.Fields{
				"stdout":          stdoutStr,
				"stderr":          stderrStr,
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
			"stdout":     stdoutStr,
			"stderr":     stderrStr,
			"exit_code":  exitCode,
			"executable": i.virtV2vInspectorPath,
			"args":       cmdArgs.MaskedArgs(),
		}).Error("virt-v2v-inspector failed")

		// Extract meaningful error lines from stderr for the error message.
		// Full output is already logged above.
		if summary := extractErrorSummary(stderrStr); summary != "" {
			return nil, fmt.Errorf("virt-v2v-inspector failed (exit code %d): %s", exitCode, summary)
		}
		return nil, fmt.Errorf("virt-v2v-inspector failed (exit code %d): %w", exitCode, err)
	}

	// Use stdout for XML parsing (stderr contains debug logs)
	inspectionData, err := parseV2VInspectionXML(stdout)
	if err != nil {
		if i.logger != nil {
			i.logger.WithFields(logrus.Fields{
				"error":  err,
				"stdout": stdoutStr,
				"stderr": stderrStr,
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

// resolveBaseDiskPaths returns existing when non-empty, otherwise the result of
// query. Separated from the vSphere call so the empty/populated/error branches
// are unit-testable without a live vCenter.
func resolveBaseDiskPaths(existing []string, query func() ([]string, error)) ([]string, error) {
	if len(existing) > 0 {
		return existing, nil
	}
	return query()
}

// queryBaseDiskPathsFromVSphere traverses the backing chain to get base disk
// paths. Mirrors VirtInspector.getBaseDiskPathsFromVSphere so virt-v2v-inspector
// works when the caller did not pre-populate diskInfo.BaseDiskPaths.
func queryBaseDiskPathsFromVSphere(ctx context.Context, vcenterURL, username, password, vmMoref string, tlsConfig *tlsconfig.Config, logger *logrus.Logger) ([]string, error) {
	vsphereClient, err := vsphere.NewClient(ctx, vcenterURL, username, password, tlsConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to vSphere: %w", err)
	}
	defer vsphereClient.Close()

	baseDiskPaths, err := vsphereClient.GetBaseDiskPaths(ctx, vmMoref)
	if err != nil {
		return nil, fmt.Errorf("failed to get base disk paths: %w", err)
	}
	return baseDiskPaths, nil
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

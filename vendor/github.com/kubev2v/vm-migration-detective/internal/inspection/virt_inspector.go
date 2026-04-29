package inspection

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kubev2v/vm-migration-detective/internal/vsphere"
	"github.com/kubev2v/vm-migration-detective/pkg/types"
	"github.com/sirupsen/logrus"
)

// UseVirtV2VOpen controls whether to use virt-v2v-open (true) or nbdkit directly (false)
// Default is false (use nbdkit directly)
const UseVirtV2VOpen = false

// Inspector handles VM inspection operations
type VirtInspector struct {
	virtInspectorPath string
	timeout           time.Duration
	logger            *logrus.Logger
}

// NewInspector creates a new Inspector instance
func NewVirtInspector(virtInspectorPath string, timeout time.Duration, logger *logrus.Logger) *VirtInspector {
	if virtInspectorPath == "" {
		virtInspectorPath = "virt-inspector" // Use system PATH
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &VirtInspector{
		virtInspectorPath: virtInspectorPath,
		timeout:           timeout,
		logger:            logger,
	}
}

func (i *VirtInspector) Inspect(
	ctx context.Context,
	vmMoref string,
	snapshotMoref string,
	vcenterURL string,
	username string,
	password string,
	diskInfo *types.SnapshotDiskInfo, // Snapshot disk info from vm_service
) (*types.VirtInspectorXML, error) {
	// Try inspection with automatic retry on cold-start VDDK crash
	// VDDK has a known bug where it crashes on first connection after container start
	// See: https://issues.redhat.com/browse/RHEL-54377
	result, err := i.attemptInspect(ctx, vmMoref, snapshotMoref, vcenterURL, username, password, diskInfo)
	if err != nil {
		// Check if this looks like a cold-start failure (connection refused, EOF, etc.)
		errStr := err.Error()
		isColdStartFailure := strings.Contains(errStr, "Connection refused") ||
			strings.Contains(errStr, "Unexpected end-of-file") ||
			strings.Contains(errStr, "Failed to read option reply")

		if isColdStartFailure && i.logger != nil {
			i.logger.WithError(err).Warn("Inspection failed with cold-start symptoms, retrying once")
			// Wait a moment for the system to stabilize
			time.Sleep(2 * time.Second)
			// Retry - VDDK should be warm now
			result, err = i.attemptInspect(ctx, vmMoref, snapshotMoref, vcenterURL, username, password, diskInfo)
			if err == nil && i.logger != nil {
				i.logger.Info("Inspection succeeded on retry (VDDK cold-start issue worked around)")
			}
		}
	}
	return result, err
}

func (i *VirtInspector) attemptInspect(
	ctx context.Context,
	vmMoref string,
	snapshotMoref string,
	vcenterURL string,
	username string,
	password string,
	diskInfo *types.SnapshotDiskInfo,
) (*types.VirtInspectorXML, error) {

	var nbdURLs []string
	var sessionCloser func()

	if UseVirtV2VOpen {
		i.logger.WithFields(logrus.Fields{
			"vm_moref":       vmMoref,
			"snapshot_moref": snapshotMoref,
			"vcenter_url":    vcenterURL,
		}).Info("Running virt-inspector using virt-v2v-open (VDDK + snapshot)")

		openCtx, cancel := context.WithTimeout(ctx, i.timeout)
		defer cancel()

		v2vSession, err := OpenWithVirtV2V(
			openCtx,
			vmMoref,
			snapshotMoref,
			vcenterURL,
			username,
			password,
		)
		if err != nil {
			return nil, err
		}
		nbdURLs = []string{v2vSession.NBDURL}
		sessionCloser = v2vSession.Close

		// Give NBD time to initialize
		time.Sleep(4 * time.Second)
	} else {
		i.logger.WithFields(logrus.Fields{
			"vm_moref":       vmMoref,
			"snapshot_moref": snapshotMoref,
			"vcenter_url":    vcenterURL,
		}).Info("Running virt-inspector using nbdkit-vddk (VDDK + snapshot)")

		i.logger.WithFields(logrus.Fields{
			"vm_moref":       diskInfo.VMMoref,
			"snapshot_moref": diskInfo.SnapshotMoref,
		}).Debug("Using VM and snapshot morefs from caller")

		// Query vSphere to get base disk paths by traversing backing chain
		baseDiskPaths, err := i.getBaseDiskPathsFromVSphere(ctx, vcenterURL, username, password, diskInfo.VMMoref)
		if err != nil {
			return nil, fmt.Errorf("failed to query base disk paths from vSphere: %w", err)
		}

		i.logger.WithFields(logrus.Fields{
			"disk_count":      len(baseDiskPaths),
			"base_disk_paths": baseDiskPaths,
		}).Info("Queried base disk paths from vSphere")

		openCtx, cancel := context.WithTimeout(ctx, i.timeout)
		defer cancel()

		// Start one NBDkit session per disk
		var nbdkitSessions []*NBDKitSession

		for idx, baseDiskPath := range baseDiskPaths {
			i.logger.WithFields(logrus.Fields{
				"disk_index":     idx,
				"base_disk_path": baseDiskPath,
			}).Debug("Starting NBDkit session for disk")

			nbdkitSession, err := OpenWithNBDKitVDDK(
				openCtx,
				diskInfo.VMMoref,
				diskInfo.SnapshotMoref,
				baseDiskPath,
				vcenterURL,
				username,
				password,
				i.logger,
			)
			if err != nil {
				// Close any sessions we've already created
				for _, session := range nbdkitSessions {
					session.Close()
				}
				return nil, fmt.Errorf("failed to start NBDkit session for disk %d: %w", idx, err)
			}
			nbdkitSessions = append(nbdkitSessions, nbdkitSession)
			nbdURLs = append(nbdURLs, nbdkitSession.NBDURL)

			// Wait for NBD server to be ready (more reliable than sleep)
			if err := nbdkitSession.WaitForReady(30 * time.Second); err != nil {
				i.logger.WithError(err).WithField("disk_index", idx).Error("NBD server not ready")
				// Close all sessions
				for _, session := range nbdkitSessions {
					session.Close()
				}
				return nil, fmt.Errorf("NBD server not ready for disk %d: %w", idx, err)
			}
		}

		// Create a cleanup function that closes all sessions
		sessionCloser = func() {
			for _, session := range nbdkitSessions {
				session.Close()
			}
		}
	}
	defer sessionCloser()

	inspectCtx, cancel := context.WithTimeout(ctx, i.timeout)
	defer cancel()

	i.logger.WithFields(logrus.Fields{
		"nbd_urls":   nbdURLs,
		"disk_count": len(nbdURLs),
	}).Info("Running virt-inspector on NBD")

	// Build command with multiple -a options for all disks
	// Format must be specified before each -a parameter
	var aOptions string
	for _, url := range nbdURLs {
		aOptions += fmt.Sprintf(" --format=raw -a '%s'", url)
	}
	cmdString := fmt.Sprintf("unset LD_LIBRARY_PATH && LIBGUESTFS_DEBUG=1 %s%s",
		i.virtInspectorPath, aOptions)

	virtInspectorCmd := exec.CommandContext(inspectCtx, "sh", "-c", cmdString)

	// Capture stdout and stderr separately
	// XML output goes to stdout, debug logs go to stderr
	stdout, stderr, err := captureSeparateOutput(virtInspectorCmd)

	// Log stderr (debug output) separately
	if len(stderr) > 0 && i.logger != nil {
		i.logger.WithField("stderr", string(stderr)).Debug("virt-inspector stderr output")
	}

	// Use stdout for XML parsing (stderr contains debug logs)
	outputStr := string(stdout)
	stderrStr := string(stderr)
	if err != nil {
		// Get exit code if available
		exitCode := -1
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}

		// Check if this is likely an encrypted disk error (check both stdout and stderr)
		combinedOutput := outputStr + stderrStr
		if isEncryptedDiskError(combinedOutput) {
			i.logger.WithFields(logrus.Fields{
				"stdout":     outputStr,
				"stderr":     stderrStr,
				"exit_code":  exitCode,
				"nbd_urls":   nbdURLs,
				"disk_count": len(nbdURLs),
				"command":    cmdString,
			}).Error("virt-inspector failed - disk appears to be encrypted")

			return nil, fmt.Errorf("disk encryption detected: virt-inspector cannot access encrypted disks. The VM disk appears to be encrypted and cannot be inspected without decryption. Exit code: %d", exitCode)
		}

		i.logger.WithFields(logrus.Fields{
			"stdout":     outputStr,
			"stderr":     stderrStr,
			"exit_code":  exitCode,
			"nbd_urls":   nbdURLs,
			"disk_count": len(nbdURLs),
			"command":    cmdString,
		}).Error("virt-inspector failed")

		// Include output in error message for better debugging
		if outputStr != "" || stderrStr != "" {
			return nil, fmt.Errorf("virt-inspector failed (exit code %d): %w\nStdout: %s\nStderr: %s", exitCode, err, outputStr, stderrStr)
		}
		return nil, fmt.Errorf("virt-inspector failed (exit code %d): %w", exitCode, err)
	}

	inspectionData, err := parseInspectionXML(stdout)
	if err != nil {
		if i.logger != nil {
			i.logger.WithFields(logrus.Fields{
				"error":  err,
				"stdout": outputStr,
				"stderr": string(stderr),
			}).Error("Failed to parse virt-inspector XML output")
		}
		return nil, fmt.Errorf("failed to parse inspection output: %w", err)
	}

	if UseVirtV2VOpen {
		i.logger.Info("virt-v2v-open snapshot inspection completed successfully")
	} else {
		i.logger.Info("nbdkit-vddk snapshot inspection completed successfully")
	}
	return inspectionData, nil
}

// parseInspectionXML parses virt-inspector XML output and returns the native XML structure
// It extracts XML from debug output if LIBGUESTFS_DEBUG is enabled
func parseInspectionXML(xmlData []byte) (*types.VirtInspectorXML, error) {
	outputStr := string(xmlData)

	// When LIBGUESTFS_DEBUG=1 is set, the output contains debug messages mixed with XML
	// Debug lines start with "libguestfs:" prefix
	// We need to extract just the XML portion by finding lines that don't start with debug prefix

	// First, try to find the XML start marker
	xmlStart := strings.Index(outputStr, "<?xml")
	if xmlStart == -1 {
		// Some versions don't include the XML declaration
		xmlStart = strings.Index(outputStr, "<operatingsystems")
	}

	if xmlStart == -1 {
		// No XML found - try parsing the whole output (maybe debug wasn't enabled)
		var xmlRoot types.VirtInspectorXML
		err := xml.Unmarshal(xmlData, &xmlRoot)
		if err != nil {
			return nil, fmt.Errorf("XML parsing error: %w", err)
		}
		if len(xmlRoot.Operatingsystems) == 0 {
			return nil, fmt.Errorf("no operating systems found in inspection output")
		}
		return &xmlRoot, nil
	}

	// Extract from XML start to end
	xmlOnlyStr := outputStr[xmlStart:]

	// Find the end of XML (after </operatingsystems>)
	xmlEnd := strings.Index(xmlOnlyStr, "</operatingsystems>")
	if xmlEnd > 0 {
		xmlEnd += len("</operatingsystems>")
		xmlOnlyStr = xmlOnlyStr[:xmlEnd]
	}

	// Remove any debug output that might be interspersed in the XML
	// Debug output starts with "libguestfs:" and can span multiple patterns
	xmlClean := xmlOnlyStr

	// Remove all occurrences of libguestfs debug lines
	// Pattern: lines starting with "libguestfs:" until newline
	for {
		startIdx := strings.Index(xmlClean, "libguestfs:")
		if startIdx == -1 {
			break
		}

		// Find the end of this debug line (next newline or end of string)
		endIdx := strings.Index(xmlClean[startIdx:], "\n")
		if endIdx == -1 {
			// No newline found, remove to end of string
			xmlClean = xmlClean[:startIdx]
			break
		}

		// Remove this debug line (including the newline)
		xmlClean = xmlClean[:startIdx] + xmlClean[startIdx+endIdx+1:]
	}

	// Parse the extracted XML
	var xmlRoot types.VirtInspectorXML
	err := xml.Unmarshal([]byte(xmlClean), &xmlRoot)
	if err != nil {
		// Log a sample of the cleaned XML for debugging
		sampleLen := 500
		if len(xmlClean) < sampleLen {
			sampleLen = len(xmlClean)
		}
		return nil, fmt.Errorf("XML parsing error: %w (XML sample: %s...)", err, xmlClean[:sampleLen])
	}

	if len(xmlRoot.Operatingsystems) == 0 {
		return nil, fmt.Errorf("no operating systems found in inspection output")
	}

	return &xmlRoot, nil
}

// getBaseDiskPathsFromVSphere queries vSphere to get base disk paths by traversing the backing chain
func (i *VirtInspector) getBaseDiskPathsFromVSphere(ctx context.Context, vcenterURL, username, password, vmMoref string) ([]string, error) {
	// Import the vsphere package
	vsphereClient, err := vsphere.NewClient(ctx, vcenterURL, username, password, true, i.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to vSphere: %w", err)
	}
	defer vsphereClient.Close()

	// Query base disk paths
	baseDiskPaths, err := vsphereClient.GetBaseDiskPaths(ctx, vmMoref)
	if err != nil {
		return nil, fmt.Errorf("failed to get base disk paths: %w", err)
	}

	return baseDiskPaths, nil
}

// captureSeparateOutput runs the command and captures stdout and stderr separately
func captureSeparateOutput(cmd *exec.Cmd) (stdout []byte, stderr []byte, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
}

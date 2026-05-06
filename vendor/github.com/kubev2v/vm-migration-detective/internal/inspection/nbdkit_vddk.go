package inspection

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/kubev2v/vm-migration-detective/internal/cmdbuilder"
	"github.com/kubev2v/vm-migration-detective/internal/vddk"
	"github.com/sirupsen/logrus"
)

// NBDKitSession represents an NBD server session created by nbdkit with VDDK plugin
type NBDKitSession struct {
	NBDURL       string // Unix socket path or NBD URL
	socketPath   string // Unix socket path (if using Unix socket)
	passwordFile string // Temporary password file path
	cmd          *exec.Cmd
	logger       *logrus.Logger
	stderrBuf    *bytes.Buffer
	stdoutBuf    *bytes.Buffer
}

// OpenWithNBDKitVDDK opens a VMware snapshot using nbdkit with VDDK plugin directly
// Parameters:
//   - vmMoref: VM managed object reference (e.g., "vm-123")
//   - snapshotMoref: Snapshot managed object reference (e.g., "snapshot-456")
//   - baseDiskPath: Base VMDK disk path (e.g., "[datastore] vm/vm.vmdk")
//   - vcenterURL: vCenter URL (e.g., "https://vcenter.example.com")
//   - username: vCenter username
//   - password: vCenter password
//   - logger: Logger instance
func OpenWithNBDKitVDDK(
	ctx context.Context,
	vmMoref string,
	snapshotMoref string,
	baseDiskPath string,
	vcenterURL string,
	username string,
	password string,
	logger *logrus.Logger,
) (*NBDKitSession, error) {
	// Parse vCenter URL to extract hostname
	parsedURL, err := url.Parse(vcenterURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vCenter URL: %w", err)
	}
	vcenterHost := parsedURL.Hostname()

	// Get vCenter SSL thumbprint
	var thumbprint string
	if logger != nil {
		logger.Debug("Getting vCenter SSL thumbprint")
	}
	thumbprint, err = getVCenterThumbprint(vcenterHost)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("Failed to get thumbprint, proceeding without SSL verification")
		}
		thumbprint = ""
	}
	if thumbprint != "" && logger != nil {
		logger.WithField("thumbprint", thumbprint).Debug("Got vCenter thumbprint")
	}
	// Create temporary Unix socket for nbdkit (more reliable than TCP port)
	socketPath := filepath.Join("/tmp", fmt.Sprintf("nbdkit-%s.sock", uuid.New().String()))

	// Create temporary password file for secure password passing
	passwordFile, err := createNBDKitPasswordFile(password)
	if err != nil {
		return nil, fmt.Errorf("failed to create password file: %w", err)
	}

	// Get VDDK library directory from common configuration
	vddkLibDir := vddk.GetLibDir()
	if vddkLibDir == "" {
		return nil, fmt.Errorf("VDDK library directory not found - ensure VDDK is installed or configured")
	}

	// password=+file keeps the password out of the process list.
	nbdkitCmd := cmdbuilder.New().
		WithLogger(logger).
		Flag("-U", socketPath).
		Add("--foreground").
		Add("--exit-with-parent").
		Add("-r").
		Add("vddk").
		Add(fmt.Sprintf("server=%s", vcenterHost)).
		Add(fmt.Sprintf("user=%s", username)).
		SensitiveArg(fmt.Sprintf("password=+%s", passwordFile), "password=+***").
		Add(fmt.Sprintf("vm=moref=%s", vmMoref)).
		Add(fmt.Sprintf("snapshot=%s", snapshotMoref)).
		Add(fmt.Sprintf("file=%s", baseDiskPath)).
		Add(fmt.Sprintf("libdir=%s", vddkLibDir)).
		AddIf(thumbprint != "", fmt.Sprintf("thumbprint=%s", thumbprint)) // Add thumbprint if available (for SSL verification)

	if logger != nil {
		logger.WithFields(logrus.Fields{
			"socket_path":    socketPath,
			"vm_moref":       vmMoref,
			"snapshot_moref": snapshotMoref,
			"disk_path":      baseDiskPath,
		}).Info("Starting nbdkit with VDDK plugin")
	}

	// nbdkit is a long-lived server process; Start() not Run().
	cmd := nbdkitCmd.Command(ctx, "nbdkit")

	// Capture both stdout and stderr to check for errors
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	cmd.Stderr = stderrBuf
	cmd.Stdout = stdoutBuf

	// Start nbdkit
	// nbdkit is a long-lived server process so Start() not Run()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start nbdkit: %w", err)
	}

	// Wait a moment for nbdkit to start
	time.Sleep(2 * time.Second)

	// Check if process is still running
	// ProcessState is only set after Wait(), so we need to check the process directly
	if cmd.Process == nil {
		return nil, fmt.Errorf("nbdkit process is nil after start")
	}

	// Check if process has exited by sending signal 0 (doesn't kill, just checks)
	if err := cmd.Process.Signal(os.Signal(syscall.Signal(0))); err != nil {
		// Process has exited, read stderr and stdout for error messages
		stderrOutput := stderrBuf.String()
		stdoutOutput := stdoutBuf.String()
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"stderr":      stderrOutput,
				"stdout":      stdoutOutput,
				"socket_path": socketPath,
			}).Error("nbdkit process exited immediately")
		}
		errorMsg := "nbdkit process exited immediately"
		if stderrOutput != "" {
			errorMsg += fmt.Sprintf(" (stderr: %s)", stderrOutput)
		}
		if stdoutOutput != "" {
			errorMsg += fmt.Sprintf(" (stdout: %s)", stdoutOutput)
		}
		return nil, fmt.Errorf("%s", errorMsg)
	}

	// Build NBD URL using Unix socket format (matching origin/main)
	nbdURL := fmt.Sprintf("nbd+unix:///?socket=%s", socketPath)

	return &NBDKitSession{
		NBDURL:       nbdURL,
		socketPath:   socketPath,
		passwordFile: passwordFile,
		cmd:          cmd,
		logger:       logger,
		stderrBuf:    stderrBuf,
		stdoutBuf:    stdoutBuf,
	}, nil
}

// Close stops the nbdkit process and cleans up
func (s *NBDKitSession) Close() {
	if s == nil {
		return
	}

	if s.cmd != nil && s.cmd.Process != nil {
		// Send SIGTERM first for graceful shutdown
		_ = s.cmd.Process.Signal(os.Interrupt)

		// Wait a bit for graceful shutdown
		done := make(chan error, 1)
		go func() {
			done <- s.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(5 * time.Second):
			// Force kill if it doesn't exit
			_ = s.cmd.Process.Kill()
			_, _ = s.cmd.Process.Wait()
		}
	}

	// Clean up Unix socket file
	if s.socketPath != "" {
		_ = os.Remove(s.socketPath)
	}

	// Clean up password file
	if s.passwordFile != "" {
		_ = os.Remove(s.passwordFile)
	}
}

// WaitForReady waits for the NBD server to be ready by checking if the Unix socket exists
func (s *NBDKitSession) WaitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	// First, verify the process is still running
	if s.cmd != nil && s.cmd.Process != nil {
		// Check if process is still alive
		if err := s.cmd.Process.Signal(os.Signal(syscall.Signal(0))); err != nil {
			return fmt.Errorf("nbdkit process died before NBD server was ready: %w", err)
		}
	}

	checkCount := 0
	for time.Now().Before(deadline) {
		checkCount++

		// Check if process is still running
		if s.cmd != nil && s.cmd.Process != nil {
			if err := s.cmd.Process.Signal(os.Signal(syscall.Signal(0))); err != nil {
				// Process died, get error output
				errorDetails := ""
				if s.stderrBuf != nil {
					errorDetails = s.stderrBuf.String()
				}
				if s.stdoutBuf != nil && errorDetails == "" {
					errorDetails = s.stdoutBuf.String()
				}
				if s.logger != nil {
					s.logger.WithFields(logrus.Fields{
						"stderr":      errorDetails,
						"socket_path": s.socketPath,
					}).Error("nbdkit process died while waiting for NBD server")
				}
				if errorDetails != "" {
					return fmt.Errorf("nbdkit process died while waiting for NBD server: %w (output: %s)", err, errorDetails)
				}
				return fmt.Errorf("nbdkit process died while waiting for NBD server: %w", err)
			}
		}

		// Periodically log nbdkit output for debugging (every 5 seconds)
		if s.logger != nil && checkCount%10 == 0 {
			if s.stderrBuf != nil && s.stderrBuf.Len() > 0 {
				s.logger.WithField("stderr", s.stderrBuf.String()).Debug("nbdkit stderr output")
			}
			if s.stdoutBuf != nil && s.stdoutBuf.Len() > 0 {
				s.logger.WithField("stdout", s.stdoutBuf.String()).Debug("nbdkit stdout output")
			}
		}

		// Check if Unix socket exists
		if _, err := os.Stat(s.socketPath); err == nil {
			// Socket file exists, wait briefly for NBD server to be ready
			// Note: VDDK does lazy init on first connection, which can cause cold-start issues
			// We handle this with retry logic in the inspection layer
			time.Sleep(2 * time.Second)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Final check - did the process die?
	if s.cmd != nil && s.cmd.Process != nil {
		if err := s.cmd.Process.Signal(os.Signal(syscall.Signal(0))); err != nil {
			// Try to read any error output
			errorDetails := ""
			if s.stderrBuf != nil {
				errorDetails = s.stderrBuf.String()
			}
			if s.stdoutBuf != nil && errorDetails == "" {
				errorDetails = s.stdoutBuf.String()
			}
			if s.logger != nil {
				s.logger.WithFields(logrus.Fields{
					"stderr":      errorDetails,
					"socket_path": s.socketPath,
				}).Error("nbdkit process died while waiting for NBD server")
			}
			if errorDetails != "" {
				return fmt.Errorf("nbdkit process died: %w (NBD server not ready after %v, error: %s)", err, timeout, errorDetails)
			}
			return fmt.Errorf("nbdkit process died: %w (NBD server not ready after %v)", err, timeout)
		}
	}

	// Log that process is running but socket is not accessible
	errorDetails := ""
	if s.stderrBuf != nil {
		errorDetails = s.stderrBuf.String()
	}
	if s.stdoutBuf != nil && errorDetails == "" {
		errorDetails = s.stdoutBuf.String()
	}

	if s.logger != nil {
		s.logger.WithFields(logrus.Fields{
			"socket_path": s.socketPath,
			"stderr":      errorDetails,
			"stdout": func() string {
				if s.stdoutBuf != nil {
					return s.stdoutBuf.String()
				}
				return ""
			}(),
		}).Error("NBD server process running but socket not accessible")
	}

	// Include nbdkit error output in the error message
	if errorDetails != "" {
		return fmt.Errorf("NBD server not ready after %v (process still running, but socket %s not accessible). nbdkit output: %s", timeout, s.socketPath, errorDetails)
	}
	return fmt.Errorf("NBD server not ready after %v (process still running, but socket %s not accessible)", timeout, s.socketPath)
}

// getVCenterThumbprint gets the SSL certificate thumbprint from vCenter
func getVCenterThumbprint(vcenterHost string) (string, error) {
	// Connect to vCenter to get SSL certificate
	conn, err := tls.Dial("tcp", vcenterHost+":443", &tls.Config{
		InsecureSkipVerify: true, // We just need the cert, not to verify it
	})
	if err != nil {
		return "", fmt.Errorf("failed to connect to vCenter: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Get the certificate chain
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", fmt.Errorf("no certificates found")
	}

	// Use the first certificate (server certificate)
	cert := certs[0]

	// Calculate SHA-256 thumbprint
	thumbprint := sha256.Sum256(cert.Raw)

	// Format as colon-separated hex string (VMware format)
	hexThumbprint := hex.EncodeToString(thumbprint[:])
	formatted := ""
	for i := 0; i < len(hexThumbprint); i += 2 {
		if i > 0 {
			formatted += ":"
		}
		formatted += hexThumbprint[i : i+2]
	}

	return formatted, nil
}

// createNBDKitPasswordFile creates a temporary file with the password for nbdkit
// nbdkit-vddk-plugin supports password=+file to read password securely from file
func createNBDKitPasswordFile(password string) (string, error) {
	tmpFile, err := os.CreateTemp("", "nbdkit-password-*")
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

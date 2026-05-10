package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
)

const (
	// Host /dev is mounted at /host-dev (not /dev) so that kubelet's
	// VolumeDevice bind mount at /populatorblock is not shadowed.
	iscsiByPathDir    = "/host-dev/disk/by-path"
	devicePollTimeout = 120 * time.Second
	devicePollDelay   = 500 * time.Millisecond
	metricsPort       = ":8443"
	// ddStallTimeout is how long we wait for dd to produce progress output
	// before considering it stalled (e.g. iSCSI session dropped).
	ddStallTimeout = 30 * time.Minute
)

// Directories required by iscsid at runtime (lock files, node DB, etc.).
var iscsidRuntimeDirs = []string{
	"/run/lock/iscsi",
	"/run/lock",
	"/var/run",
	"/etc/iscsi/nodes",
	"/etc/iscsi/ifaces",
	"/etc/iscsi/send_targets",
}

// DiskSpec describes one LUN to copy.
type DiskSpec struct {
	LunID      int    `json:"lunId"`
	VolumePath string `json:"volumePath"`
}

type lunWork struct {
	disk       DiskSpec
	devicePath string
	sizeBytes  int64
}

type outputTracker struct {
	mu         sync.Mutex
	lastOutput time.Time
}

var (
	iscsidCmd     *exec.Cmd
	useHostIscsid = true
	pvcOwnerUID   string
)

var (
	portalRe = regexp.MustCompile(`^(\[[a-fA-F0-9:]+\]|[a-zA-Z0-9.\-]+):\d+$`)
	iqnRe    = regexp.MustCompile(`^iqn\.[0-9]{4}-[0-9]{2}\.[a-zA-Z0-9.\-:]+$`)
)

// progressGauge drives the UI progress bar via the populator-controller.
var progressGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "progress",
		Help: "Copy progress percentage (0-100), keyed by PVC owner UID.",
	},
	[]string{"ownerUID"},
)

func main() {
	var portal, targetIQN, initiatorIQN, diskSpecsJSON string
	flag.StringVar(&portal, "portal", "", "iSCSI target portal (host:port)")
	flag.StringVar(&targetIQN, "target-iqn", "", "iSCSI target IQN")
	flag.StringVar(&initiatorIQN, "initiator-iqn", "", "iSCSI initiator IQN for ACL")
	flag.StringVar(&diskSpecsJSON, "disk-specs", "", "JSON array of disk specs [{lunId, volumePath}]")
	// The populator controller passes these flags to every populator pod.
	flag.String("cr-name", "", "")
	flag.String("cr-namespace", "", "")
	flag.String("secret-name", "", "")
	flag.Int64("pvc-size", 0, "")
	flag.StringVar(&pvcOwnerUID, "owner-uid", "", "PVC owner UID for progress reporting")
	klog.InitFlags(nil)
	flag.Parse()

	if portal == "" || targetIQN == "" || initiatorIQN == "" || diskSpecsJSON == "" {
		klog.Fatal("Required parameters: --portal, --target-iqn, --initiator-iqn, --disk-specs")
	}

	var disks []DiskSpec
	if err := json.Unmarshal([]byte(diskSpecsJSON), &disks); err != nil {
		klog.Fatalf("Failed to parse --disk-specs: %v", err)
	}
	if len(disks) == 0 {
		klog.Fatal("--disk-specs is empty — at least one disk must be specified")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigs
		klog.Infof("Received signal %v, initiating shutdown", sig)
		cancel()
	}()

	startMetricsServer()

	if err := run(ctx, portal, targetIQN, initiatorIQN, disks); err != nil {
		stopIscsid()
		klog.Fatalf("Copy failed: %v", err)
	}
	stopIscsid()
	klog.Info("All disks copied successfully")
}

// hostCmd always runs in the host's mount namespace via nsenter.
func hostCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	nsenterArgs := []string{"-t", "1", "-m", "--", name}
	nsenterArgs = append(nsenterArgs, args...)
	return exec.CommandContext(ctx, "nsenter", nsenterArgs...)
}

// iscsiCmd routes iSCSI commands to the host or the container depending on
// which iscsid daemon is in use. When the host daemon is reachable, commands
// run via nsenter so they're version-matched. When using the local fallback
// daemon, commands run directly in the container.
func iscsiCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	if useHostIscsid {
		return hostCmd(ctx, name, args...)
	}
	return exec.CommandContext(ctx, name, args...)
}

// run connects to the Hyper-V iSCSI target and copies all LUNs to their destination volumes.
func run(ctx context.Context, portal, targetIQN, initiatorIQN string, disks []DiskSpec) error {
	if !portalRe.MatchString(portal) {
		return fmt.Errorf("invalid portal format: %s", portal)
	}
	if !iqnRe.MatchString(targetIQN) {
		return fmt.Errorf("invalid target IQN format: %s", targetIQN)
	}
	if !iqnRe.MatchString(initiatorIQN) {
		return fmt.Errorf("invalid initiator IQN format: %s", initiatorIQN)
	}

	// Stage 1: ensure iscsid is available (host daemon or local fallback)
	klog.Info("Ensuring iscsid is available")
	if err := startIscsid(); err != nil {
		return fmt.Errorf("start iscsid: %w", err)
	}

	// Stage 2: create a per-session iface with our initiator IQN
	ifaceName, err := createCustomIface(ctx, initiatorIQN)
	if err != nil {
		return fmt.Errorf("create custom iface: %w", err)
	}
	defer deleteCustomIface(ctx, ifaceName)

	// Stage 3: create a static node record and log in to the target
	klog.Infof("Creating iSCSI node record for %s on %s (iface %s)", targetIQN, portal, ifaceName)
	if err := iscsiCreateNode(ctx, portal, targetIQN, ifaceName); err != nil {
		return fmt.Errorf("iSCSI node create for %s: %w", targetIQN, err)
	}
	defer func() {
		if err := iscsiDeleteNode(context.Background(), portal, targetIQN, ifaceName); err != nil {
			klog.Errorf("iSCSI node delete failed (non-fatal): %v", err)
		}
	}()

	klog.Infof("Logging in to target %s on %s", targetIQN, portal)
	if err := iscsiLogin(ctx, portal, targetIQN, ifaceName); err != nil {
		return fmt.Errorf("iSCSI login to %s: %w", targetIQN, err)
	}

	defer func() {
		// Stage 5: tear down — logout
		klog.Info("Logging out of iSCSI session")
		if err := iscsiLogout(context.Background(), portal, targetIQN, ifaceName); err != nil {
			klog.Errorf("iSCSI logout failed (non-fatal): %v", err)
		}
	}()

	work, totalJobBytes, err := discoverLUNs(ctx, portal, targetIQN, disks)
	if err != nil {
		return err
	}

	return copyAllDisks(ctx, work, totalJobBytes)
}

// discoverLUNs waits for each LUN's block device to appear and reads its
// size. Sizes are collected up front so the progress gauge reflects the
// overall job, not individual disks.
func discoverLUNs(ctx context.Context, portal, targetIQN string, disks []DiskSpec) ([]lunWork, int64, error) {
	var work []lunWork
	var totalJobBytes int64
	for _, disk := range disks {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		devicePath, err := waitForDevice(ctx, portal, targetIQN, disk.LunID)
		if err != nil {
			return nil, 0, fmt.Errorf("wait for device LUN %d: %w", disk.LunID, err)
		}
		klog.Infof("LUN %d device found at %s", disk.LunID, devicePath)

		sz, err := getDeviceSize(devicePath)
		if err != nil {
			klog.Warningf("Could not determine device size for %s: %v", devicePath, err)
		} else {
			klog.Infof("LUN %d device size: %d bytes", disk.LunID, sz)
		}
		totalJobBytes += sz
		work = append(work, lunWork{disk: disk, devicePath: devicePath, sizeBytes: sz})
	}
	return work, totalJobBytes, nil
}

func copyAllDisks(ctx context.Context, work []lunWork, totalJobBytes int64) error {
	var completedBytes int64
	for _, w := range work {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := copyDisk(ctx, w.devicePath, w.disk, completedBytes, totalJobBytes); err != nil {
			return fmt.Errorf("copy disk LUN %d: %w", w.disk.LunID, err)
		}
		completedBytes += w.sizeBytes
	}

	if pvcOwnerUID != "" {
		progressGauge.WithLabelValues(pvcOwnerUID).Set(100)
	}
	return nil
}

// createCustomIface creates a per-session iSCSI iface that binds the initiator
// IQN to this session only. This avoids touching the global initiatorname.iscsi
// file and ensures iscsid uses the correct IQN for session reconnection.
func createCustomIface(ctx context.Context, initiatorIQN string) (string, error) {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("generate iface suffix: %w", err)
	}
	ifaceName := fmt.Sprintf("forklift-%d-%s", os.Getpid(), hex.EncodeToString(suffix[:]))

	cmd := iscsiCmd(ctx, "iscsiadm", "-m", "iface", "-I", ifaceName, "--op=new")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("create iface %s: %w\noutput: %s", ifaceName, err, string(out))
	}
	klog.Infof("Created custom iface %s", ifaceName)

	cmd = iscsiCmd(ctx, "iscsiadm", "-m", "iface", "-I", ifaceName,
		"--op=update", "-n", "iface.initiatorname", "-v", initiatorIQN)
	out, err = cmd.CombinedOutput()
	if err != nil {
		deleteCustomIface(ctx, ifaceName)
		return "", fmt.Errorf("set initiator IQN on iface %s: %w\noutput: %s", ifaceName, err, string(out))
	}
	klog.Infof("Set initiator IQN on iface %s to %s", ifaceName, initiatorIQN)
	return ifaceName, nil
}

func deleteCustomIface(ctx context.Context, ifaceName string) {
	if ifaceName == "" {
		return
	}
	cmd := iscsiCmd(ctx, "iscsiadm", "-m", "iface", "-I", ifaceName, "--op=delete")
	out, err := cmd.CombinedOutput()
	if err != nil {
		klog.Warningf("Failed to delete iface %s (non-fatal): %v\noutput: %s", ifaceName, err, string(out))
		return
	}
	klog.Infof("Deleted custom iface %s", ifaceName)
}

func copyDisk(ctx context.Context, devicePath string, disk DiskSpec, completedBytes, totalJobBytes int64) error {
	lunLabel := strconv.Itoa(disk.LunID)

	klog.Infof("Starting dd copy: %s -> %s", devicePath, disk.VolumePath)
	if err := ddCopy(ctx, devicePath, disk.VolumePath, completedBytes, totalJobBytes, lunLabel); err != nil {
		return err
	}

	if err := verifyDiskNotEmpty(disk.VolumePath, disk.LunID); err != nil {
		return err
	}

	klog.Infof("LUN %d copy complete", disk.LunID)
	return nil
}

// verifyDiskNotEmpty checks the destination for a valid MBR/GPT signature.
// An all-zero disk means the iSCSI LUN served empty data.
func verifyDiskNotEmpty(path string, lunID int) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("post-copy verify: open %s: %w", path, err)
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		return fmt.Errorf("post-copy verify: read %s: %w", path, err)
	}
	buf = buf[:n]

	allZero := true
	for _, b := range buf {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return fmt.Errorf("post-copy verify FAILED for LUN %d: first %d bytes of %s are all zeros. "+
			"The iSCSI source LUN likely served empty data (check differencing disk setup on the Hyper-V host)",
			lunID, n, path)
	}

	// MBR disks carry the boot signature 0x55AA at bytes 510-511.
	// GPT disks have the ASCII string "EFI PART" at the start of LBA 1 (byte 512).
	hasMBR := n >= 512 && buf[510] == 0x55 && buf[511] == 0xAA
	hasGPT := n >= 520 && string(buf[512:520]) == "EFI PART"
	if hasMBR || hasGPT {
		klog.Infof("LUN %d post-copy verify: valid partition table found (MBR=%v, GPT=%v)", lunID, hasMBR, hasGPT)
	} else {
		klog.Warningf("LUN %d post-copy verify: no MBR/GPT signature in first %d bytes "+
			"(may be a raw filesystem or data disk — not necessarily an error)", lunID, n)
	}
	return nil
}

// startIscsid ensures the iSCSI daemon is available.
func startIscsid() error {
	for _, dir := range iscsidRuntimeDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create runtime dir %s: %w", dir, err)
		}
	}

	// hostCmd runs iscsiadm in the node's mount namespace; if iscsid is running
	// there, `iscsiadm -m session` exits 0 or 21 (no sessions) and we skip
	// starting iscsid inside this container.
	//
	// The "No active sessions" check is output-based and may vary with locale
	// or iscsiadm version. A false negative is safe: we just start a local
	// iscsid, which works fine alongside the host daemon since the populator
	// uses its own per-session iface.
	probe := hostCmd(context.Background(), "iscsiadm", "-m", "session")
	if out, err := probe.CombinedOutput(); err == nil || strings.Contains(string(out), "No active sessions") {
		klog.Info("Host iscsid is reachable, skipping local daemon start")
		return nil
	}

	klog.Info("No reachable host iscsid, starting local daemon inside the container")
	useHostIscsid = false
	iscsidCmd = exec.Command("iscsid", "--foreground", "--no-pid-file")
	iscsidCmd.Stdout = os.Stdout
	iscsidCmd.Stderr = os.Stderr
	if err := iscsidCmd.Start(); err != nil {
		return fmt.Errorf("iscsid start: %w", err)
	}
	klog.Infof("iscsid process started (pid=%d), waiting for socket...", iscsidCmd.Process.Pid)
	time.Sleep(1 * time.Second)
	klog.Info("Local iscsid started")
	return nil
}

func stopIscsid() {
	if iscsidCmd != nil && iscsidCmd.Process != nil {
		_ = iscsidCmd.Process.Kill()
		_ = iscsidCmd.Wait()
		klog.Info("Local iscsid stopped")
	}
}

// iscsiCreateNode creates a static node record for the target so we can
// login without relying on SendTargets discovery (which Windows filters by ACL).
func iscsiCreateNode(ctx context.Context, portal, targetIQN, ifaceName string) error {
	cmd := iscsiCmd(ctx, "iscsiadm",
		"-m", "node",
		"-T", targetIQN,
		"-p", portal,
		"-I", ifaceName,
		"--op=new",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// iscsiadm exit code 15 means the node record already exists.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 15 {
			klog.Info("iSCSI node record already exists, continuing")
			return nil
		}
		return fmt.Errorf("iscsiadm node new: %w\noutput: %s", err, string(out))
	}
	return nil
}

func iscsiLogin(ctx context.Context, portal, targetIQN, ifaceName string) error {
	cmd := iscsiCmd(ctx, "iscsiadm",
		"-m", "node",
		"-T", targetIQN,
		"-p", portal,
		"-I", ifaceName,
		"--login",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iscsiadm login: %w\noutput: %s", err, string(out))
	}
	klog.V(2).Infof("Login output:\n%s", string(out))
	return nil
}

func iscsiLogout(ctx context.Context, portal, targetIQN, ifaceName string) error {
	cmd := iscsiCmd(ctx, "iscsiadm",
		"-m", "node",
		"-T", targetIQN,
		"-p", portal,
		"-I", ifaceName,
		"--logout",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iscsiadm logout: %w\noutput: %s", err, string(out))
	}
	return nil
}

func iscsiDeleteNode(ctx context.Context, portal, targetIQN, ifaceName string) error {
	cmd := iscsiCmd(ctx, "iscsiadm",
		"-m", "node",
		"-T", targetIQN,
		"-p", portal,
		"-I", ifaceName,
		"--op=delete",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iscsiadm node delete: %w\noutput: %s", err, string(out))
	}
	return nil
}

func waitForDevice(ctx context.Context, portal, targetIQN string, lunID int) (string, error) {
	if _, err := os.Stat(iscsiByPathDir); os.IsNotExist(err) {
		return "", fmt.Errorf("%s does not exist in the container; "+
			"ensure the pod mounts the host's /dev at /host-dev", iscsiByPathDir)
	}

	host, _, err := net.SplitHostPort(portal)
	if err != nil {
		host = portal
	}
	pattern := fmt.Sprintf("ip-%s:*-iscsi-%s-lun-%d", host, targetIQN, lunID)

	enxioLogged := false
	deadline := time.Now().Add(devicePollTimeout)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timeout waiting for device matching %s in %s", pattern, iscsiByPathDir)
		}

		if path, ok, err := tryResolveDevice(pattern, lunID, &enxioLogged); err != nil {
			return "", err
		} else if ok {
			return path, nil
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(devicePollDelay):
		}
	}
}

// tryResolveDevice checks whether the iSCSI device symlink exists and is openable.
func tryResolveDevice(pattern string, lunID int, enxioLogged *bool) (string, bool, error) {
	matches, globErr := filepath.Glob(filepath.Join(iscsiByPathDir, pattern))
	if globErr != nil {
		return "", false, fmt.Errorf("glob %s/%s: %w", iscsiByPathDir, pattern, globErr)
	}
	if len(matches) == 0 {
		return "", false, nil
	}
	sort.Strings(matches)
	resolved, err := filepath.EvalSymlinks(matches[0])
	if err != nil {
		return "", false, fmt.Errorf("resolve symlink %s: %w", matches[0], err)
	}
	f, openErr := os.Open(resolved)
	if openErr != nil {
		if !*enxioLogged {
			klog.Infof("LUN %d device %s not ready (%v), will keep retrying...", lunID, resolved, openErr)
			*enxioLogged = true
		}
		return "", false, nil
	}
	f.Close()
	return resolved, true, nil
}

// getDeviceSize reads the block device size in bytes from /sys/block/<dev>/size.
// Uses nsenter when the host iscsid owns the device.
func getDeviceSize(devicePath string) (int64, error) {
	base := filepath.Base(devicePath)
	sizeFile := filepath.Join("/sys/class/block", base, "size")

	var data string
	if useHostIscsid {
		cmd := hostCmd(context.Background(), "cat", sizeFile)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return 0, fmt.Errorf("read host sysfs %s: %w (output: %s)", sizeFile, err, string(out))
		}
		data = string(out)
	} else {
		raw, err := os.ReadFile(sizeFile)
		if err != nil {
			return 0, err
		}
		data = string(raw)
	}

	sectors, err := strconv.ParseInt(strings.TrimSpace(data), 10, 64)
	if err != nil {
		return 0, err
	}
	return sectors * 512, nil
}

// ddCopy uses dd with oflag=direct to copy src to dst, bypassing the page
// cache entirely so every byte is written straight to the backing store.
// completedBytes is the number of bytes already copied by earlier LUNs;
// totalJobBytes is the grand total across all LUNs for the overall gauge.
func ddCopy(ctx context.Context, src, dst string, completedBytes, totalJobBytes int64, lunLabel string) error {
	args := []string{
		"if=" + src,
		"of=" + dst,
		"bs=8M",
		"iflag=direct",
		"oflag=direct",
		"status=progress",
		"conv=notrunc",
	}

	stallCtx, stallCancel := context.WithCancel(ctx)
	defer stallCancel()

	cmd := exec.CommandContext(stallCtx, "dd", args...)
	cmd.Stdout = os.Stdout

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("dd stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("dd start: %w", err)
	}

	stallTracker := &outputTracker{}
	go stallTracker.watchForStall(stallCtx, stallCancel)

	var wg sync.WaitGroup
	wg.Add(1)
	go trackDDProgress(&wg, stderrPipe, stallTracker, completedBytes, totalJobBytes, lunLabel)

	if err := cmd.Wait(); err != nil {
		if stallCtx.Err() != nil && ctx.Err() == nil {
			return fmt.Errorf("dd stalled (no output for %v): %w", ddStallTimeout, err)
		}
		return fmt.Errorf("dd copy failed: %w", err)
	}
	wg.Wait()

	return nil
}

func (t *outputTracker) touch() {
	t.mu.Lock()
	t.lastOutput = time.Now()
	t.mu.Unlock()
}

func (t *outputTracker) watchForStall(ctx context.Context, cancel context.CancelFunc) {
	t.touch()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.mu.Lock()
			stalled := time.Since(t.lastOutput) > ddStallTimeout
			t.mu.Unlock()
			if stalled {
				klog.Errorf("dd stalled for %v with no progress; killing", ddStallTimeout)
				cancel()
				return
			}
		}
	}
}

// trackDDProgress parses dd's stderr progress output (e.g. "1073741824 bytes ... copied")
// and reports transfer percentage. dd uses \r for in-place updates and \n for
// final summary lines, so we split on either delimiter.
func trackDDProgress(wg *sync.WaitGroup, stderrPipe io.Reader, tracker *outputTracker, completedBytes, totalJobBytes int64, lunLabel string) {
	defer wg.Done()
	var lastLogged float64
	scanner := bufio.NewScanner(stderrPipe)
	scanner.Split(splitOnCRorLF)
	for scanner.Scan() {
		tracker.touch()
		line := scanner.Text()
		bytesCopied := parseDDProgress(line)
		if bytesCopied <= 0 || totalJobBytes <= 0 {
			continue
		}
		pct := float64(completedBytes+bytesCopied) / float64(totalJobBytes) * 100
		if pct > 100 {
			pct = 100
		}
		if pvcOwnerUID != "" {
			progressGauge.WithLabelValues(pvcOwnerUID).Set(pct)
		}
		if pct-lastLogged >= 5 || pct >= 100 {
			klog.Infof("LUN %s copy progress: %.1f%% (%d / %d bytes)",
				lunLabel, pct, completedBytes+bytesCopied, totalJobBytes)
			lastLogged = pct
		}
	}
}

func parseDDProgress(line string) int64 {
	idx := strings.Index(line, " bytes")
	if idx <= 0 {
		return 0
	}
	numStr := strings.TrimSpace(line[:idx])
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// splitOnCRorLF is a bufio.SplitFunc that tokenizes on \r or \n,
// matching dd's progress output style (uses \r for in-place updates).
func splitOnCRorLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		if b == '\r' || b == '\n' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func startMetricsServer() {
	prometheus.MustRegister(progressGauge)

	certsDir, err := os.MkdirTemp("", "hyperv-populator-certs")
	if err != nil {
		klog.Fatalf("Failed to create temp dir for TLS certs: %v", err)
	}
	// Pod is short-lived; certsDir is cleaned up when the container exits.

	certBytes, keyBytes, err := cert.GenerateSelfSignedCertKey("", nil, nil)
	if err != nil {
		klog.Fatalf("Failed to generate self-signed cert: %v", err)
	}

	certFile := filepath.Join(certsDir, "tls.crt")
	keyFile := filepath.Join(certsDir, "tls.key")
	if err := os.WriteFile(certFile, certBytes, 0600); err != nil {
		klog.Fatalf("Failed to write TLS cert: %v", err)
	}
	if err := os.WriteFile(keyFile, keyBytes, 0600); err != nil {
		klog.Fatalf("Failed to write TLS key: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	go func() {
		server := &http.Server{
			Addr:      metricsPort,
			Handler:   mux,
			TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		}
		klog.Infof("Starting metrics server on %s", metricsPort)
		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			klog.Fatalf("Metrics server failed: %v", err)
		}
	}()
}

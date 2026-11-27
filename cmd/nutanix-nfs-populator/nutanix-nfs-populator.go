// nutanix-nfs-populator reads a Nutanix VM disk from the storage container
// export — mounted read-only by kubelet as an in-tree nfs volume at /source —
// and writes it into the target volume, bypassing the Prism REST Image Service
// (and its HTTP 429 rate limiting). The binary itself never speaks NFS; the
// NFS endpoint (server/export) lives in the pod volume spec built by the
// populator-controller, not in this process.
//
// It is launched by the forklift populator-controller (via the lib-volume
// populator machinery) with the NutanixNfsVolumePopulator spec rendered as CLI
// flags. Progress is exposed as a Prometheus gauge named
// "nutanix_nfs_populator_progress" labelled by ownerUID (the target PVC UID),
// which the machinery scrapes and copies into the CR's status.progress.
package main

import (
	"bufio"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kubev2v/forklift/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

// sourceMountPath is where the populator pod mounts the Nutanix storage
// container export (read-only, attached by kubelet).
const sourceMountPath = "/source"

type config struct {
	volumePath string
	sourcePath string
	pvcSize    int64
	crName     string
	crNS       string
	ownerUID   string
	secretName string
}

var progressRE = regexp.MustCompile(`\(([0-9]+\.?[0-9]*)/100%\)`)

func main() {
	cfg := &config{}
	flag.StringVar(&cfg.volumePath, "volume-path", "", "Path to populate (block device or disk.img)")
	flag.StringVar(&cfg.sourcePath, "source-path", "", "Path to the disk file within the NFS source mount (mounted at "+sourceMountPath+")")
	flag.Int64Var(&cfg.pvcSize, "pvc-size", 0, "Size of the source disk (bytes)")
	flag.StringVar(&cfg.crName, "cr-name", "", "Custom Resource name")
	flag.StringVar(&cfg.crNS, "cr-namespace", "", "Custom Resource namespace")
	flag.StringVar(&cfg.ownerUID, "owner-uid", "", "Owner UID (target PVC UID), used as the progress label")
	// Accepted for compatibility with the populator machinery (which always
	// passes it and wires the secret via EnvFrom); NFS needs no credentials.
	flag.StringVar(&cfg.secretName, "secret-name", "", "Secret name (unused; NFS is unauthenticated)")
	flag.Parse()

	if cfg.volumePath == "" || cfg.sourcePath == "" {
		klog.Fatal("volume-path and source-path are required")
	}

	certsDir, err := os.MkdirTemp("", "certsdir")
	if err != nil {
		klog.Fatal(err)
	}
	metrics.StartPrometheusEndpoint(certsDir)

	progress := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nutanix_nfs_populator_progress",
			Help: "Percentage of the Nutanix disk copied over NFS",
		},
		[]string{"ownerUID"},
	)
	if err := prometheus.Register(progress); err != nil {
		klog.Error("progress gauge not registered: ", err)
	}

	src := filepath.Join(sourceMountPath, cfg.sourcePath)
	klog.Infof("Copying %s -> %s", src, cfg.volumePath)

	if err := convert(src, cfg.volumePath, cfg.ownerUID, progress); err != nil {
		klog.Fatal(err)
	}
	progress.WithLabelValues(cfg.ownerUID).Set(100)
	klog.Info("Finished populating the volume. Progress: 100%")
}

// convert runs qemu-img, streaming its -p progress into the Prometheus gauge.
// The source is a local file on the kubelet-mounted NFS export (read-only);
// the target is written as raw. The source format is probed by qemu-img.
//
// Image locking is disabled on the source only: qemu takes an fcntl lock even
// with -U, and on the NFS mount (which cannot use nolock) that becomes an NLM
// RPC that can block forever on a hard mount. The source is an immutable
// catalog-image clone that nothing else locks, so the lock protects nothing.
func convert(src, dst, ownerUID string, progress *prometheus.GaugeVec) error {
	// Commas are option separators in --image-opts and must be doubled.
	srcOpts := "file.driver=file,file.locking=off,file.filename=" + strings.ReplaceAll(src, ",", ",,")
	// #nosec G204 - args are controller-provided populator spec fields.
	cmd := exec.Command("/usr/bin/qemu-img", "convert", "-p", "-O", "raw", "--image-opts", srcOpts, dst)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Split(scanProgress)
	for scanner.Scan() {
		if m := progressRE.FindStringSubmatch(scanner.Text()); m != nil {
			if pct, perr := strconv.ParseFloat(m[1], 64); perr == nil {
				progress.WithLabelValues(ownerUID).Set(pct)
				klog.Infof("Progress: %.2f%%", pct)
			}
		}
	}
	return cmd.Wait()
}

// scanProgress splits qemu-img output on both newlines and carriage returns so
// each progress update (which qemu-img emits with \r) is a separate token.
func scanProgress(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// nutanix-nfs-populator reads a Nutanix VM disk directly from the Prism
// Element storage container over NFS and writes it into the target volume,
// bypassing the Prism REST Image Service (and its HTTP 429 rate limiting).
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
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/kubev2v/forklift/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

type config struct {
	volumePath string
	serverIP   string
	export     string
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
	flag.StringVar(&cfg.serverIP, "server-ip", "", "NFS server IP (Prism Element cluster VIP)")
	flag.StringVar(&cfg.export, "export", "", "NFS export (storage container name)")
	flag.StringVar(&cfg.sourcePath, "source-path", "", "Path to the disk file within the export")
	flag.Int64Var(&cfg.pvcSize, "pvc-size", 0, "Size of the source disk (bytes)")
	flag.StringVar(&cfg.crName, "cr-name", "", "Custom Resource name")
	flag.StringVar(&cfg.crNS, "cr-namespace", "", "Custom Resource namespace")
	flag.StringVar(&cfg.ownerUID, "owner-uid", "", "Owner UID (target PVC UID), used as the progress label")
	// Accepted for compatibility with the populator machinery (which always
	// passes it and wires the secret via EnvFrom); NFS needs no credentials.
	flag.StringVar(&cfg.secretName, "secret-name", "", "Secret name (unused; NFS is unauthenticated)")
	flag.Parse()

	if cfg.volumePath == "" || cfg.serverIP == "" || cfg.export == "" || cfg.sourcePath == "" {
		klog.Fatal("volume-path, server-ip, export and source-path are required")
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

	src := fmt.Sprintf("nfs://%s/%s/%s", cfg.serverIP, cfg.export, cfg.sourcePath)
	klog.Infof("Copying %s -> %s", src, cfg.volumePath)

	if err := convert(src, cfg.volumePath, cfg.ownerUID, progress); err != nil {
		klog.Fatal(err)
	}
	progress.WithLabelValues(cfg.ownerUID).Set(100)
	klog.Info("Finished populating the volume. Progress: 100%")
}

// convert runs qemu-img, streaming its -p progress into the Prometheus gauge.
// The source is read over NFS via qemu-img's built-in libnfs driver (userspace,
// unprivileged); the target is written as raw.
func convert(src, dst, ownerUID string, progress *prometheus.GaugeVec) error {
	// #nosec G204 - args are controller-provided populator spec fields.
	cmd := exec.Command("qemu-img", "convert", "-p", "-O", "raw", src, dst)
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

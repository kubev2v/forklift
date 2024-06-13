package main

import (
	"bufio"
	"bytes"
	"flag"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/konveyor/forklift-controller/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"k8s.io/klog/v2"
)

func main() {
	var srcVolPath, dstVolPath, srcFormat, dstFormat, volumeMode, ownerUID string

	flag.StringVar(&srcVolPath, "src-path", "", "Source volume path")
	flag.StringVar(&dstVolPath, "dst-path", "", "Target volume path")
	flag.StringVar(&srcFormat, "src-format", "", "Format of the source volume")
	flag.StringVar(&dstFormat, "dst-format", "", "Format of the target volume")
	flag.StringVar(&volumeMode, "volume-mode", "", "Format of the target volume")
	flag.StringVar(&ownerUID, "owner-uid", "", "Owner UID (usually PVC UID)")

	flag.Parse()

	klog.Info("srcVolPath: ", srcVolPath, " dstVolPath: ", dstVolPath, " sourceFormat: ", srcFormat, " targetFormat: ", dstFormat)

	certsDirectory, err := os.MkdirTemp("", "certsdir")
	if err != nil {
		klog.Fatal(err)
	}

	metrics.StartPrometheusEndpoint(certsDirectory)

	err = convert(srcVolPath, dstVolPath, srcFormat, dstFormat, volumeMode, ownerUID)
	if err != nil {
		klog.Fatal(err)
	}
}

func convert(srcVolPath, dstVolPath, srcFormat, dstFormat, volumeMode, ownerUID string) error {
	err := qemuimgConvert(srcVolPath, dstVolPath, srcFormat, dstFormat, ownerUID)
	if err != nil {
		return err
	}

	klog.Info("Copying over source")

	// Copy dst over src
	switch volumeMode {
	case "Block":
		err = qemuimgConvert(dstVolPath, srcVolPath, dstFormat, dstFormat, ownerUID, "-W")
		if err != nil {
			return err
		}
	case "Filesystem":
		// Use mv for files as it's faster than qemu-img convert
		cmd := exec.Command("mv", dstVolPath, srcVolPath)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr // Capture stderr
		klog.Info("Executing command: ", cmd.String())
		err := cmd.Run()
		if err != nil {
			klog.Error(stderr.String())
			return err
		}
	}

	return nil
}

func qemuimgConvert(srcVolPath, dstVolPath, srcFormat, dstFormat, ownerUID string, additionalArgs ...string) error {
	cmd := exec.Command(
		"qemu-img",
		"convert",
		"-p",
		"-f", srcFormat,
		"-O", dstFormat,
		srcVolPath,
		dstVolPath,
	)

	if len(additionalArgs) > 0 {
		cmd.Args = append(cmd.Args, additionalArgs...)
	}

	klog.Info("Executing command: ", cmd.String())
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			klog.Error(line)
		}
	}()

	scanner := bufio.NewScanner(stdout)
	go func() {
		progressVec := createProgressCounter()
		for scanner.Scan() {
			line := scanner.Text()
			klog.Info(line)
			progress, err := parseQemuimgProgress(line)
			if err != nil {
				klog.Error(err)
				continue
			}

			updateProgress(progressVec, ownerUID, progress)
		}
	}()

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}

func updateProgress(progressVec *prometheus.CounterVec, ownerUID string, progress float64) {
	if progress == 0 {
		return
	}
	metric := &dto.Metric{}
	if err := progressVec.WithLabelValues(ownerUID).Write(metric); err != nil {
		klog.Errorf("updateProgress: failed to write metric; %v", err)
	}

	if progress > *metric.Counter.Value {
		progressVec.WithLabelValues(ownerUID).Add(progress - *metric.Counter.Value)
	}
}

func parseQemuimgProgress(line string) (float64, error) {
	// Parse qemu-img progress
	// Example: "(10.00/100%)"
	trimmed := strings.Trim(line, "\r\n\t")
	if strings.HasSuffix(trimmed, "100%)") {
		start := strings.Index(trimmed, "(") + 1
		end := strings.Index(trimmed, "/")

		if start < end && end <= len(trimmed) {
			progressStr := trimmed[start:end]
			progress, err := strconv.ParseFloat(progressStr, 64)
			if err != nil {
				return 0, err
			}
			return progress, nil
		}
	}

	return 0, nil
}

func createProgressCounter() *prometheus.CounterVec {
	progressVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "image_converter_progress",
			Help: "Progress of image conversion",
		},
		[]string{"ownerUID"},
	)

	if err := prometheus.Register(progressVec); err != nil {
		klog.Error("Prometheus progress counter not registered:", err)
	}

	return progressVec
}

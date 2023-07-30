package main

import (
	"bufio"
	"flag"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"k8s.io/klog/v2"
)

var COPY_DISK_RE = regexp.MustCompile(`^.*Copying disk (\d+)/(\d+)`)
var DISK_PROGRESS_RE = regexp.MustCompile(`^..\s*(\d+)% \[.*\]`)
var FINISHED_RE = regexp.MustCompile(`^\[[ .0-9]*\] Finishing off`)

// Here is a scan function that imposes limit on returned line length. virt-v2v
// writes some overly long lines that don't fit into the internal buffer of
// Scanner. We could just provide bigger buffer, but it is hard to guess what
// size is large enough. Instead we just claim that line ends when it reaches
// buffer size.
func LimitedScanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	advance, token, err = bufio.ScanLines(data, atEOF)
	if token != nil || err != nil {
		return
	}
	if len(data) == bufio.MaxScanTokenSize {
		// Line is too long for the buffer. Trim it.
		advance = len(data)
		token = data
	}
	return
}

func updateProgress(progressCounter *prometheus.CounterVec, disk, progress uint64) (err error) {
	if disk == 0 {
		return
	}

	label := strconv.FormatUint(disk, 10)

	var m = &dto.Metric{}
	if err = progressCounter.WithLabelValues(label).Write(m); err != nil {
		return
	}
	previous_progress := m.Counter.GetValue()

	change := float64(progress) - previous_progress
	if change > 0 {
		klog.Infof("Progress changed for disk %d about %v", disk, change)
		progressCounter.WithLabelValues(label).Add(change)
	}
	return
}

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	flag.Parse()

	// Start prometheus metrics HTTP handler
	klog.Info("Setting up prometheus endpoint :2112/metrics")
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":2112", nil)

	progressCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "v2v",
			Name:      "disk_transfers",
			Help:      "Percent of disk copied",
		},
		[]string{"disk_id"},
	)
	if err := prometheus.Register(progressCounter); err != nil {
		// Exit gracefully if we fail here. We don't need monitoring
		// failures to hinder guest conversion.
		klog.Error("Prometheus progress counter not registered:", err)
		return
	}
	klog.Info("Prometheus progress counter registered.")

	var diskNumber uint64 = 0
	var disks uint64 = 0
	var progress uint64 = 0

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(LimitedScanLines)
	for scanner.Scan() {
		line := scanner.Bytes()
		os.Stdout.Write(line)
		os.Stdout.Write([]byte("\n"))
		err := scanner.Err()
		if err != nil {
			klog.Fatal("Output monitoring failed! ", err)
		}

		if match := COPY_DISK_RE.FindSubmatch(line); match != nil {
			diskNumber, _ = strconv.ParseUint(string(match[1]), 10, 0)
			disks, _ = strconv.ParseUint(string(match[2]), 10, 0)
			klog.Infof("Copying disk %d out of %d", diskNumber, disks)
			progress = 0
			err = updateProgress(progressCounter, diskNumber, progress)
		} else if match := DISK_PROGRESS_RE.FindSubmatch(line); match != nil {
			progress, _ = strconv.ParseUint(string(match[1]), 10, 0)
			klog.Infof("Progress update, completed %d %%", progress)
			err = updateProgress(progressCounter, diskNumber, progress)
		} else if match := FINISHED_RE.Find(line); match != nil {
			// Make sure we flag conversion as finished. This is
			// just in case we miss the last progress update for some reason.
			klog.Infof("Finished")
			for disk := uint64(0); disk < disks; disk++ {
				err = updateProgress(progressCounter, disk, 100)
			}
		} else {
			klog.V(1).Info("Ignoring line: ", string(line))
		}
		if err != nil {
			// Don't make processing errors fatal.
			klog.Error("Error updating progress: ", err)
			err = nil
		}
	}
	err := scanner.Err()
	if err != nil {
		klog.Fatal("Output monitoring failed! ", err)
	}
}

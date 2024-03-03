package main

import (
	"bufio"
	"bytes"
	"flag"
	"os/exec"

	"k8s.io/klog/v2"
)

func main() {
	var srcVolPath, dstVolPath, srcFormat, dstFormat, volumeMode string

	flag.StringVar(&srcVolPath, "src-path", "", "Source volume path")
	flag.StringVar(&dstVolPath, "dst-path", "", "Target volume path")
	flag.StringVar(&srcFormat, "src-format", "", "Format of the source volume")
	flag.StringVar(&dstFormat, "dst-format", "", "Format of the target volume")
	flag.StringVar(&volumeMode, "volume-mode", "", "Format of the target volume")

	flag.Parse()

	klog.Info("srcVolPath: ", srcVolPath, " dstVolPath: ", dstVolPath, " sourceFormat: ", srcFormat, " targetFormat: ", dstFormat)
	err := convert(srcVolPath, dstVolPath, srcFormat, dstFormat, volumeMode)
	if err != nil {
		klog.Fatal(err)
	}
}

func convert(srcVolPath, dstVolPath, srcFormat, dstFormat, volumeMode string) error {
	err := qemuimgConvert(srcVolPath, dstVolPath, srcFormat, dstFormat)
	if err != nil {
		return err
	}

	klog.Info("Copying over source")

	// Copy dst over src
	switch volumeMode {
	case "Block":
		err = qemuimgConvert(dstVolPath, srcVolPath, dstFormat, dstFormat)
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

func qemuimgConvert(srcVolPath, dstVolPath, srcFormat, dstFormat string) error {
	cmd := exec.Command(
		"qemu-img",
		"convert",
		"-p",
		"-f", srcFormat,
		"-O", dstFormat,
		srcVolPath,
		dstVolPath,
	)

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
	for scanner.Scan() {
		line := scanner.Text()
		klog.Info(line)
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}

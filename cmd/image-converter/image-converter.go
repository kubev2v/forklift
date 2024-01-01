package main

import (
	"bufio"
	"flag"
	"os/exec"

	"k8s.io/klog/v2"
)

func main() {
	var srcVolPath, dstVolPath, srcFormat, dstFormat string

	flag.StringVar(&srcVolPath, "src-path", "", "Source volume path")
	flag.StringVar(&dstVolPath, "dst-path", "", "Target volume path")
	flag.StringVar(&srcFormat, "src-format", "", "Format of the source volume")
	flag.StringVar(&dstFormat, "dst-format", "", "Format of the target volume")

	flag.Parse()

	klog.Info("srcVolPath: ", srcVolPath, " dstVolPath: ", dstVolPath, " sourceFormat: ", srcFormat, " targetFormat: ", dstFormat)
	err := convert(srcVolPath, dstVolPath, srcFormat, dstFormat)
	if err != nil {
		klog.Fatal(err)
	}
}

func convert(srcVolPath, dstVolPath, sourceFormat, targetFormat string) error {
	cmd := exec.Command(
		"qemu-img",
		"convert",
		"-p",
		"-f", sourceFormat,
		"-O", targetFormat,
		srcVolPath,
		dstVolPath,
	)

	klog.Info("Executing command: ", cmd.String())
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		klog.Info(line)
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	// Replace /mnt/disk.img with /output/disk.img
	cmd = exec.Command(
		"mv",
		dstVolPath,
		srcVolPath,
	)

	if err := cmd.Start(); err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const apiGroup = "forklift.konveyor.io"

func main() {
	var srcVolPath, dstVolPath, sourceFormat, targetFormat, pvcName, namespace, command string

	flag.StringVar(&srcVolPath, "src-path", "", "Source volume path")
	flag.StringVar(&dstVolPath, "dst-path", "", "Target volume path")
	flag.StringVar(&sourceFormat, "src-format", "", "Format of the source volume")
	flag.StringVar(&targetFormat, "target-format", "", "Format of the target volume")
	flag.StringVar(&pvcName, "pvc-name", "", "Name of PVC to measure")
	flag.StringVar(&namespace, "namespace", "", "Namespace of PVC to measure")
	flag.StringVar(&command, "command", "", "Command to run (convert, measure)")

	flag.Parse()

	klog.Info("Running command: ", command)

	switch command {
	case "convert":
		err := convert(srcVolPath, dstVolPath, sourceFormat, targetFormat)
		if err != nil {
			klog.Fatal(err)
		}
	case "measure":
		requiredSize, err := measure(srcVolPath, sourceFormat, targetFormat)
		if err != nil {
			klog.Fatal(err)
		}

		err = annotatePvc(pvcName, namespace, requiredSize)
		if err != nil {
			klog.Fatal(err)
		}
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

	return nil
}

func measure(srcVolPath, targetFormat, sourceFormat string) (int64, error) {
	cmd := exec.Command(
		"qemu-img",
		"measure",
		"-O", targetFormat,
		"-f", sourceFormat,
		srcVolPath,
		"--output", "json",
	)

	klog.Info("Executing command: ", cmd.String())

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}

	if err := cmd.Start(); err != nil {
		return -1, err
	}

	outputBytes, err := io.ReadAll(stdout)
	if err != nil {
		return -1, err
	}

	type MeasureOutput struct {
		Required       int64 `json:"required"`
		FullyAllocated int64 `json:"fully-allocated"`
	}

	var measure MeasureOutput
	if err := json.Unmarshal(outputBytes, &measure); err != nil {
		return -1, err
	}

	klog.Info("output: ", measure)

	return measure.Required, nil
}

func annotatePvc(pvcName, namespace string, required int64) error {
	cfg := ctrl.GetConfigOrDie()
	c, err := client.New(cfg, client.Options{})
	if err != nil {
		return err
	}

	pvc := &corev1.PersistentVolumeClaim{}
	err = c.Get(context.TODO(), client.ObjectKey{
		Namespace: namespace,
		Name:      pvcName,
	}, pvc)
	if err != nil {
		return err
	}

	patch := client.MergeFrom(pvc.DeepCopy())

	if pvc.Annotations == nil {
		pvc.Annotations = map[string]string{}
	}
	pvc.Annotations[fmt.Sprintf("%s/%s", apiGroup, "required-size")] = strconv.FormatInt(required, 10)

	err = c.Patch(context.TODO(), pvc, patch)
	if err != nil {
		return err
	}

	return nil
}

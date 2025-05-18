package testplan

import (
	"certificate-tool/internal/k8s"
	"certificate-tool/internal/utils"
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// TestCase defines a single test scenario.
type TestCase struct {
	Name    string                `yaml:"name"`
	Success utils.SuccessCriteria `yaml:"success"`
	VMs     []*utils.VM           `yaml:"vms"`
	Results utils.TestResult      `yaml:"results"`
}

// Run provisions per-pod PVCs, VMs, launches populator pods, and waits.
func (tc *TestCase) Run(ctx context.Context, clientset *kubernetes.Clientset, namespace, podImage, vmImage, storageClassName, pvcYamlPath, storageVendorProduct string) error {
	if err := ensureVMs(tc.Name, vmImage, tc.VMs); err != nil {
		return fmt.Errorf("VM setup failed: %w", err)
	}

	for _, vm := range tc.VMs {
		pvcName := fmt.Sprintf("pvc-%s-%s", tc.Name, vm.NamePrefix)
		if err := k8s.ApplyPVCFromTemplate(clientset, namespace, pvcName, vm.Size, storageClassName, pvcYamlPath); err != nil {
			return fmt.Errorf("failed ensuring PVC %s: %w", pvcName, err)
		}

		podName := fmt.Sprintf("populator-%s-%s", tc.Name, vm.NamePrefix)
		if err := k8s.EnsurePopulatorPod(ctx, clientset, namespace, podName, podImage, tc.Name, *vm, storageVendorProduct, pvcName); err != nil {
			return fmt.Errorf("failed creating pod %s: %w", podName, err)
		}
	}

	newCtx, _ := context.WithTimeout(ctx, 10*time.Minute)
	results, totalTime, err := k8s.PollPodsAndCheck(newCtx, clientset, namespace, fmt.Sprintf("test=%s", tc.Name), tc.Success.MaxTimeSeconds, 5*time.Second, time.Duration(tc.Success.MaxTimeSeconds)*time.Second)
	if err != nil {
		return fmt.Errorf("failed polling pods: %w", err)
	}
	for _, r := range results {
		tc.Results.Success = r.Success
		tc.Results.ElapsedTime = int64(totalTime.Seconds())
		if !r.Success {
			tc.Results.FailureReason = fmt.Sprintln(results)
		}

	}
	return nil
}

// ensureVMs is a placeholder for clone/create logic.
func ensureVMs(testName, vmImage string, vms []*utils.VM) error {
	klog.Infof("Ensuring VMs for test %s", testName)
	// TODO: implement actual clone/create
	for _, vm := range vms {
		vm.VmdkPath = "[eco-iscsi-ds1] vmtemptest/vmtemptest.vmdk"
	}
	return nil
}

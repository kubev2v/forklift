package testplan

import (
	"certificate-tool/internal/k8s"
	"certificate-tool/internal/utils"
	"context"
	"fmt"

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
func (tc *TestCase) Run(ctx context.Context, clientset *kubernetes.Clientset, namespace, image, storageClassName, pvcYamlPath, storageVendorProduct string) error {
	if err := ensureVM(tc.Name, tc.VMs); err != nil {
		return fmt.Errorf("VM setup failed: %w", err)
	}

	for _, vm := range tc.VMs {
		pvcName := fmt.Sprintf("pvc-%s-%s", tc.Name, vm.NamePrefix)
		if err := k8s.ApplyPVCFromTemplate(clientset, namespace, pvcName, vm.Size, storageClassName, pvcYamlPath); err != nil {
			return fmt.Errorf("failed ensuring PVC %s: %w", pvcName, err)
		}

		podName := fmt.Sprintf("populator-%s-%s", tc.Name, vm.NamePrefix)
		if err := k8s.EnsurePopulatorPod(ctx, clientset, namespace, podName, image, tc.Name, *vm, pvcName, storageVendorProduct); err != nil {
			return fmt.Errorf("failed creating pod %s: %w", podName, err)
		}
	}

	// 3. TODO: Poll pods & check exit codes against tc.Success.MaxTimeSeconds
	return nil
}

// ensureVM is a placeholder for clone/create logic.
func ensureVM(testName string, vms []*utils.VM) error {
	klog.Infof("Ensuring VMs for test %s", testName)
	// TODO: implement actual clone/create
	return nil
}

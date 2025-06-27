package testplan

import (
	"certificate-tool/internal/k8s"
	"certificate-tool/internal/utils"
	"certificate-tool/pkg/vmware"
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type TestCaseForPrint struct {
	Name    string                `yaml:"name"`
	Success utils.SuccessCriteria `yaml:"success"`
	VMs     []*utils.VM           `yaml:"vms"`
	Results utils.TestResult      `yaml:"results"`
}

// TestCase defines a single test scenario.
type TestCase struct {
	Name                  string                       `yaml:"name"`
	Success               utils.SuccessCriteria        `yaml:"success"`
	VMs                   []*utils.VM                  `yaml:"vms"`
	IndividualTestResults []utils.IndividualTestResult `yaml:"individualTestResults"`
	ResultSummary         utils.TestResult             `yaml:"resultsummary"`
	Namespace             string                       `yaml:"-"`
	StorageClass          string                       `yaml:"-"`
	ClientSet             *kubernetes.Clientset        `yaml:"-"`
	VSphereURL            string                       `yaml:"-"`
	VSphereUser           string                       `yaml:"-"`
	VSpherePassword       string                       `yaml:"-"`
	Datacenter            string                       `yaml:"-"`
	Datastore             string                       `yaml:"-"`
	ResourcePool          string                       `yaml:"-"`
	HostName              string                       `yaml:"-"`
	VmdkDownloadURL       string                       `yaml:"-"`
	LocalVmdkPath         string                       `yaml:"localVmdkPath"`
	IsoPath               string                       `yaml:"-"`
}

// Run provisions per-pod PVCs, VMs, launches populator pods, and waits.
func (tc *TestCase) Run(ctx context.Context, podImage, pvcYamlPath, storageVendorProduct string) error {
	_, cancel, _, _, _, _, _, err := vmware.SetupVSphere(
		10*time.Minute,
		tc.VSphereURL,
		tc.VSphereUser,
		tc.VSpherePassword,
		tc.Datacenter,
		tc.Datastore,
		tc.ResourcePool,
	)
	if err != nil {
		return fmt.Errorf("vSphere setup failed: %w", err)
	}
	defer cancel()

	if err := tc.ensureVMs(tc.Name, tc.VMs, tc.VmdkDownloadURL, tc.LocalVmdkPath, tc.IsoPath); err != nil {
		return fmt.Errorf("VM setup failed: %w", err)
	}

	for _, vm := range tc.VMs {
		pvcName := fmt.Sprintf("pvc-%s-%s", tc.Name, vm.NamePrefix)
		if err := k8s.ApplyPVCFromTemplate(tc.ClientSet, tc.Namespace, pvcName, vm.Size, tc.StorageClass, pvcYamlPath); err != nil {
			return fmt.Errorf("failed ensuring PVC %s: %w", pvcName, err)
		}

		podName := fmt.Sprintf("populator-%s-%s", tc.Name, vm.NamePrefix)
		if err := k8s.EnsurePopulatorPod(ctx, tc.ClientSet, tc.Namespace, podName, podImage, tc.Name, *vm, storageVendorProduct, pvcName); err != nil {
			return fmt.Errorf("failed creating pod %s: %w", podName, err)
		}
	}

	newCtx, _ := context.WithTimeout(ctx, 10*time.Minute)
	results, _, err := k8s.PollPodsAndCheck(newCtx, tc.ClientSet, tc.Namespace, fmt.Sprintf("test=%s", tc.Name), tc.Success.MaxTimeSeconds, 5*time.Second, time.Duration(tc.Success.MaxTimeSeconds)*time.Second)
	if err != nil {
		return fmt.Errorf("failed polling pods: %w", err)
	}
	tc.ResultSummary.Success = true
	for _, r := range results {
		newTcResult := utils.IndividualTestResult{
			PodName:     r.PodName,
			Success:     r.Success,
			ElapsedTime: int64(r.Duration.Seconds()),
		}
		if newTcResult.Success != true {
			newTcResult.FailureReason = fmt.Sprintf("Err: %s, ExitCode: %d", r.Err, r.ExitCode)

			const logLinesToFetch = 10
			logs, logErr := k8s.GetPodLogs(newCtx, tc.ClientSet, tc.Namespace, r.PodName, logLinesToFetch)
			if logErr != nil {
				newTcResult.LogLines = fmt.Sprintf("Failed to get logs: %v", logErr)
				fmt.Printf("Warning: Could not get logs for pod %s/%s: %v\n", tc.Namespace, r.PodName, logErr)
			} else {
				newTcResult.LogLines = logs
			}
		}

		tc.IndividualTestResults = append(tc.IndividualTestResults, newTcResult)
		tc.ResultSummary.Success = tc.ResultSummary.Success && r.Success
		if !r.Success {
			tc.ResultSummary.FailureReason = fmt.Sprintf("%s Pod: %s, err: %s; code: %d", tc.ResultSummary.FailureReason, r.PodName, r.Err, r.ExitCode)
		}
	}
	return nil
}

// ensureVMs creates VMs and sets their VMDK paths.
func (tc *TestCase) ensureVMs(testName string, vms []*utils.VM, downloadVmdkURL, tcLocalVmdkPath, isoPath string) error {
	klog.Infof("Ensuring VMs for test %s", testName)
	for _, vm := range vms {
		vm.Name = fmt.Sprintf("%s-%s", testName, vm.NamePrefix)
		localVmdkPath := tcLocalVmdkPath
		if vm.LocalVmdkPath != "" {
			localVmdkPath = vm.LocalVmdkPath
		}

		klog.Infof("Creating VM %s with image %s, VMDK URL: %s, Local VMDK Path: %s, ISO Path: %s", fullVMName, downloadVmdkURL, localVmdkPath, isoPath)
		remoteVmdkPath, err := vmware.CreateVM(
			vm.Name,
			tc.VSphereURL,
			tc.VSphereUser,
			tc.VSpherePassword,
			tc.Datacenter,
			tc.Datastore,
			tc.ResourcePool,
			tc.HostName,
			downloadVmdkURL,
			localVmdkPath,
			isoPath,
			10*time.Minute,
		)
		if err != nil {
			return fmt.Errorf("failed to create VM %s: %w", vm.Name, err)
		}
		vm.VmdkPath = remoteVmdkPath
		klog.Infof("VM %s created with VMDK path: %s", vm.Name, vm.VmdkPath)
	}
	return nil
}

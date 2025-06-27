package utils

// VM represents a VM configuration to be provisioned.
type VM struct {
	// this name is known during the test case execution and is not serializable
	// perhaps we set that in the test case yaml to specify existing VMs?
	Name          string
	NamePrefix    string `yaml:"namePrefix"`
	Size          string `yaml:"size"`
	VmdkPath      string `yaml:"vmdkPath"`
	LocalVmdkPath string `yaml:"localVmdkPath"`
}

// SuccessCriteria indicates the max allowed run time for a test case.
type SuccessCriteria struct {
	MaxTimeSeconds int `yaml:"maxTimeSeconds"`
}

// TestResult holds the outcome of a test case.
type TestResult struct {
	Success       bool   `yaml:"success"`
	ElapsedTime   int64  `yaml:"elapsedTime"`
	FailureReason string `yaml:"failureReason"`
}
type IndividualTestResult struct {
	PodName       string `yaml:"name"`
	Success       bool   `yaml:"success"`
	ElapsedTime   int64  `yaml:"elapsedTime"`
	FailureReason string `yaml:"failureReason"`
	LogLines      string `yaml:"logLines"`
}

package utils

// VM represents a VM configuration to be provisioned.
type VM struct {
	NamePrefix string `yaml:"name_prefix"`
	Size       string `yaml:"size"`
	VmdkPath   string `yaml:"vmkd_path"`
}

// SuccessCriteria indicates the max allowed run time for a test case.
type SuccessCriteria struct {
	MaxTimeSeconds int `yaml:"max_time_seconds"`
}

// TestResult holds the outcome of a test case.
type TestResult struct {
	Success       bool   `yaml:"success"`
	ElapsedTime   int64  `yaml:"elapsed_time"`
	FailureReason string `yaml:"failure_reason"`
}

package diagnostics

// DiagnosticsReport holds the complete diagnostics for a migration plan.
type DiagnosticsReport struct {
	PlanName       string
	PlanUID        string
	MigrationName  string
	MigrationUID   string
	TargetNS       string
	CutoverTime    string
	RemoteTarget   bool
	VMs            []VMDiagnostics
	Config         ConfigContext
	ControllerLogs []ControllerLogEntry
}

// VMDiagnostics holds diagnostics for a single VM in a migration.
type VMDiagnostics struct {
	Name       string
	ID         string
	Phase      string
	Error      string
	Conditions []ConditionEntry
	StepErrors []StepError
	Pods       []PodDiagnostics
	Conversion *ConversionInfo
	Events     []EventEntry
}

// PodDiagnostics holds log analysis and status for a migration workload pod.
type PodDiagnostics struct {
	Name       string
	Phase      string // Running, Succeeded, Failed, Evicted
	Reason     string
	Container  string
	LogTail    []string
	ErrorLines []string // Significant error lines from the full log scan
	ErrorCount int
	WarnCount  int
}

// ConversionInfo holds data from a Conversion CR linked to a VM.
type ConversionInfo struct {
	Name    string
	Phase   string
	Message string
	PodName string
}

// EventEntry holds a single Kubernetes event relevant to a migration.
type EventEntry struct {
	Type    string // Normal, Warning
	Reason  string
	Object  string
	Message string
	Age     string
}

// ConfigContext holds configuration details relevant for diagnostics.
type ConfigContext struct {
	SourceProvider string
	MigrationType  string
	VDDKImage      string
}

// ConditionEntry holds a condition from a migration VM status.
type ConditionEntry struct {
	Type    string
	Status  string
	Reason  string
	Message string
}

// StepError holds an error from a pipeline step or task.
type StepError struct {
	Step    string
	Phase   string
	Reason  string
	Message string
}

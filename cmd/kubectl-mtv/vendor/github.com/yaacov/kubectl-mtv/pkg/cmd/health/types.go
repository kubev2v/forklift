package health

import (
	"time"
)

// HealthStatus represents the overall health status
type HealthStatus string

const (
	HealthStatusHealthy  HealthStatus = "Healthy"
	HealthStatusWarning  HealthStatus = "Warning"
	HealthStatusCritical HealthStatus = "Critical"
	HealthStatusUnknown  HealthStatus = "Unknown"
)

// IssueSeverity represents the severity of a health issue
type IssueSeverity string

const (
	SeverityCritical IssueSeverity = "Critical"
	SeverityWarning  IssueSeverity = "Warning"
	SeverityInfo     IssueSeverity = "Info"
)

// HealthReport contains the complete health check results
type HealthReport struct {
	Timestamp       time.Time        `json:"timestamp" yaml:"timestamp"`
	OverallStatus   HealthStatus     `json:"overallStatus" yaml:"overallStatus"`
	Operator        OperatorHealth   `json:"operator" yaml:"operator"`
	Controller      ControllerHealth `json:"controller" yaml:"controller"`
	Pods            []PodHealth      `json:"pods" yaml:"pods"`
	LogAnalysis     []LogAnalysis    `json:"logAnalysis,omitempty" yaml:"logAnalysis,omitempty"`
	Providers       []ProviderHealth `json:"providers" yaml:"providers"`
	Plans           []PlanHealth     `json:"plans" yaml:"plans"`
	Issues          []HealthIssue    `json:"issues" yaml:"issues"`
	Recommendations []string         `json:"recommendations" yaml:"recommendations"`
	Summary         HealthSummary    `json:"summary" yaml:"summary"`
}

// HealthSummary provides a quick overview of the health report
type HealthSummary struct {
	TotalPods        int `json:"totalPods" yaml:"totalPods"`
	HealthyPods      int `json:"healthyPods" yaml:"healthyPods"`
	TotalProviders   int `json:"totalProviders" yaml:"totalProviders"`
	HealthyProviders int `json:"healthyProviders" yaml:"healthyProviders"`
	TotalPlans       int `json:"totalPlans" yaml:"totalPlans"`
	HealthyPlans     int `json:"healthyPlans" yaml:"healthyPlans"`
	TotalIssues      int `json:"totalIssues" yaml:"totalIssues"`
	CriticalIssues   int `json:"criticalIssues" yaml:"criticalIssues"`
	WarningIssues    int `json:"warningIssues" yaml:"warningIssues"`
}

// OperatorHealth contains operator health information
type OperatorHealth struct {
	Installed bool   `json:"installed" yaml:"installed"`
	Version   string `json:"version,omitempty" yaml:"version,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Status    string `json:"status" yaml:"status"`
	Error     string `json:"error,omitempty" yaml:"error,omitempty"`
}

// ControllerHealth contains ForkliftController spec analysis
type ControllerHealth struct {
	Name                       string           `json:"name" yaml:"name"`
	Namespace                  string           `json:"namespace" yaml:"namespace"`
	Found                      bool             `json:"found" yaml:"found"`
	Error                      string           `json:"error,omitempty" yaml:"error,omitempty"`
	FeatureFlags               FeatureFlags     `json:"featureFlags" yaml:"featureFlags"`
	CustomImages               []ImageOverride  `json:"customImages,omitempty" yaml:"customImages,omitempty"`
	VDDKImage                  string           `json:"vddkImage,omitempty" yaml:"vddkImage,omitempty"`
	LogLevel                   int              `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	HasVSphereProvider         bool             `json:"hasVSphereProvider" yaml:"hasVSphereProvider"`
	HasRemoteOpenShiftProvider bool             `json:"hasRemoteOpenShiftProvider" yaml:"hasRemoteOpenShiftProvider"`
	Status                     ControllerStatus `json:"status" yaml:"status"`
}

// ControllerStatus contains ForkliftController status conditions
type ControllerStatus struct {
	Running    bool   `json:"running" yaml:"running"`
	Successful bool   `json:"successful" yaml:"successful"`
	Failed     bool   `json:"failed" yaml:"failed"`
	Message    string `json:"message,omitempty" yaml:"message,omitempty"`
}

// FeatureFlags contains ForkliftController feature flag settings
type FeatureFlags struct {
	UIPlugin         *bool `json:"uiPlugin,omitempty" yaml:"uiPlugin,omitempty"`
	Validation       *bool `json:"validation,omitempty" yaml:"validation,omitempty"`
	VolumePopulator  *bool `json:"volumePopulator,omitempty" yaml:"volumePopulator,omitempty"`
	AuthRequired     *bool `json:"authRequired,omitempty" yaml:"authRequired,omitempty"`
	OCPLiveMigration *bool `json:"ocpLiveMigration,omitempty" yaml:"ocpLiveMigration,omitempty"`
}

// ImageOverride represents a custom FQIN image override
type ImageOverride struct {
	Field string `json:"field" yaml:"field"`
	Image string `json:"image" yaml:"image"`
}

// PodHealth contains health information for a single pod
type PodHealth struct {
	Name             string   `json:"name" yaml:"name"`
	Namespace        string   `json:"namespace" yaml:"namespace"`
	Status           string   `json:"status" yaml:"status"`
	Ready            bool     `json:"ready" yaml:"ready"`
	Restarts         int      `json:"restarts" yaml:"restarts"`
	Age              string   `json:"age" yaml:"age"`
	Issues           []string `json:"issues,omitempty" yaml:"issues,omitempty"`
	TerminatedReason string   `json:"terminatedReason,omitempty" yaml:"terminatedReason,omitempty"`
}

// LogAnalysis contains log analysis results for a pod/deployment
type LogAnalysis struct {
	Name       string   `json:"name" yaml:"name"`
	Errors     int      `json:"errors" yaml:"errors"`
	Warnings   int      `json:"warnings" yaml:"warnings"`
	ErrorLines []string `json:"errorLines,omitempty" yaml:"errorLines,omitempty"`
	WarnLines  []string `json:"warnLines,omitempty" yaml:"warnLines,omitempty"`
}

// ProviderHealth contains health information for a provider
type ProviderHealth struct {
	Name             string `json:"name" yaml:"name"`
	Namespace        string `json:"namespace" yaml:"namespace"`
	Type             string `json:"type" yaml:"type"`
	Phase            string `json:"phase" yaml:"phase"`
	Ready            bool   `json:"ready" yaml:"ready"`
	Connected        bool   `json:"connected" yaml:"connected"`
	Validated        bool   `json:"validated" yaml:"validated"`
	InventoryCreated bool   `json:"inventoryCreated" yaml:"inventoryCreated"`
	Message          string `json:"message,omitempty" yaml:"message,omitempty"`
}

// PlanHealth contains health information for a migration plan
type PlanHealth struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Ready     bool   `json:"ready" yaml:"ready"`
	Status    string `json:"status" yaml:"status"`
	VMCount   int    `json:"vmCount" yaml:"vmCount"`
	Failed    int    `json:"failed,omitempty" yaml:"failed,omitempty"`
	Succeeded int    `json:"succeeded,omitempty" yaml:"succeeded,omitempty"`
	Message   string `json:"message,omitempty" yaml:"message,omitempty"`
}

// HealthIssue represents a detected health issue
type HealthIssue struct {
	Severity   IssueSeverity `json:"severity" yaml:"severity"`
	Component  string        `json:"component" yaml:"component"`
	Resource   string        `json:"resource,omitempty" yaml:"resource,omitempty"`
	Message    string        `json:"message" yaml:"message"`
	Suggestion string        `json:"suggestion,omitempty" yaml:"suggestion,omitempty"`
}

// HealthCheckOptions contains options for running health checks
type HealthCheckOptions struct {
	Namespace     string
	AllNamespaces bool
	CheckLogs     bool
	LogLines      int
	Verbose       bool
}

// NewHealthReport creates a new health report with initial values
func NewHealthReport() *HealthReport {
	return &HealthReport{
		Timestamp:       time.Now(),
		OverallStatus:   HealthStatusUnknown,
		Pods:            []PodHealth{},
		LogAnalysis:     []LogAnalysis{},
		Providers:       []ProviderHealth{},
		Plans:           []PlanHealth{},
		Issues:          []HealthIssue{},
		Recommendations: []string{},
	}
}

// AddIssue adds a health issue to the report
func (r *HealthReport) AddIssue(severity IssueSeverity, component, resource, message, suggestion string) {
	r.Issues = append(r.Issues, HealthIssue{
		Severity:   severity,
		Component:  component,
		Resource:   resource,
		Message:    message,
		Suggestion: suggestion,
	})
}

// CalculateOverallStatus determines the overall health status based on issues
func (r *HealthReport) CalculateOverallStatus() {
	hasCritical := false
	hasWarning := false

	for _, issue := range r.Issues {
		switch issue.Severity {
		case SeverityCritical:
			hasCritical = true
		case SeverityWarning:
			hasWarning = true
		}
	}

	if hasCritical {
		r.OverallStatus = HealthStatusCritical
	} else if hasWarning {
		r.OverallStatus = HealthStatusWarning
	} else {
		r.OverallStatus = HealthStatusHealthy
	}
}

// CalculateSummary calculates the summary statistics
func (r *HealthReport) CalculateSummary() {
	// Reset totals (assigned directly from slice lengths)
	r.Summary.TotalPods = len(r.Pods)
	r.Summary.TotalProviders = len(r.Providers)
	r.Summary.TotalPlans = len(r.Plans)
	r.Summary.TotalIssues = len(r.Issues)

	// Reset derived counters to zero before recomputing
	r.Summary.HealthyPods = 0
	r.Summary.HealthyProviders = 0
	r.Summary.HealthyPlans = 0
	r.Summary.CriticalIssues = 0
	r.Summary.WarningIssues = 0

	for _, pod := range r.Pods {
		if pod.Ready && pod.Status == "Running" {
			r.Summary.HealthyPods++
		}
	}

	for _, provider := range r.Providers {
		if provider.Ready {
			r.Summary.HealthyProviders++
		}
	}

	for _, plan := range r.Plans {
		if plan.Ready && plan.Status != "Failed" {
			r.Summary.HealthyPlans++
		}
	}

	for _, issue := range r.Issues {
		switch issue.Severity {
		case SeverityCritical:
			r.Summary.CriticalIssues++
		case SeverityWarning:
			r.Summary.WarningIssues++
		}
	}
}

// GenerateRecommendations generates recommendations based on issues
func (r *HealthReport) GenerateRecommendations() {
	for _, issue := range r.Issues {
		if issue.Suggestion != "" {
			recommendation := issue.Message
			if issue.Resource != "" {
				recommendation = issue.Resource + ": " + recommendation
			}
			recommendation += " - " + issue.Suggestion
			r.Recommendations = append(r.Recommendations, recommendation)
		}
	}
}

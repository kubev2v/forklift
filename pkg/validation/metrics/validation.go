// Copyright (C) 2026 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"context"
	"strconv"
	"strings"
	"time"

	validationv1alpha1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidationMetrics contains bounded Prometheus metrics for validation runs.
// It intentionally excludes target names, URLs, execution IDs, and node names
// because those labels have unbounded cardinality.
type ValidationMetrics struct {
	runs             *prometheus.CounterVec
	duration         *prometheus.HistogramVec
	queueDuration    *prometheus.HistogramVec
	queueTimeouts    *prometheus.CounterVec
	terminalErrors   *prometheus.CounterVec
	executions       *prometheus.CounterVec
	reportReductions *prometheus.CounterVec
}

// NewValidationMetrics registers validation metrics with registerer.
func NewValidationMetrics(reader client.Reader, registerer prometheus.Registerer) (*ValidationMetrics, error) {
	metrics := &ValidationMetrics{
		runs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "virt_validation_runs_total",
			Help: "Total terminal virtualization validation runs by profile and verdict.",
		}, []string{"profile", "verdict"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "virt_validation_duration_seconds",
			Help:    "Duration of terminal virtualization validation runs.",
			Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 14400, 86400},
		}, []string{"profile", "verdict"}),
		queueDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "virt_validation_queue_duration_seconds",
			Help:    "Time a virtualization validation spent waiting for its target lease.",
			Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 14400, 86400},
		}, []string{"profile", "outcome"}),
		queueTimeouts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "virt_validation_queue_timeouts_total",
			Help: "Total virtualization validations that exceeded their target lease queue timeout.",
		}, []string{"profile"}),
		terminalErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "virt_validation_terminal_errors_total",
			Help: "Total virtualization validations that reached an error phase, by bounded controller reason.",
		}, []string{"profile", "reason"}),
		executions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "virt_validation_executions_total",
			Help: "Final CTRF validation executions by profile, stable check ID, outcome, and severity.",
		}, []string{"profile", "check_id", "outcome", "severity"}),
		reportReductions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "virt_validation_report_reductions_total",
			Help: "Final CTRF reports observed by profile and report-reduction level.",
		}, []string{"profile", "level"}),
	}
	for _, collector := range []prometheus.Collector{
		metrics.runs,
		metrics.duration,
		metrics.queueDuration,
		metrics.queueTimeouts,
		metrics.terminalErrors,
		metrics.executions,
		metrics.reportReductions,
		&activeRunsCollector{reader: reader, desc: prometheus.NewDesc("virt_validation_active_runs", "Current non-terminal virtualization validation requests.", []string{"profile"}, nil)},
	} {
		if err := registerer.Register(collector); err != nil {
			return nil, err
		}
	}
	return metrics, nil
}

// ObserveQueue records a request that waited for a target lease.
func (m *ValidationMetrics) ObserveQueue(profile, outcome string, duration time.Duration) {
	if m == nil || duration < 0 {
		return
	}
	m.queueDuration.WithLabelValues(metricProfile(profile), outcome).Observe(duration.Seconds())
}

// ObserveQueueTimeout records a validation that could not acquire its target
// lease within the requested queue timeout.
func (m *ValidationMetrics) ObserveQueueTimeout(profile string) {
	if m == nil {
		return
	}
	m.queueTimeouts.WithLabelValues(metricProfile(profile)).Inc()
}

// ObserveTerminalError records controller-owned terminal error reasons. The
// reason is normalized rather than exposing arbitrary API or report text.
func (m *ValidationMetrics) ObserveTerminalError(profile, reason string) {
	if m == nil {
		return
	}
	m.terminalErrors.WithLabelValues(metricProfile(profile), terminalErrorReason(reason)).Inc()
}

// ObserveTerminal records a validation only when it enters a terminal phase.
func (m *ValidationMetrics) ObserveTerminal(validation *validationv1alpha1.VirtualizationValidation, previous validationv1alpha1.ValidationPhase) {
	if m == nil || terminal(previous) || !terminal(validation.Status.Phase) {
		return
	}
	profile := metricProfile(validation.Spec.Run.Profile)
	verdict := string(validation.Status.Verdict)
	m.runs.WithLabelValues(profile, verdict).Inc()
	if validation.Status.CompletedAt == nil {
		return
	}
	started := validation.CreationTimestamp.Time
	if validation.Status.StartedAt != nil {
		started = validation.Status.StartedAt.Time
	}
	if duration := validation.Status.CompletedAt.Sub(started).Seconds(); duration >= 0 {
		m.duration.WithLabelValues(profile, verdict).Observe(duration)
	}
}

// ObserveFinalReport records the complete CTRF execution set before status
// projection can reduce it for Kubernetes object-size limits.
func (m *ValidationMetrics) ObserveFinalReport(profile string, executions []validationv1alpha1.VirtualizationValidationExecution, reduction validationv1alpha1.ValidationReportReduction) {
	if m == nil {
		return
	}
	profile = metricProfile(profile)
	for _, execution := range executions {
		m.executions.WithLabelValues(profile, metricCheckID(execution.CheckID), string(execution.Outcome), string(execution.Severity)).Inc()
	}
	m.reportReductions.WithLabelValues(profile, reductionLevel(reduction.Level)).Inc()
}

type activeRunsCollector struct {
	reader client.Reader
	desc   *prometheus.Desc
}

var supportedProfiles = []string{"basic-v1", "basic-all-nodes-v1"}

func (c *activeRunsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *activeRunsCollector) Collect(ch chan<- prometheus.Metric) {
	validations := &validationv1alpha1.VirtualizationValidationList{}
	if err := c.reader.List(context.Background(), validations); err != nil {
		return
	}
	// Export zeroes for every supported profile. An absent series is ambiguous to
	// Prometheus consumers: it could mean no active runs, a scrape failure, or an
	// incorrectly configured controller.
	active := make(map[string]float64, len(supportedProfiles)+1)
	for _, profile := range supportedProfiles {
		active[profile] = 0
	}
	for index := range validations.Items {
		validation := &validations.Items[index]
		if validation.DeletionTimestamp.IsZero() && !terminal(validation.Status.Phase) {
			active[metricProfile(validation.Spec.Run.Profile)]++
		}
	}
	for profile, count := range active {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, count, profile)
	}
}

func terminal(phase validationv1alpha1.ValidationPhase) bool {
	return phase == validationv1alpha1.ValidationPhaseCompleted || phase == validationv1alpha1.ValidationPhaseCancelled || phase == validationv1alpha1.ValidationPhaseError
}

func metricCheckID(checkID string) string {
	if strings.HasPrefix(checkID, "unknown/") {
		return "unknown"
	}
	return checkID
}

func metricProfile(profile string) string {
	switch profile {
	case "basic-v1", "basic-all-nodes-v1":
		return profile
	default:
		return "unknown"
	}
}

func reductionLevel(level int32) string {
	return strconv.FormatInt(int64(level), 10)
}

func terminalErrorReason(reason string) string {
	switch reason {
	case "InvalidSpec", "InvalidChecks", "CredentialsNotFound", "InvalidCredentials", "QueueTimeout", "JobMissing", "IncompleteReport":
		return reason
	default:
		return "unknown"
	}
}

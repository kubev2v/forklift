package rules

import (
	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/konveyor/forklift-controller/pkg/monitoring/rules/recordingrules"
)

const (
	forkliftPrometheusRuleName = "prometheus-forklift-rules"

	prometheusLabelKey   = "prometheus.forklift.io"
	prometheusLabelValue = "true"

	k8sAppLabelKey     = "k8s-app"
	forkliftLabelValue = "forklift"
)

func SetupRules(namespace string) error {
	err := recordingrules.Register(namespace)
	if err != nil {
		return err
	}

	return nil
}

func BuildPrometheusRule(namespace string) (*promv1.PrometheusRule, error) {
	rules, err := operatorrules.BuildPrometheusRule(
		forkliftPrometheusRuleName,
		namespace,
		map[string]string{
			prometheusLabelKey: prometheusLabelValue,
			k8sAppLabelKey:     forkliftLabelValue,
		},
	)
	if err != nil {
		return nil, err
	}

	return rules, nil
}

func ListRecordingRules() []operatorrules.RecordingRule {
	return operatorrules.ListRecordingRules()
}

func ListAlerts() []promv1.Rule {
	return operatorrules.ListAlerts()
}

package recordingrules

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var plansRecordingRules = []operatorrules.RecordingRule{
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "mtv_plans_status_succeeded",
			Help: "The number of successful plans.",
		},
		MetricType: operatormetrics.GaugeType,
		Expr:       intstr.FromString("sum(mtv_workload_plans{status='succeeded'}) by (provider)"),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "mtv_plans_status_failed",
			Help: "The number of failed plans.",
		},
		MetricType: operatormetrics.GaugeType,
		Expr:       intstr.FromString("sum(mtv_workload_plans{status='failed'}) by (provider)"),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "mtv_plans_type",
			Help: "The number of plans by type.",
		},
		MetricType: operatormetrics.GaugeType,
		Expr:       intstr.FromString("sum(mtv_workload_plans_type) by (type)"),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "mtv_plans_destination",
			Help: "The number of plans by destination.",
		},
		MetricType: operatormetrics.GaugeType,
		Expr:       intstr.FromString("sum(mtv_operator_destination) by (destination)"),
	},
}

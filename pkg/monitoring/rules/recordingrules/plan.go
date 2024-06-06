package recordingrules

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var plansRecordingRules = []operatorrules.RecordingRule{
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "forklift_plans_status",
			Help: "The number of succsesfull plans.",
		},
		MetricType: operatormetrics.GaugeType,
		Expr:       intstr.FromString("count(count (forklift_plans_status) by (node))"),
	},
}

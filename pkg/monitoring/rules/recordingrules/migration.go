package recordingrules

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var migrationsRecordingRules = []operatorrules.RecordingRule{
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "mtv_plans_status",
			Help: "The number of allocatable nodes in the cluster.",
		},
		MetricType: operatormetrics.GaugeType,
		Expr:       intstr.FromString("count(count (mtv_workload_migrations) by (status))"),
	},
}

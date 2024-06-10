package recordingrules

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var migrationsRecordingRules = []operatorrules.RecordingRule{
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "mtv_migrations_status_succeeded",
			Help: "The number of successful migrations.",
		},
		MetricType: operatormetrics.GaugeType,
		Expr:       intstr.FromString("sum(mtv_workload_migrations{status='succeeded'}) by (provider)"),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "mtv_migrations_status_failed",
			Help: "The number of failed migrations.",
		},
		MetricType: operatormetrics.GaugeType,
		Expr:       intstr.FromString("sum(mtv_workload_migrations{status='failed'}) by (provider)"),
	},
}

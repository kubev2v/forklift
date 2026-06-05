package health

import (
	"github.com/yaacov/kubectl-mtv/pkg/util/describe"
)

// FormatReport formats the health report in the specified output format.
func FormatReport(report *HealthReport, outputFormat string) (string, error) {
	return describe.Format(report.ToDescription(), outputFormat)
}

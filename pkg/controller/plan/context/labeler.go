package context

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/labeler"
)

const (
	LabelMigration = "migration"
	LabelPlan      = "plan"
	LabelVM        = "vmID"
)

// TODO: comments, prune methods
type Labeler struct {
	*Context
	labeler.Labeler
}

func (r *Labeler) PlanLabels() map[string]string {
	return map[string]string{
		LabelPlan: string(r.Plan.GetUID()),
	}
}

func (r *Labeler) MigrationLabels() map[string]string {
	return map[string]string{
		LabelMigration: string(r.Migration.UID),
		LabelPlan:      string(r.Plan.GetUID()),
	}
}

func (r *Labeler) VMLabels(vmRef ref.Ref) map[string]string {
	labels := r.MigrationLabels()
	labels[LabelVM] = vmRef.ID
	return labels
}

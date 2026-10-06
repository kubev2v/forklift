package copyappliance

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/labeler"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Labeler struct {
	labeler.Labeler
}

func (r *Labeler) ApplianceLabels(provider *api.Provider, migrationUID types.UID, vmID string) map[string]string {
	return map[string]string{
		api.LabelApp:       api.AppForklift,
		api.LabelSubapp:    api.SubappAppliance,
		api.LabelProvider:  string(provider.UID),
		api.LabelMigration: string(migrationUID),
		api.LabelVM:        vmID,
	}
}

func (r *Labeler) CheckLabels(provider *api.Provider) map[string]string {
	return map[string]string{
		api.LabelApp:      api.AppForklift,
		api.LabelSubapp:   api.SubappCheck,
		api.LabelProvider: string(provider.UID),
	}
}

// Ensurer finds and creates CopyAppliance CRs by label (names are GenerateName).
type Ensurer struct {
	Labeler Labeler
	Log     logging.LevelLogger
	k8sclient.Client
}

// Appliance ensures a live appliance with the built labels exists.
func (r *Ensurer) Appliance(ctx context.Context, appliance *api.CopyAppliance) (*api.CopyAppliance, error) {
	found, err := r.Find(ctx, appliance.Namespace, appliance.Labels, true)
	if err != nil || found != nil {
		return found, err
	}
	if err = r.Create(ctx, appliance); err != nil {
		return nil, liberr.Wrap(err)
	}
	r.Log.Info("Created CopyAppliance.", "appliance", appliance.Name, "namespace", appliance.Namespace)
	return appliance, nil
}

// Find returns the appliance matching labels, or nil. liveOnly skips tearing-down ones.
func (r *Ensurer) Find(ctx context.Context, namespace string, labels map[string]string, liveOnly bool) (*api.CopyAppliance, error) {
	list := &api.CopyApplianceList{}
	err := r.List(ctx, list, &k8sclient.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(labels),
		Namespace:     namespace,
	})
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	var found *api.CopyAppliance
	for i := range list.Items {
		item := &list.Items[i]
		if liveOnly && item.DeletionTimestamp != nil {
			continue
		}
		if found != nil {
			return nil, liberr.New(
				"found multiple copy appliances with the same labels",
				"labels", labels,
				"namespace", namespace)
		}
		found = item
	}
	return found, nil
}

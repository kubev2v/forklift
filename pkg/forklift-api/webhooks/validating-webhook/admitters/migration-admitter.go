package admitters

import (
	"context"
	"encoding/json"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
	admissionv1 "k8s.io/api/admission/v1beta1"
	cnv "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MigrationAdmitter struct {
	Client    client.Client
	migration api.Migration
	plan      api.Plan
}

func (admitter *MigrationAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("Migration admitter was called")
	raw := ar.Request.Object.Raw

	err := json.Unmarshal(raw, &admitter.migration)
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	err = admitter.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: admitter.migration.Spec.Plan.Namespace,
			Name:      admitter.migration.Spec.Plan.Name,
		},
		&admitter.plan)
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	destinationProvider := api.Provider{}
	err = admitter.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: admitter.plan.Spec.Provider.Destination.Namespace,
			Name:      admitter.plan.Spec.Provider.Destination.Name,
		},
		&destinationProvider)
	if err != nil {
		log.Error(err, "Failed to get destination provider, can't determine permissions")
		return util.ToAdmissionResponseError(err)
	}

	if destinationProvider.IsHost() {
		//  make sure that the user can create virtual machines in the target namespace
		// before allowing the migration object
		err = util.PermitUser(ar.Request, admitter.Client, cnv.Resource("virtualmachines"), "", admitter.plan.Spec.TargetNamespace, util.Create)
		if err != nil {
			return util.ToAdmissionResponseError(err)
		}
	}

	sourceProvider := api.Provider{}
	err = admitter.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: admitter.plan.Spec.Provider.Source.Namespace,
			Name:      admitter.plan.Spec.Provider.Source.Name,
		},
		&sourceProvider)
	if err != nil {
		log.Error(err, "Failed to get source provider, can't determine permissions")
		return util.ToAdmissionResponseError(err)
	}

	if sourceProvider.IsHost() {
		for _, vm := range admitter.plan.Spec.VMs {
			err = util.PermitUser(ar.Request, admitter.Client, cnv.Resource("virtualmachines"), vm.Name, vm.Namespace, util.Get)
			if err != nil {
				return util.ToAdmissionResponseError(err)
			}
		}
	}

	return util.ToAdmissionResponseAllow()
}

package admitters

import (
	"context"
	"encoding/json"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProviderAdmitter struct {
	Client   client.Client
	provider api.Provider
}

func (admitter *ProviderAdmitter) validateVDDK() error {
	if admitter.provider.Type() != api.VSphere {
		log.Info("Provider of this type does not require VDDK, passing", "type", admitter.provider.Type())
		return nil
	}

	if _, found := admitter.provider.Spec.Settings[api.VDDK]; found {
		log.Info("VDDK image found, passing")
		return nil
	}

	plans := api.PlanList{}
	err := admitter.Client.List(context.TODO(), &plans, &client.ListOptions{})
	if err != nil {
		log.Error(err, "Couldn't get all plans", "namespace", admitter.provider.Namespace)
		return err
	}

	for _, plan := range plans.Items {
		if plan.Spec.Provider.Source.Namespace != admitter.provider.Namespace ||
			plan.Spec.Provider.Source.Name != admitter.provider.Name {
			log.V(1).Info("Plan not associated to provider, skipping",
				"plan", plan.Name,
				"namespace", plan.Namespace)
			continue
		}
		if plan.Spec.Archived {
			log.V(1).Info("Plan is archived, skipping",
				"plan", plan.Name,
				"namespace", plan.Namespace)
			continue
		}

		var destinationProvider api.Provider
		err = admitter.Client.Get(
			context.TODO(),
			client.ObjectKey{
				Namespace: plan.Spec.Provider.Destination.Namespace,
				Name:      plan.Spec.Provider.Destination.Name,
			},
			&destinationProvider)
		if err != nil {
			log.Error(err, "Couldn't get the destination provider for plan, skipping unwillingly",
				"plan", plan.Name,
				"namespace", plan.Namespace)
			continue
		}
		plan.Referenced.Provider.Source = &admitter.provider
		plan.Referenced.Provider.Destination = &destinationProvider

		el9, el9Err := plan.VSphereUsesEl9VirtV2v()
		if el9Err != nil {
			log.Error(el9Err, "Could not analyze plan, skipping unwillingly",
				"plan", plan.Name,
				"namespace", plan.Namespace)
			continue
		}
		if !el9 {
			err := liberr.New("Plans requiring VDDK are associated with this provider")
			log.Error(err, "Plans requiring VDDK are associated with this provider, failing",
				"plan", plan.Name,
				"namespace", plan.Namespace)
			return err
		}
	}
	return nil
}

func (admitter *ProviderAdmitter) validateSdkEndpointType() error {
	endpoint, ok := admitter.provider.Spec.Settings[api.SDK]
	if ok && admitter.provider.Type() == api.VSphere && endpoint != api.VCenter && endpoint != api.ESXI {
		return liberr.New("vSphere provider is set with an invalid SDK endpoint type", "endpoint", endpoint)
	}
	return nil
}

func (admitter *ProviderAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("Provider admitter was called")
	raw := ar.Request.Object.Raw

	if err := json.Unmarshal(raw, &admitter.provider); err != nil {
		return util.ToAdmissionResponseError(err)
	}

	if err := admitter.validateVDDK(); err != nil {
		return util.ToAdmissionResponseError(err)
	}

	if err := admitter.validateSdkEndpointType(); err != nil {
		return util.ToAdmissionResponseError(err)
	}

	return util.ToAdmissionResponseAllow()
}

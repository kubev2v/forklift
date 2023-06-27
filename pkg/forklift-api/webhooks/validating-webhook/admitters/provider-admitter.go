package admitters

import (
	"context"
	"encoding/json"

	"github.com/konveyor/forklift-controller/pkg/apis"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	admissionv1 "k8s.io/api/admission/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProviderAdmitter struct {
	client   client.Client
	provider api.Provider
}

func (admitter *ProviderAdmitter) validateVDDK() error {
	if admitter.provider.Type() != api.VSphere {
		log.Info("Provider of this type does not require VDDK, passing", "type", admitter.provider.Type())
		return nil
	}

	if _, found := admitter.provider.Spec.Settings["vddkInitImage"]; found {
		log.Info("VDDK image found, passing")
		return nil
	}

	plans := api.PlanList{}
	err := admitter.client.List(context.TODO(), &plans, &client.ListOptions{})
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
		err = admitter.client.Get(
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

func (admitter *ProviderAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("Provider admitter was called")
	raw := ar.Request.Object.Raw

	err := json.Unmarshal(raw, &admitter.provider)
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "Couldn't get the cluster configuration")
		return util.ToAdmissionResponseError(err)
	}

	err = api.SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Error(err, "Couldn't build the scheme")
		return util.ToAdmissionResponseError(err)
	}
	err = apis.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Error(err, "Couldn't add forklift API to the scheme")
		return util.ToAdmissionResponseError(err)
	}

	admitter.client, err = client.New(config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Error(err, "Couldn't create a cluster client")
		return util.ToAdmissionResponseError(err)
	}

	err = admitter.validateVDDK()
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	return util.ToAdmissionResponseAllow()
}

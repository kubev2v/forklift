package admitters

import (
	"context"
	v1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"encoding/json"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/konveyor/forklift-controller/pkg/apis"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
)

type PlanAdmitter struct {
	client client.Client
	plan api.Plan
	sourceProvider api.Provider
	destinationProvider api.Provider
}

func (admitter *PlanAdmitter) validateStorage() *admissionv1.AdmissionResponse {

	if admitter.plan.Spec.Warm {
		log.Info("Warm migration supports all storages, passing")
		return util.ToAdmissionResponseAllow()
	}

	if admitter.sourceProvider.Type() == api.VSphere {
		log.Info("Provider supports all storages, passing")
		return util.ToAdmissionResponseAllow()
	}

	if !admitter.destinationProvider.IsHost() {
		log.Info("Migration to a remote provider supports all storages, passing")
		return util.ToAdmissionResponseAllow()
	}

	storageClasses := v1.StorageClassList{}
	err := admitter.client.List(context.TODO(), &storageClasses, &client.ListOptions{})
	if err != nil {
		log.Error(err, "Couldn't get the cluster storage classes")
		return util.ToAdmissionResponseError(err)
	}

	storageMap := api.StorageMap{}
	err = admitter.client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: admitter.plan.Spec.Map.Storage.Namespace,
			Name: admitter.plan.Spec.Map.Storage.Name,
		},
		&storageMap)
	if err != nil {
		log.Error(err, "Couldn't get the storage map")
		return util.ToAdmissionResponseError(err)
	}
	storagePairList := storageMap.Spec.Map
	var badStorageClasses []string
	for _, storagePair := range storagePairList {
		scName := storagePair.Destination.StorageClass
		for _, sc := range storageClasses.Items {
			if scName == sc.Name && sc.Provisioner == "kubernetes.io/no-provisioner" {
				badStorageClasses = append(badStorageClasses, sc.Name)
			}
		}
	}
	if len(badStorageClasses) > 0 {
		err := liberr.New(fmt.Sprintf("Static storage class(es) found: %v", badStorageClasses))
		log.Error(err, "Static storage class(es) found failing", "classes", badStorageClasses)
		return util.ToAdmissionResponseError(err)
	}

	log.Info("Passed storage validation")
	return util.ToAdmissionResponseAllow()
}


func (admitter *PlanAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("Plan admitter was called")
	raw := ar.Request.Object.Raw

	err := json.Unmarshal(raw, &admitter.plan)
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

	err = admitter.client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: admitter.plan.Spec.Provider.Source.Namespace,
			Name: admitter.plan.Spec.Provider.Source.Name,
		},
		&admitter.sourceProvider)
	if err != nil {
		log.Error(err, "Couldn't get the source provider, passing unwillingly")
		return util.ToAdmissionResponseAllow()
	}

	err = admitter.client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: admitter.plan.Spec.Provider.Destination.Namespace,
			Name: admitter.plan.Spec.Provider.Destination.Name,
		},
		&admitter.destinationProvider)
	if err != nil {
		log.Error(err, "Couldn't get the destination provider, passing unwillingly")
		return util.ToAdmissionResponseAllow()
	}

	response := admitter.validateStorage()
	if !response.Allowed {
		return response
	}

	return util.ToAdmissionResponseAllow()
}

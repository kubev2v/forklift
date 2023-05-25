package admitters

import (
	"context"
	v1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"encoding/json"
	"fmt"
	admissionv1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"net/http"

	"github.com/konveyor/forklift-controller/pkg/apis"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
)

type PlanAdmitter struct {
}

func (admitter *PlanAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("Plan admitter was called")
	raw := ar.Request.Object.Raw

	plan := &api.Plan{}
	err := json.Unmarshal(raw, plan)
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	if plan.Spec.Warm {
		log.Info("Warm migration supports all storages, passing")
		return util.ToAdmissionResponseAllow()
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

	cl, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Error(err, "Couldn't create a cluster client")
		return util.ToAdmissionResponseError(err)
	}

	sourceProvider := api.Provider{}
	err = cl.Get(context.TODO(), client.ObjectKey{Namespace: plan.Spec.Provider.Source.Namespace, Name: plan.Spec.Provider.Source.Name}, &sourceProvider)
	if err != nil {
		log.Error(err, "Couldn't get the source provider")
		return util.ToAdmissionResponseError(err)
	}

	if sourceProvider.Type() == api.VSphere {
		log.Info("Provider supports all storages, passing")
		return util.ToAdmissionResponseAllow()
	}

	destinationProvider := api.Provider{}
	err = cl.Get(context.TODO(), client.ObjectKey{Namespace: plan.Spec.Provider.Destination.Namespace, Name: plan.Spec.Provider.Destination.Name}, &destinationProvider)
	if err != nil {
		log.Error(err, "Couldn't get the destination provider")
		return util.ToAdmissionResponseError(err)
	}

	if !destinationProvider.IsHost() {
		log.Info("Migration to a remote provider supports all storages, passing")
		return util.ToAdmissionResponseAllow()
	}

	storageClasses := v1.StorageClassList{}
	err = cl.List(context.TODO(), &storageClasses, &client.ListOptions{})
	if err != nil {
		log.Error(err, "Couldn't get the cluster storage classes")
		return util.ToAdmissionResponseError(err)
	}

	storageMap := api.StorageMap{}
	err = cl.Get(context.TODO(), client.ObjectKey{Namespace: plan.Spec.Map.Storage.Namespace, Name: plan.Spec.Map.Storage.Name}, &storageMap)
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
		log.Error(fmt.Errorf("storage class(es) '%v' is static, failing", badStorageClasses), "Storage class(es) is static")
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("This plan requires dynamic volume provisioning. Therefore the following destination storage classes cannot be used: %v", badStorageClasses),
			},
		}
	}
	log.Info("Passed storage validation")
	return util.ToAdmissionResponseAllow()
}

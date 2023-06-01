package admitters

import (
	"context"
	v1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"encoding/json"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/konveyor/forklift-controller/pkg/apis"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
)

type PlanAdmitter struct {
	client              client.Client
	plan                api.Plan
	sourceProvider      api.Provider
	destinationProvider api.Provider
}

func (admitter *PlanAdmitter) validateStorage() error {

	if admitter.plan.Spec.Warm {
		log.Info("Warm migration supports all storages, passing")
		return nil
	}

	if admitter.sourceProvider.Type() == api.VSphere {
		log.Info("Provider supports all storages, passing")
		return nil
	}

	if !admitter.destinationProvider.IsHost() {
		log.Info("Migration to a remote provider supports all storages, passing")
		return nil
	}

	storageClasses := v1.StorageClassList{}
	err := admitter.client.List(context.TODO(), &storageClasses, &client.ListOptions{})
	if err != nil {
		log.Error(err, "Couldn't get the cluster storage classes")
		return err
	}

	storageMap := api.StorageMap{}
	err = admitter.client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: admitter.plan.Spec.Map.Storage.Namespace,
			Name:      admitter.plan.Spec.Map.Storage.Name,
		},
		&storageMap)
	if err != nil {
		log.Error(err, "Couldn't get the storage map")
		return err
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
		return err
	}

	log.Info("Passed storage validation")
	return nil
}

func (admitter *PlanAdmitter) validateVDDK() error {
	if admitter.sourceProvider.Type() != api.VSphere {
		log.Info("Provider type (non-VSphere) does not require VDDK, passing")
		return nil
	}

	el9, el9Err := admitter.plan.VSphereUsesEl9VirtV2v()
	if el9Err != nil {
		log.Error(el9Err, "Could not analyze plan, failing")
		return el9Err
	}
	if el9 {
		// VDDK image is optional when EL9 virt-v2v image is in use
		log.Info("VDDK image is optional when EL9 virt-v2v image is in use, passing")
		return nil
	}

	if _, found := admitter.sourceProvider.Spec.Settings["vddkInitImage"]; !found {
		err := liberr.New("VDDK image is necessary for this type of migration")
		log.Error(err, "VDDK image required for this type of migration")
		return err
	}

	return nil
}

func (admitter *PlanAdmitter) validateWarmMigrations() error {
	providerType := admitter.sourceProvider.Type()
	isWarmMigration := admitter.plan.Spec.Warm
	if providerType == api.OpenStack && isWarmMigration {
		err := liberr.New("warm migration is not supported by the provider")
		log.Error(err, "provider", providerType)
		return err
	}
	return nil
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
			Name:      admitter.plan.Spec.Provider.Source.Name,
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
			Name:      admitter.plan.Spec.Provider.Destination.Name,
		},
		&admitter.destinationProvider)
	if err != nil {
		log.Error(err, "Couldn't get the destination provider, passing unwillingly")
		return util.ToAdmissionResponseAllow()
	}

	admitter.plan.Referenced.Provider.Source = &admitter.sourceProvider
	admitter.plan.Referenced.Provider.Destination = &admitter.destinationProvider

	err = admitter.validateStorage()
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	err = admitter.validateVDDK()
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	err = admitter.validateWarmMigrations()
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	return util.ToAdmissionResponseAllow()
}

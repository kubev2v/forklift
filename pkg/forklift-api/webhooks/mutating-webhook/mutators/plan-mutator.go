package mutators

import (
	"context"
	"encoding/json"
	"net/http"

	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/konveyor/forklift-controller/pkg/apis"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
	admissionv1 "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AnnPopulatorLabels = "populatorLabels"
)

type PlanMutator struct {
	ar   *admissionv1.AdmissionReview
	plan api.Plan
}

func (mutator *PlanMutator) Mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("plan mutator was called")
	mutator.ar = ar
	raw := ar.Request.Object.Raw
	err := json.Unmarshal(raw, &mutator.plan)
	if err != nil {
		log.Error(err, "mutating webhook error, failed to unmarshel plan")
		return util.ToAdmissionResponseError(err)
	}

	specChanged, err := mutator.setTransferNetworkIfNotSet()
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}
	metadataChanged := mutator.addPopulatorLabelsAnnotation()
	if specChanged || metadataChanged {
		patches := mutator.patchPayload(specChanged, metadataChanged)
		patchBytes, err := util.GeneratePatchPayload(patches...)
		if err != nil {
			log.Error(err, "mutating webhook error, failed to generate payload for patch request")
			return util.ToAdmissionResponseError(err)
		}

		jsonPatchType := admissionv1.PatchTypeJSONPatch
		return &admissionv1.AdmissionResponse{
			Allowed:   true,
			Patch:     patchBytes,
			PatchType: &jsonPatchType,
		}
	} else {
		return &admissionv1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: "Certificate retrieval is not required, passing ",
				Code:    http.StatusOK,
			},
		}
	}
}

func (mutator *PlanMutator) setTransferNetworkIfNotSet() (bool, error) {
	var planChanged bool

	if mutator.plan.Spec.TransferNetwork == nil {
		config, err := rest.InClusterConfig()
		if err != nil {
			log.Error(err, "Couldn't get the cluster configuration")
			return false, err
		}

		err = api.SchemeBuilder.AddToScheme(scheme.Scheme)
		if err != nil {
			log.Error(err, "Couldn't build the scheme")
			return false, err
		}
		err = apis.AddToScheme(scheme.Scheme)
		if err != nil {
			log.Error(err, "Couldn't add forklift API to the scheme")
			return false, err
		}
		err = net.AddToScheme(scheme.Scheme)
		if err != nil {
			log.Error(err, "Couldn't add network-attachment-definition-client to the scheme")
			return false, err
		}

		cl, err := client.New(config, client.Options{Scheme: scheme.Scheme})
		if err != nil {
			log.Error(err, "Couldn't create a cluster client")
			return false, err
		}

		targetProvider := api.Provider{}
		err = cl.Get(context.TODO(), client.ObjectKey{Namespace: mutator.plan.Spec.Provider.Destination.Namespace, Name: mutator.plan.Spec.Provider.Destination.Name}, &targetProvider)
		if err != nil {
			log.Error(err, "Couldn't get the target provider")
			return false, err
		}

		if network := targetProvider.Annotations["forklift.konveyor.io/defaultTransferNetwork"]; network != "" {
			key := client.ObjectKey{
				Namespace: mutator.plan.Spec.TargetNamespace,
				Name:      network,
			}

			var tcl client.Client // target client, i.e., client to a possibly remote cluster
			if targetProvider.IsHost() {
				tcl = cl
			} else {
				ref := targetProvider.Spec.Secret
				secret := &core.Secret{}
				err = cl.Get(
					context.TODO(),
					client.ObjectKey{
						Namespace: ref.Namespace,
						Name:      ref.Name,
					},
					secret)
				if err != nil {
					log.Error(err, "Failed to get secret for target provider")
					return false, err
				}
				tcl, err = targetProvider.Client(secret)
			}
			if err != nil {
				log.Error(err, "Failed to initiate client to target cluster")
				return false, err
			}

			netAttachDef := &net.NetworkAttachmentDefinition{}
			if err = tcl.Get(context.TODO(), key, netAttachDef); err == nil {
				log.Info("Patching the plan's transfer network")
				mutator.plan.Spec.TransferNetwork = &core.ObjectReference{
					Name:      network,
					Namespace: mutator.plan.Spec.TargetNamespace,
				}
				planChanged = true
			} else if !k8serr.IsNotFound(err) { // TODO: else if !NotFound ...
				log.Error(err, "Failed to get the network-attachment-definition")
				return false, err
			}
		}
	}
	return planChanged, nil
}

func (mutator *PlanMutator) addPopulatorLabelsAnnotation() bool {
	if mutator.ar.Request.Operation == admissionv1.Create {
		if _, ok := mutator.plan.Annotations[AnnPopulatorLabels]; !ok {
			if mutator.plan.Annotations == nil {
				mutator.plan.Annotations = make(map[string]string)
			}
			mutator.plan.Annotations[AnnPopulatorLabels] = "True"
			log.Info("Patching the plan's annotation")
			return true
		}
	}
	return false
}

func (mutator *PlanMutator) patchPayload(specChanged, metadataChanged bool) []util.PatchOperation {
	var patches []util.PatchOperation
	if specChanged {
		patches = append(patches, util.PatchOperation{
			Op:    "replace",
			Path:  "/spec",
			Value: mutator.plan.Spec,
		})
	}
	if metadataChanged {
		patches = append(patches, util.PatchOperation{
			Op:    "replace",
			Path:  "/metadata",
			Value: mutator.plan.ObjectMeta,
		})
	}
	return patches
}

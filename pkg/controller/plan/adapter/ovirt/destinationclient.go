package ovirt

import (
	"context"
	"path"
	"strconv"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	ocpclient "github.com/konveyor/forklift-controller/pkg/lib/client/openshift"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type DestinationClient struct {
	*plancontext.Context
}

// Delete OvirtVolumePopulator CustomResource list.
func (r *DestinationClient) DeletePopulatorDataSource(vm *plan.VMStatus) error {
	r.Log.Info("Benny - DeletePopulatorDataSource")
	populatorCrList, err := r.getPopulatorCrList()
	if err != nil {
		return liberr.Wrap(err)
	}
	r.Log.Info("Benny - DeletePopulatorDataSource", "populatorCrList", populatorCrList)
	for _, populatorCr := range populatorCrList.Items {
		err = r.DeleteObject(&populatorCr, vm, "Deleted OvirtPopulator CR.", "OvirtVolumePopulator")
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}

// Set the OvirtVolumePopulator CustomResource Ownership.
func (r *DestinationClient) SetPopulatorCrOwnership() (err error) {
	populatorCrList, err := r.getPopulatorCrList()
	if err != nil {
		return
	}

	for _, populatorCr := range populatorCrList.Items {
		pvc, err := r.findPVCByCR(&populatorCr)
		if err != nil {
			continue
		}

		populatorCrCopy := populatorCr.DeepCopy()
		err = k8sutil.SetOwnerReference(pvc, &populatorCr, r.Scheme())
		if err != nil {
			continue
		}
		patch := client.MergeFrom(populatorCrCopy)
		err = r.Destination.Client.Patch(context.TODO(), &populatorCr, patch)
		if err != nil {
			continue
		}
	}
	return
}

func (r *DestinationClient) calculateAPIGroup(kind string) (*schema.GroupVersionKind, error) {
	// If OCP version is >= 4.16 use forklift.cdi.konveyor.io
	// Otherwise use forklift.konveyor.io
	r.Log.Info("Benny calculateAPIGroup")
	restCfg := ocpclient.RestCfg(r.Destination.Provider, r.Plan.Referenced.Secret)
	r.Log.Info("Benny Rest", "restCfg", restCfg)
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	r.Log.Info("Benny Before discoveryClient.ServerVersion()")

	discoveryClient := clientset.Discovery()
	version, err := discoveryClient.ServerVersion()
	if err != nil {
		r.Log.Info("Benny ServerVersion() error", "error", err)

		return nil, liberr.Wrap(err)
	}

	r.Log.Info("Benny calculateAPIGroup after discoveryClient.ServerVersion()")

	major, err := strconv.Atoi(version.Major)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	minor, err := strconv.Atoi(version.Minor)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	r.Log.Info("Benny calculateAPIGroup before return")

	if major < 1 || (major == 1 && minor <= 28) {
		return &schema.GroupVersionKind{Group: "forklift.konveyor.io", Version: "v1beta1", Kind: kind}, nil
	}

	return &schema.GroupVersionKind{Group: "forklift.cdi.konveyor.io", Version: "v1beta1", Kind: kind}, nil
}

// Get the OvirtVolumePopulator CustomResource List.
// Get the OvirtVolumePopulator CustomResource List.
func (r *DestinationClient) getPopulatorCrList() (populatorCrList v1beta1.OvirtVolumePopulatorList, err error) {
	r.Log.Info("Getting OvirtVolumePopulatorList")
	populatorCrList = v1beta1.OvirtVolumePopulatorList{}
	gvk, err := r.calculateAPIGroup("OvirtVolumePopulator")
	if err != nil {
		r.Log.Info("Error calculating API group", "error", err)
		return
	}
	r.Log.Info("API Group", "apiGroup", gvk)

	// Create a dynamic client using the correct GVK
	dynamicClient, err := dynamic.NewForConfig(ocpclient.RestCfg(r.Destination.Provider, r.Plan.Referenced.Secret))
	if err != nil {
		r.Log.Info("Error creating dynamic client", "error", err)
		return
	}

	resource := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: "ovirtvolumepopulators", // Use the plural form of the resource
	}

	unstructuredList, err := dynamicClient.Resource(resource).Namespace(r.Plan.Spec.TargetNamespace).List(context.TODO(), meta.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{"migration": string(r.Plan.Status.Migration.ActiveSnapshot().Migration.UID)}).String(),
	})
	if err != nil {
		r.Log.Info("Error listing OvirtVolumePopulator", "error", err)
		return
	}

	// Convert the unstructured list to the structured list
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredList.UnstructuredContent(), &populatorCrList)
	if err != nil {
		r.Log.Info("Error converting unstructured list", "error", err)
		return
	}

	r.Log.Info("Successfully retrieved OvirtVolumePopulator list")
	return
}

// Deletes an object from destination cluster associated with the VM.
func (r *DestinationClient) DeleteObject(object client.Object, vm *plan.VMStatus, message, objType string) (err error) {
	err = r.Destination.Client.Delete(context.TODO(), object)
	if err != nil {
		if k8serr.IsNotFound(err) {
			err = nil
		} else {
			return liberr.Wrap(err)
		}
	} else {
		r.Log.Info(
			message,
			objType,
			path.Join(
				object.GetNamespace(),
				object.GetName()),
			"vm",
			vm.String())
	}
	return
}

func (r *DestinationClient) findPVCByCR(cr *v1beta1.OvirtVolumePopulator) (pvc *core.PersistentVolumeClaim, err error) {
	pvcList := core.PersistentVolumeClaimList{}
	err = r.Destination.Client.List(
		context.TODO(),
		&pvcList,
		&client.ListOptions{
			Namespace: r.Plan.Spec.TargetNamespace,
			LabelSelector: labels.SelectorFromSet(map[string]string{
				"migration": string(r.Plan.Status.Migration.ActiveSnapshot().Migration.UID),
				"diskID":    cr.Spec.DiskID,
			}),
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if len(pvcList.Items) == 0 {
		err = liberr.New("PVC not found", "diskID", cr.Spec.DiskID)
		return
	}

	if len(pvcList.Items) > 1 {
		err = liberr.New("Multiple PVCs found", "diskID", cr.Spec.DiskID)
		return
	}

	pvc = &pvcList.Items[0]

	return
}

package populator_machinery

import (
	"context"
	"fmt"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/dynamiclister"
	"k8s.io/client-go/kubernetes/fake"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	testGroup = "forklift.konveyor.io"
)

// newTestController builds a minimal controller wired to fake clients and
// in-memory listers so that syncPvc can be called end-to-end.
func newTestController(
	t *testing.T,
	gk schema.GroupKind,
	retain bool,
	pvc *corev1.PersistentVolumeClaim,
	pod *corev1.Pod,
	cr *unstructured.Unstructured,
	populatorNs string,
) (*controller, *fake.Clientset) {
	t.Helper()

	kubeClient := fake.NewSimpleClientset(pvc)

	pvcIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if err := pvcIndexer.Add(pvc); err != nil {
		t.Fatal(err)
	}

	podIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if err := podIndexer.Add(pod); err != nil {
		t.Fatal(err)
	}

	unstIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if err := unstIndexer.Add(cr); err != nil {
		t.Fatal(err)
	}

	gvr := schema.GroupVersionResource{Group: gk.Group, Version: "v1beta1", Resource: "testresource"}

	m := initMetrics()
	if gk.Kind == api.VSphereXcopyVolumePopulatorKind {
		m.initCompletionMetrics()
	}

	c := &controller{
		kubeClient:          kubeClient,
		retainPopulatorPods: retain,
		gk:                  gk,
		populatedFromAnno:   testGroup + "/" + populatedFromAnnoSuffix,
		pvcLister:           corelisters.NewPersistentVolumeClaimLister(pvcIndexer),
		podLister:           corelisters.NewPodLister(podIndexer),
		unstLister:          dynamiclister.New(unstIndexer, gvr),
		notifyMap:           make(map[string]*stringSet),
		cleanupMap:          make(map[string]*stringSet),
		populatorArgs: func(_ bool, _ *unstructured.Unstructured, _ corev1.PersistentVolumeClaim) ([]string, error) {
			return []string{"--cr-namespace=" + populatorNs}, nil
		},
		metrics:  m,
		recorder: record.NewFakeRecorder(100),
	}
	return c, kubeClient
}

func makeTestPVC(name, ns string, uid types.UID, kind string, annotations map[string]string) *corev1.PersistentVolumeClaim {
	apiGroup := testGroup
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			UID:         uid,
			Annotations: annotations,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     kind,
				Name:     "test-cr",
			},
		},
	}
	return pvc
}

func makeFailedPod(pvcUID types.UID, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", populatorPodPrefix, pvcUID),
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
		},
	}
}

func makeTestCR(name, namespace, kind string) *unstructured.Unstructured {
	cr := &unstructured.Unstructured{}
	cr.SetName(name)
	cr.SetNamespace(namespace)
	cr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   testGroup,
		Version: "v1beta1",
		Kind:    kind,
	})
	return cr
}

func TestDeleteFailedPVC_RetainDisabled(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "test-ns",
		},
	}
	kubeClient := fake.NewSimpleClientset(pvc)

	c := &controller{
		kubeClient:          kubeClient,
		retainPopulatorPods: false,
	}

	err := c.deleteFailedPVC(context.Background(), pvc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err = kubeClient.CoreV1().PersistentVolumeClaims("test-ns").Get(context.Background(), "test-pvc", metav1.GetOptions{})
	if err == nil {
		t.Fatal("expected PVC to be deleted, but it still exists")
	}
}

func TestSyncPvc_RetainEnabled_XcopyFailure(t *testing.T) {
	const (
		pvcNs       = "test-ns"
		pvcName     = "test-pvc"
		populatorNs = "populator-ns"
		pvcUID      = types.UID("xcopy-uid-001")
	)
	kind := api.VSphereXcopyVolumePopulatorKind
	gk := schema.GroupKind{Group: testGroup, Kind: kind}

	pvc := makeTestPVC(pvcName, pvcNs, pvcUID, kind, nil)
	pod := makeFailedPod(pvcUID, populatorNs)
	cr := makeTestCR("test-cr", pvcNs, kind)

	c, kubeClient := newTestController(t, gk, true, pvc, pod, cr, populatorNs)

	key := pvcNs + "/" + pvcName
	err := c.syncPvc(context.Background(), key, pvcNs, pvcName)
	if err != nil {
		t.Fatalf("syncPvc should return nil when retain is enabled, got: %v", err)
	}

	_, err = kubeClient.CoreV1().PersistentVolumeClaims(pvcNs).Get(context.Background(), pvcName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("PVC should still exist when retain is enabled, got: %v", err)
	}
}

func TestSyncPvc_RetainEnabled_GenericFailureAfterRetries(t *testing.T) {
	const (
		pvcNs       = "test-ns"
		pvcName     = "generic-pvc"
		populatorNs = "populator-ns"
		pvcUID      = types.UID("generic-uid-002")
	)
	kind := api.OvirtVolumePopulatorKind
	gk := schema.GroupKind{Group: testGroup, Kind: kind}

	pvc := makeTestPVC(pvcName, pvcNs, pvcUID, kind, map[string]string{
		AnnPopulatorReCreations: "3",
	})
	pod := makeFailedPod(pvcUID, populatorNs)
	cr := makeTestCR("test-cr", pvcNs, kind)

	c, kubeClient := newTestController(t, gk, true, pvc, pod, cr, populatorNs)

	key := pvcNs + "/" + pvcName
	err := c.syncPvc(context.Background(), key, pvcNs, pvcName)
	if err != nil {
		t.Fatalf("syncPvc should return nil when retain is enabled after retries exhausted, got: %v", err)
	}

	_, err = kubeClient.CoreV1().PersistentVolumeClaims(pvcNs).Get(context.Background(), pvcName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("PVC should still exist when retain is enabled, got: %v", err)
	}
}

func TestRetainPopulatorPods_FieldSetCorrectly(t *testing.T) {
	c := &controller{
		retainPopulatorPods: true,
	}
	if !c.retainPopulatorPods {
		t.Fatal("retainPopulatorPods should be true")
	}

	c.retainPopulatorPods = false
	if c.retainPopulatorPods {
		t.Fatal("retainPopulatorPods should be false")
	}
}

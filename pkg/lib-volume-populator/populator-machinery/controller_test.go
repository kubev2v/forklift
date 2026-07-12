package populator_machinery

import (
	"context"
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/dynamiclister"
	"k8s.io/client-go/kubernetes/fake"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
)

var (
	xcopyGVR = schema.GroupVersionResource{
		Group:    api.SchemeGroupVersion.Group,
		Version:  "v1beta1",
		Resource: api.VSphereXcopyVolumePopulatorResource,
	}
	xcopyGK = schema.GroupKind{
		Group: api.SchemeGroupVersion.Group,
		Kind:  api.VSphereXcopyVolumePopulatorKind,
	}
)

// makePVC returns a minimal unbound PVC pointing at a VSphereXcopyVolumePopulator.
func makePVC(name, namespace, crName, scName string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID("pvc-uid-" + name),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: ptr.To(scName),
			VolumeMode:       ptr.To(corev1.PersistentVolumeBlock),
			DataSourceRef: &corev1.TypedObjectReference{
				APIGroup: ptr.To(api.SchemeGroupVersion.Group),
				Kind:     api.VSphereXcopyVolumePopulatorKind,
				Name:     crName,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
}

// makeCR returns an unstructured VSphereXcopyVolumePopulator with optional sourceHost label and migrationHost spec.
func makeCR(name, namespace, sourceHost, migrationHost string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   api.SchemeGroupVersion.Group,
		Version: "v1beta1",
		Kind:    api.VSphereXcopyVolumePopulatorKind,
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)

	if sourceHost != "" {
		obj.SetLabels(map[string]string{labelSourceHost: sourceHost})
	}
	if migrationHost != "" {
		_ = unstructured.SetNestedField(obj.Object, migrationHost, "spec", "migrationHost")
	}
	_ = unstructured.SetNestedField(obj.Object, "test-secret", "spec", "secretName")
	return obj
}

// makeRunningPod returns a Running pod with the given throttleHost label, simulating an active populator.
func makeRunningPod(name, namespace, throttleHost string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{labelThrottleHost: throttleHost},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

// makeStorageClass returns a CSI storage class (not in-tree, passes checkIntreeStorageClass).
func makeStorageClass(name string) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		ObjectMeta:  metav1.ObjectMeta{Name: name},
		Provisioner: "csi.example.com",
	}
}

// buildController wires up a controller with fake listers and a fake kubeClient.
// existingPods are pre-populated in the pod indexer (simulates in-flight pods).
func buildController(t *testing.T, maxInFlight int, existingPods []*corev1.Pod,
	pvcs []*corev1.PersistentVolumeClaim, crs []*unstructured.Unstructured) (*controller, *fake.Clientset, *record.FakeRecorder) {
	t.Helper()

	pvcIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, pvc := range pvcs {
		_ = pvcIndexer.Add(pvc)
	}

	podIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, pod := range existingPods {
		_ = podIndexer.Add(pod)
	}

	scIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	_ = scIndexer.Add(makeStorageClass("test-sc"))

	crIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, cr := range crs {
		_ = crIndexer.Add(cr)
	}

	fakeClient := fake.NewSimpleClientset()
	fakeRecorder := record.NewFakeRecorder(20)
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())

	c := &controller{
		populatedFromAnno: "forklift.konveyor.io/populated-from",
		kubeClient:        fakeClient,
		imageName:         "test-image",
		devicePath:        "/dev/block",
		mountPath:         "/mnt/data",
		pvcLister:         corelisters.NewPersistentVolumeClaimLister(pvcIndexer),
		pvLister:          corelisters.NewPersistentVolumeLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
		podLister:         corelisters.NewPodLister(podIndexer),
		scLister:          storagelisters.NewStorageClassLister(scIndexer),
		unstLister:        dynamiclister.New(crIndexer, xcopyGVR),
		notifyMap:         make(map[string]*stringSet),
		cleanupMap:        make(map[string]*stringSet),
		workqueue:         q,
		gk:                xcopyGK,
		metrics:           initMetrics(),
		recorder:          fakeRecorder,
		maxInFlight:       maxInFlight,
		populatorArgs: func(_ bool, _ *unstructured.Unstructured, _ corev1.PersistentVolumeClaim) ([]string, error) {
			return []string{"--cr-namespace=test-ns", "--secret-name=test-secret"}, nil
		},
	}
	return c, fakeClient, fakeRecorder
}

// isThrottled returns true if syncPvc emitted a PopulatorThrottled event (synchronous, no sleep needed).
func isThrottled(recorder *record.FakeRecorder) bool {
	select {
	case event := <-recorder.Events:
		return strings.Contains(event, "PopulatorThrottled")
	default:
		return false
	}
}

// createdPodLabels returns the labels of the pod created via the fake kubeClient, or nil if none was created.
func createdPodLabels(fakeClient *fake.Clientset) map[string]string {
	for _, action := range fakeClient.Actions() {
		if action.GetVerb() == "create" && action.GetResource().Resource == "pods" {
			obj := action.(interface{ GetObject() runtime.Object }).GetObject()
			pod := obj.(*corev1.Pod)
			return pod.Labels
		}
	}
	return nil
}

// ── Test cases ──────────────────────────────────────────────────────────────

// 1.1 No dedicated hosts — throttle fires when maxInFlight reached on source host.
func TestThrottle_NoMigrationHost_SingleSourceHost_Throttled(t *testing.T) {
	const host = "host-A"
	existingPod := makeRunningPod("pod-existing", "test-ns", host)
	pvc := makePVC("pvc-1", "test-ns", "cr-1", "test-sc")
	cr := makeCR("cr-1", "test-ns", host, "" /* no migrationHost */)

	c, _, recorder := buildController(t, 1, []*corev1.Pod{existingPod}, []*corev1.PersistentVolumeClaim{pvc}, []*unstructured.Unstructured{cr})

	err := c.syncPvc(context.Background(), "test-ns/pvc-1", "test-ns", "pvc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isThrottled(recorder) {
		t.Error("expected PVC to be throttled on source host")
	}
}

// 1.2 No dedicated hosts — two independent source hosts don't throttle each other.
func TestThrottle_NoMigrationHost_TwoSourceHosts_Independent(t *testing.T) {
	podOnA := makeRunningPod("pod-A", "test-ns", "host-A")
	pvc := makePVC("pvc-B", "test-ns", "cr-B", "test-sc")
	cr := makeCR("cr-B", "test-ns", "host-B", "")

	c, fakeClient, recorder := buildController(t, 1, []*corev1.Pod{podOnA}, []*corev1.PersistentVolumeClaim{pvc}, []*unstructured.Unstructured{cr})

	err := c.syncPvc(context.Background(), "test-ns/pvc-B", "test-ns", "pvc-B")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isThrottled(recorder) {
		t.Error("expected PVC on host-B NOT to be throttled (host-A is full, host-B is free)")
	}
	if labels := createdPodLabels(fakeClient); labels == nil {
		t.Error("expected a pod to be created for host-B")
	}
}

// 2.1 Dedicated host (1 host) — throttle keys on migrationHost, not sourceHost.
func TestThrottle_MigrationHost_ThrottlesOnMigrationHost(t *testing.T) {
	podOnDedicated := makeRunningPod("pod-dedicated", "test-ns", "dedicated-host")
	pvc := makePVC("pvc-1", "test-ns", "cr-1", "test-sc")
	cr := makeCR("cr-1", "test-ns", "source-host", "dedicated-host")

	c, _, recorder := buildController(t, 1, []*corev1.Pod{podOnDedicated}, []*corev1.PersistentVolumeClaim{pvc}, []*unstructured.Unstructured{cr})

	err := c.syncPvc(context.Background(), "test-ns/pvc-1", "test-ns", "pvc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isThrottled(recorder) {
		t.Error("expected PVC to be throttled on dedicated-host")
	}
}

// 2.2 Dedicated host (1 host) — created pod carries throttleHost=migrationHost label.
func TestThrottle_MigrationHost_PodLabel(t *testing.T) {
	pvc := makePVC("pvc-1", "test-ns", "cr-1", "test-sc")
	cr := makeCR("cr-1", "test-ns", "source-host", "dedicated-host")

	c, fakeClient, _ := buildController(t, 10, nil, []*corev1.PersistentVolumeClaim{pvc}, []*unstructured.Unstructured{cr})

	err := c.syncPvc(context.Background(), "test-ns/pvc-1", "test-ns", "pvc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	labels := createdPodLabels(fakeClient)
	if labels == nil {
		t.Fatal("expected pod to be created")
	}
	if labels[labelThrottleHost] != "dedicated-host" {
		t.Errorf("expected throttleHost label = %q, got %q", "dedicated-host", labels[labelThrottleHost])
	}
	if labels[labelSourceHost] != "source-host" {
		t.Errorf("expected sourceHost label = %q, got %q", "source-host", labels[labelSourceHost])
	}
}

// 3.1 Dedicated hosts (2 hosts) — throttle keys on the correct migrationHost per CR.
func TestThrottle_TwoDedicatedHosts_ThrottleIndependent(t *testing.T) {
	podOnHost1 := makeRunningPod("pod-h1", "test-ns", "dedicated-host1")
	pvc := makePVC("pvc-1", "test-ns", "cr-1", "test-sc")
	cr := makeCR("cr-1", "test-ns", "source-host", "dedicated-host2")

	c, fakeClient, recorder := buildController(t, 1, []*corev1.Pod{podOnHost1}, []*corev1.PersistentVolumeClaim{pvc}, []*unstructured.Unstructured{cr})

	err := c.syncPvc(context.Background(), "test-ns/pvc-1", "test-ns", "pvc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isThrottled(recorder) {
		t.Error("expected PVC targeting dedicated-host2 NOT to be throttled (only host1 is full)")
	}
	if labels := createdPodLabels(fakeClient); labels == nil {
		t.Error("expected pod to be created")
	}
}

// 3.2 Dedicated hosts (2 hosts) — pod gets correct throttleHost label for each host.
func TestThrottle_TwoDedicatedHosts_PodLabel(t *testing.T) {
	pvc := makePVC("pvc-1", "test-ns", "cr-1", "test-sc")
	cr := makeCR("cr-1", "test-ns", "source-host", "dedicated-host2")

	c, fakeClient, _ := buildController(t, 10, nil, []*corev1.PersistentVolumeClaim{pvc}, []*unstructured.Unstructured{cr})

	err := c.syncPvc(context.Background(), "test-ns/pvc-1", "test-ns", "pvc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	labels := createdPodLabels(fakeClient)
	if labels == nil {
		t.Fatal("expected pod to be created")
	}
	if labels[labelThrottleHost] != "dedicated-host2" {
		t.Errorf("expected throttleHost label = %q, got %q", "dedicated-host2", labels[labelThrottleHost])
	}
}

// 4.1 Mixed — CR with migrationHost throttles on migrationHost, not sourceHost.
func TestThrottle_Mixed_WithMigrationHost_ThrottlesOnMigrationHost(t *testing.T) {
	podOnDedicated := makeRunningPod("pod-dedicated", "test-ns", "dedicated-host")
	pvc := makePVC("pvc-1", "test-ns", "cr-1", "test-sc")
	cr := makeCR("cr-1", "test-ns", "source-host-free", "dedicated-host")

	c, _, recorder := buildController(t, 1, []*corev1.Pod{podOnDedicated}, []*corev1.PersistentVolumeClaim{pvc}, []*unstructured.Unstructured{cr})

	err := c.syncPvc(context.Background(), "test-ns/pvc-1", "test-ns", "pvc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isThrottled(recorder) {
		t.Error("expected throttle on dedicated-host even though sourceHost is free")
	}
}

// 4.2 Mixed — CR without migrationHost throttles on sourceHost (no regression).
func TestThrottle_Mixed_WithoutMigrationHost_ThrottlesOnSourceHost(t *testing.T) {
	podOnSource := makeRunningPod("pod-source", "test-ns", "source-host")
	pvc := makePVC("pvc-1", "test-ns", "cr-1", "test-sc")
	cr := makeCR("cr-1", "test-ns", "source-host", "" /* no migrationHost */)

	c, _, recorder := buildController(t, 1, []*corev1.Pod{podOnSource}, []*corev1.PersistentVolumeClaim{pvc}, []*unstructured.Unstructured{cr})

	err := c.syncPvc(context.Background(), "test-ns/pvc-1", "test-ns", "pvc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isThrottled(recorder) {
		t.Error("expected throttle on sourceHost when no migrationHost configured")
	}
}

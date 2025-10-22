/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package populator_machinery

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/dynamiclister"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-helpers/storage/volume"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

const (
	populatorContainerName  = "populate"
	populatorPodPrefix      = "populate"
	populatorPodVolumeName  = "target"
	populatorPvcPrefix      = "prime"
	populatedFromAnnoSuffix = "populated-from"
	pvcFinalizerSuffix      = "populate-target-protection"
	annSelectedNode         = "volume.kubernetes.io/selected-node"
	controllerNameSuffix    = "populator"

	reasonPodCreationError   = "PopulatorCreationError"
	reasonPodCreationSuccess = "PopulatorCreated"
	reasonPodFailed          = "PopulatorFailed"
	reasonPodFinished        = "PopulatorFinished"
	reasonPVCCreationError   = "PopulatorPVCCreationError"
	reasonPopulatorProgress  = "PopulatorProgress"
	AnnTransferNetwork       = "k8s.v1.cni.cncf.io/networks"
	AnnPopulatorReCreations  = "recreations"

	qemuGroup = 107
)

type empty struct{}

type stringSet struct {
	set map[string]empty
}

var (
	populatorToResource = map[string]*populatorResource{
		"OvirtVolumePopulator": {
			storageResourceKey: "disk_id",
			resource:           "ovirtvolumepopulators",
			regexKey:           "ovirt_volume_populator",
		},
		"OpenstackVolumePopulator": {
			storageResourceKey: "image_id",
			resource:           "openstackvolumepopulators",
			regexKey:           "openstack_volume_populator",
		},
		api.VSphereXcopyVolumePopulatorKind: {
			storageResourceKey: "source_vmdk",
			resource:           api.VSphereXcopyVolumePopulatorResource,
			regexKey:           "vsphere_xcopy_volume_populator",
		},
	}

	monitoredPVCs = map[string]interface{}{}
)

type populatorResource struct {
	storageResourceKey string
	resource           string
	regexKey           string
}

type controller struct {
	populatedFromAnno string
	pvcFinalizer      string
	kubeClient        kubernetes.Interface
	dynamicClient     dynamic.Interface
	imageName         string
	devicePath        string
	mountPath         string
	pvcLister         corelisters.PersistentVolumeClaimLister
	pvcSynced         cache.InformerSynced
	pvLister          corelisters.PersistentVolumeLister
	pvSynced          cache.InformerSynced
	podLister         corelisters.PodLister
	podSynced         cache.InformerSynced
	scLister          storagelisters.StorageClassLister
	scSynced          cache.InformerSynced
	unstLister        dynamiclister.Lister
	unstSynced        cache.InformerSynced
	mu                sync.Mutex
	notifyMap         map[string]*stringSet
	cleanupMap        map[string]*stringSet
	workqueue         workqueue.TypedRateLimitingInterface[string]
	populatorArgs     func(bool, *unstructured.Unstructured, corev1.PersistentVolumeClaim) ([]string, error)
	gk                schema.GroupKind
	metrics           *metricsManager
	recorder          record.EventRecorder
	httpClient        *http.Client
}

func RunController(masterURL, kubeconfig, imageName, httpEndpoint, metricsPath, prefix string,
	gk schema.GroupKind, gvr schema.GroupVersionResource, mountPath, devicePath string,
	populatorArgs func(bool, *unstructured.Unstructured, corev1.PersistentVolumeClaim) ([]string, error),
) {
	klog.Infof("Starting populator controller for %s", gk)

	stopCh := make(chan struct{})
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		close(stopCh)
		<-sigCh
		os.Exit(1) // second signal. Exit directly.
	}()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to create config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Failed to create client: %v", err)
	}

	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Failed to create dynamic client: %v", err)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	dynInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, time.Second*30)

	pvcInformer := kubeInformerFactory.Core().V1().PersistentVolumeClaims()
	pvInformer := kubeInformerFactory.Core().V1().PersistentVolumes()
	podInformer := kubeInformerFactory.Core().V1().Pods()
	scInformer := kubeInformerFactory.Storage().V1().StorageClasses()
	unstInformer := dynInformerFactory.ForResource(gvr).Informer()

	c := &controller{
		kubeClient:        kubeClient,
		dynamicClient:     dynClient,
		imageName:         imageName,
		devicePath:        devicePath,
		mountPath:         mountPath,
		populatedFromAnno: prefix + "/" + populatedFromAnnoSuffix,
		pvcFinalizer:      prefix + "/" + pvcFinalizerSuffix,
		pvcLister:         pvcInformer.Lister(),
		pvcSynced:         pvcInformer.Informer().HasSynced,
		pvLister:          pvInformer.Lister(),
		pvSynced:          pvInformer.Informer().HasSynced,
		podLister:         podInformer.Lister(),
		podSynced:         podInformer.Informer().HasSynced,
		scLister:          scInformer.Lister(),
		scSynced:          scInformer.Informer().HasSynced,
		unstLister:        dynamiclister.New(unstInformer.GetIndexer(), gvr),
		unstSynced:        unstInformer.HasSynced,
		notifyMap:         make(map[string]*stringSet),
		cleanupMap:        make(map[string]*stringSet),
		workqueue:         workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
		populatorArgs:     populatorArgs,
		gk:                gk,
		metrics:           initMetrics(),
		recorder:          getRecorder(kubeClient, prefix+"-"+controllerNameSuffix),
	}

	c.metrics.startListener(httpEndpoint, metricsPath)
	defer c.metrics.stopListener()

	_, err = pvcInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handlePVC,
		UpdateFunc: func(old, new interface{}) {
			newPvc := new.(*corev1.PersistentVolumeClaim)
			oldPvc := old.(*corev1.PersistentVolumeClaim)
			if newPvc.ResourceVersion == oldPvc.ResourceVersion {
				return
			}
			c.handlePVC(new)
		},
		DeleteFunc: c.handlePVC,
	})
	if err != nil {
		klog.Fatalf("Failed to add event handler for pvc: %v", err)
	}

	_, err = pvInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handlePV,
		UpdateFunc: func(old, new interface{}) {
			newPv := new.(*corev1.PersistentVolume)
			oldPv := old.(*corev1.PersistentVolume)
			if newPv.ResourceVersion == oldPv.ResourceVersion {
				return
			}
			c.handlePV(new)
		},
		DeleteFunc: c.handlePV,
	})
	if err != nil {
		klog.Fatalf("Failed to add event handler for pv: %v", err)
	}

	_, err = podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handlePod,
		UpdateFunc: func(old, new interface{}) {
			newPod := new.(*corev1.Pod)
			oldPod := old.(*corev1.Pod)
			if newPod.ResourceVersion == oldPod.ResourceVersion {
				return
			}
			c.handlePod(new)
		},
		DeleteFunc: c.handlePod,
	})
	if err != nil {
		klog.Fatalf("Failed to add event handler for pod: %v", err)
	}

	_, err = scInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleSC,
		UpdateFunc: func(old, new interface{}) {
			newSc := new.(*storagev1.StorageClass)
			oldSc := old.(*storagev1.StorageClass)
			if newSc.ResourceVersion == oldSc.ResourceVersion {
				return
			}
			c.handleSC(new)
		},
		DeleteFunc: c.handleSC,
	})
	if err != nil {
		klog.Fatalf("Failed to add event handler for sc: %v", err)
	}

	_, err = unstInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleUnstructured,
		UpdateFunc: func(old, new interface{}) {
			newUnstructured := new.(*unstructured.Unstructured)
			oldUnstructured := old.(*unstructured.Unstructured)
			if newUnstructured.GetResourceVersion() == oldUnstructured.GetResourceVersion() {
				return
			}
			c.handleUnstructured(new)
		},
		DeleteFunc: c.handleUnstructured,
	})
	if err != nil {
		klog.Fatalf("Failed to add event handler for unstructured: %v", err)
	}

	kubeInformerFactory.Start(stopCh)
	dynInformerFactory.Start(stopCh)

	if err = c.run(stopCh); err != nil {
		klog.Fatalf("Failed to run controller: %v", err)
	}
}

func getRecorder(kubeClient kubernetes.Interface, controllerName string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerName})
	return recorder
}

func (c *controller) addNotification(keyToCall, objType, namespace, name string) {
	var key string
	if 0 == len(namespace) {
		key = objType + "/" + name
	} else {
		key = objType + "/" + namespace + "/" + name
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.notifyMap[key]
	if s == nil {
		s = &stringSet{make(map[string]empty)}
		c.notifyMap[key] = s
	}
	s.set[keyToCall] = empty{}
	s = c.cleanupMap[keyToCall]
	if s == nil {
		s = &stringSet{make(map[string]empty)}
		c.cleanupMap[keyToCall] = s
	}
	s.set[key] = empty{}
}

func (c *controller) cleanupNotifications(keyToCall string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.cleanupMap[keyToCall]
	if s == nil {
		return
	}
	for key := range s.set {
		t := c.notifyMap[key]
		if t == nil {
			continue
		}
		delete(t.set, keyToCall)
		if 0 == len(t.set) {
			delete(c.notifyMap, key)
		}
	}
}

func translateObject(obj interface{}) metav1.Object {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return nil
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return nil
		}
	}
	return object
}

func (c *controller) handleMapped(obj interface{}, objType string) {
	object := translateObject(obj)
	if object == nil {
		return
	}
	var key string
	if len(object.GetNamespace()) == 0 {
		key = objType + "/" + object.GetName()
	} else {
		key = objType + "/" + object.GetNamespace() + "/" + object.GetName()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.notifyMap[key]; ok {
		for k := range s.set {
			c.workqueue.Add(k)
		}
	}
}

func (c *controller) handlePVC(obj interface{}) {
	c.handleMapped(obj, "pvc")
	object := translateObject(obj)
	if object == nil {
		return
	}

	c.workqueue.Add("pvc/" + object.GetNamespace() + "/" + object.GetName())
}

func (c *controller) handlePV(obj interface{}) {
	c.handleMapped(obj, "pv")
}

func (c *controller) handlePod(obj interface{}) {
	c.handleMapped(obj, "pod")
}

func (c *controller) handleSC(obj interface{}) {
	c.handleMapped(obj, "sc")
}

func (c *controller) handleUnstructured(obj interface{}) {
	c.handleMapped(obj, "unstructured")
}

func (c *controller) run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	ok := cache.WaitForCacheSync(stopCh, c.pvcSynced, c.pvSynced, c.podSynced, c.scSynced, c.unstSynced)
	if !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh

	return nil
}

func (c *controller) runWorker() {
	processNextWorkItem := func(obj string) error {
		defer c.workqueue.Done(obj)
		var key string
		if key = obj; key == "" {
			utilruntime.HandleError(fmt.Errorf("expected valid string in workqueue but got empty string"))
			return nil
		}
		var err error
		parts := strings.Split(key, "/")
		switch parts[0] {
		case "pvc":
			if len(parts) != 3 {
				utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
				return nil
			}
			err = c.syncPvc(context.TODO(), key, parts[1], parts[2])
		default:
			utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
			return nil
		}
		if err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		c.workqueue.Forget(obj)
		return nil
	}

	for {
		obj, shutdown := c.workqueue.Get()
		if shutdown {
			return
		}
		err := processNextWorkItem(obj)
		if err != nil {
			utilruntime.HandleError(err)
		}
	}
}

func (c *controller) syncPvc(ctx context.Context, key, pvcNamespace, pvcName string) error {
	var err error

	var pvc *corev1.PersistentVolumeClaim
	pvc, err = c.pvcLister.PersistentVolumeClaims(pvcNamespace).Get(pvcName)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("pvc '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	dataSourceRef := pvc.Spec.DataSourceRef
	if dataSourceRef == nil {
		// Ignore PVCs without a datasource
		return nil
	}

	apiGroup := ""
	if dataSourceRef.APIGroup != nil {
		apiGroup = *dataSourceRef.APIGroup
	}
	if c.gk.Group != apiGroup || c.gk.Kind != dataSourceRef.Kind || "" == dataSourceRef.Name {
		// Ignore PVCs that aren't for this populator to handle
		return nil
	}

	var crInstance *unstructured.Unstructured
	crInstance, err = c.unstLister.Namespace(pvc.Namespace).Get(dataSourceRef.Name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		c.addNotification(key, "unstructured", pvc.Namespace, dataSourceRef.Name)
		// We'll get called again later when the data source exists
		return nil
	}
	var rawBlock bool
	if nil != pvc.Spec.VolumeMode && corev1.PersistentVolumeBlock == *pvc.Spec.VolumeMode {
		rawBlock = true
	}

	// Set the args for the populator pod
	args, err := c.populatorArgs(rawBlock, crInstance, *pvc)
	if err != nil {
		return err
	}

	var secretName string
	var populatorNamespace string
	for _, val := range args {
		if strings.HasPrefix(val, "--cr-namespace=") {
			populatorNamespace = strings.Split(val, "--cr-namespace=")[1]
		} else if strings.HasPrefix(val, "--secret-name=") {
			secretName = strings.Split(val, "--secret-name=")[1]
		}
	}

	pvcSize := pvc.Spec.Resources.Requests.Storage().Value()
	args = append(args, fmt.Sprintf("--pvc-size=%d", pvcSize))
	args = append(args, fmt.Sprintf("--owner-uid=%s", pvc.UID))

	var waitForFirstConsumer bool
	var nodeName string
	if pvc.Spec.StorageClassName != nil {
		storageClassName := *pvc.Spec.StorageClassName

		var storageClass *storagev1.StorageClass
		storageClass, err = c.scLister.Get(storageClassName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
			c.addNotification(key, "sc", "", storageClassName)
			// We'll get called again later when the storage class exists
			return nil
		}

		if err := c.checkIntreeStorageClass(pvc, storageClass); err != nil {
			klog.V(2).Infof("Ignoring PVC %s/%s: %s", pvcNamespace, pvcName, err)
			return nil
		}

		if storageClass.VolumeBindingMode != nil && storagev1.VolumeBindingWaitForFirstConsumer == *storageClass.VolumeBindingMode {
			waitForFirstConsumer = true
			nodeName = pvc.Annotations[annSelectedNode]
			if nodeName == "" {
				// Wait for the PVC to get a node name before continuing
				return nil
			}
		}
	}

	// Look for the populator pod
	podName := fmt.Sprintf("%s-%s", populatorPodPrefix, pvc.UID)
	c.addNotification(key, "pod", populatorNamespace, podName)
	var pod *corev1.Pod
	pod, err = c.podLister.Pods(populatorNamespace).Get(podName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	// Look for PVC'
	pvcPrimeName := fmt.Sprintf("%s-%s", populatorPvcPrefix, pvc.UID)
	c.addNotification(key, "pvc", populatorNamespace, pvcPrimeName)
	var pvcPrime *corev1.PersistentVolumeClaim
	pvcPrime, err = c.pvcLister.PersistentVolumeClaims(populatorNamespace).Get(pvcPrimeName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	// *** Here is the first place we start to create/modify objects ***

	// If the PVC is unbound, we need to perform the population
	if "" == pvc.Spec.VolumeName {

		// Ensure the PVC has a finalizer on it so we can clean up the stuff we create
		err = c.ensureFinalizer(ctx, pvc, c.pvcFinalizer, true)
		if err != nil {
			return err
		}

		// Record start time for populator metric
		c.metrics.operationStart(pvc.UID)

		// If the pod doesn't exist yet, create it
		if pod == nil {
			transferNetwork, found, err := unstructured.NestedStringMap(crInstance.Object, "spec", "transferNetwork")
			if err != nil {
				return err
			}
			annotations := make(map[string]string)
			if found {
				// Join the transfer network namespace and name
				annotations[AnnTransferNetwork] = fmt.Sprintf("%s/%s", transferNetwork["namespace"], transferNetwork["name"])
			}
			migration, found, err := unstructured.NestedString(crInstance.Object, "metadata", "labels", "migration")
			if err != nil {
				return err
			}
			labels := map[string]string{"pvcName": pvc.Name}
			if found {
				labels["migration"] = migration
			}

			// Make the pod
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        podName,
					Namespace:   populatorNamespace,
					Annotations: annotations,
					Labels:      labels,
				},
				Spec: makePopulatePodSpec(pvcPrimeName, secretName),
			}
			if c.gk.Kind == api.VSphereXcopyVolumePopulatorKind {
				pod.Spec.ServiceAccountName = "populator"
			}
			pod.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName = pvcPrimeName
			con := &pod.Spec.Containers[0]
			con.Image = c.imageName
			con.Args = args
			if rawBlock {
				con.VolumeDevices = []corev1.VolumeDevice{
					{
						Name:       populatorPodVolumeName,
						DevicePath: c.devicePath,
					},
				}
			} else {
				con.VolumeMounts = []corev1.VolumeMount{
					{
						Name:      populatorPodVolumeName,
						MountPath: c.mountPath,
					},
				}
			}

			if waitForFirstConsumer {
				pod.Spec.NodeName = nodeName
			}
			_, err = c.kubeClient.CoreV1().Pods(populatorNamespace).Create(ctx, pod, metav1.CreateOptions{})
			if err != nil {
				c.recorder.Eventf(pvc, corev1.EventTypeWarning, reasonPodCreationError, "Failed to create populator pod: %s", err)
				return err
			}
			c.recorder.Eventf(pvc, corev1.EventTypeNormal, reasonPodCreationSuccess, "Populator started")

			// If PVC' doesn't exist yet, create it
			if pvcPrime == nil {
				pvcPrime = &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvcPrimeName,
						Namespace: populatorNamespace,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      pvc.Spec.AccessModes,
						Resources:        pvc.Spec.Resources,
						StorageClassName: pvc.Spec.StorageClassName,
						VolumeMode:       pvc.Spec.VolumeMode,
					},
				}
				if waitForFirstConsumer {
					pvcPrime.Annotations = map[string]string{
						annSelectedNode: nodeName,
					}
				}
				_, err = c.kubeClient.CoreV1().PersistentVolumeClaims(populatorNamespace).Create(ctx, pvcPrime, metav1.CreateOptions{})
				if err != nil {
					c.recorder.Eventf(pvc, corev1.EventTypeWarning, reasonPVCCreationError, "Failed to create populator PVC: %s", err)
					return err
				}
			}

			// We'll get called again later when the pod exists
			return nil
		} else {
			if pod.Status.PodIP != "" {
				if _, ok := monitoredPVCs[string(pvc.UID)]; !ok {
					monitoredPVCs[string(pvc.UID)] = true
					go func() {
						c.recorder.Eventf(pod, corev1.EventTypeWarning, reasonPopulatorProgress, "Starting to monitor progress for PVC %s", pvc.Name)
						for {
							err = c.updateProgress(pod, pvc, crInstance)
							if err != nil {
								klog.V(5).Info("Failed to update progress", err)
								continue
							}

							pod, err = c.podLister.Pods(populatorNamespace).Get(pod.Name)
							if err != nil {
								break
							}
							if pod.Status.Phase != corev1.PodRunning {
								break
							}

							// TODO make configurable?
							time.Sleep(5 * time.Second)
						}
					}()
				}
			}
		}

		if corev1.PodSucceeded != pod.Status.Phase {
			if corev1.PodFailed == pod.Status.Phase {
				restarts, ok := pvc.Annotations[AnnPopulatorReCreations]
				if !ok {
					return c.retryFailedPopulator(ctx, pvc, populatorNamespace, pod.Name, 1)
				}
				restartsInteger, err := strconv.Atoi(restarts)
				if err != nil {
					return err
				}
				if restartsInteger < 3 {
					return c.retryFailedPopulator(ctx, pvc, populatorNamespace, pod.Name, restartsInteger+1)
				}
				c.recorder.Eventf(pvc, corev1.EventTypeWarning, reasonPodFailed, "Populator failed after few (3) attempts: Please check the logs of the populator pod, %s/%s", populatorNamespace, pod.Name)
			}
			// We'll get called again later when the pod succeeds
			return nil
		}

		// This would be bad
		if pvcPrime == nil {
			return fmt.Errorf("Failed to find PVC for populator pod")
		}

		// Get PV
		var pv *corev1.PersistentVolume
		c.addNotification(key, "pv", "", pvcPrime.Spec.VolumeName)
		pv, err = c.kubeClient.CoreV1().PersistentVolumes().Get(ctx, pvcPrime.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
			// We'll get called again later when the PV exists
			return nil
		}

		// Examine the claimref for the PV and see if it's bound to the correct PVC
		claimRef := pv.Spec.ClaimRef
		if claimRef.Name != pvc.Name || claimRef.Namespace != pvc.Namespace || claimRef.UID != pvc.UID {
			// Make new PV with strategic patch values to perform the PV rebind
			patchPv := corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:        pv.Name,
					Annotations: map[string]string{},
				},
				Spec: corev1.PersistentVolumeSpec{
					ClaimRef: &corev1.ObjectReference{
						Namespace:       pvc.Namespace,
						Name:            pvc.Name,
						UID:             pvc.UID,
						ResourceVersion: pvc.ResourceVersion,
					},
				},
			}
			patchPv.Annotations[c.populatedFromAnno] = pvc.Namespace + "/" + dataSourceRef.Name
			var patchData []byte
			patchData, err = json.Marshal(patchPv)
			if err != nil {
				return err
			}
			_, err = c.kubeClient.CoreV1().PersistentVolumes().Patch(ctx, pv.Name, types.StrategicMergePatchType,
				patchData, metav1.PatchOptions{})
			if err != nil {
				return err
			}

			// Don't start cleaning up yet -- we need to bind controller to acknowledge
			// the switch
			return nil
		}
	}

	// Wait for the bind controller to rebind the PV
	if pvcPrime != nil {
		if corev1.ClaimLost != pvcPrime.Status.Phase {
			return nil
		}
	}

	// Record start time for populator metric
	c.metrics.recordMetrics(pvc.UID, "success")

	// *** At this point the volume population is done and we're just cleaning up ***
	c.recorder.Eventf(pvc, corev1.EventTypeNormal, reasonPodFinished, "Populator finished")

	// If PVC' still exists, delete it
	if pvcPrime != nil {
		err = c.kubeClient.CoreV1().PersistentVolumeClaims(populatorNamespace).Delete(ctx, pvcPrime.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	// Make sure the PVC finalizer is gone
	err = c.ensureFinalizer(ctx, pvc, c.pvcFinalizer, false)
	if err != nil {
		return err
	}

	// Clean up our internal callback maps
	c.cleanupNotifications(key)

	// Stop progress monitoring
	delete(monitoredPVCs, string(pvc.UID))

	return nil
}

func (c *controller) retryFailedPopulator(ctx context.Context, pvc *corev1.PersistentVolumeClaim, namespace, podName string, counter int) error {
	pvc.Annotations[AnnPopulatorReCreations] = strconv.Itoa(counter)
	err := c.updatePvc(ctx, pvc, namespace)
	if err != nil {
		return err
	}
	err = c.kubeClient.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *controller) updatePvc(ctx context.Context, pvc *corev1.PersistentVolumeClaim, namespace string) (err error) {
	_, err = c.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, pvc, metav1.UpdateOptions{})
	return err
}

func (c *controller) updateProgress(pod *corev1.Pod, pvc *corev1.PersistentVolumeClaim, cr *unstructured.Unstructured) error {
	populatorKind := pvc.Spec.DataSourceRef.Kind
	importRegExp := regexp.MustCompile("progress\\{ownerUID=\"" + string(pvc.UID) + "\"\\} (\\d+\\.?\\d*)")

	url, err := getMetricsURL(pod)
	if err != nil {
		klog.V(5).Info("Failed to get metrics URL: ", err)
		return err
	}

	if c.httpClient == nil {
		c.httpClient = buildHTTPClient()
	}

	resp, err := c.httpClient.Get(url)
	if err != nil {
		klog.V(5).Info("Failed to get metrics: ", err)
		return err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		klog.V(5).Info(err)
		return err
	}

	match := importRegExp.FindStringSubmatch(string(body))
	if match == nil {
		klog.V(5).Info("Failed to find matches, regex: ", importRegExp)
		return nil
	}

	progress, err := strconv.ParseFloat(string(match[1]), 64)
	if err != nil {
		klog.V(5).Info("Could not convert progress: ", err)
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    *pvc.Spec.DataSourceRef.APIGroup,
		Version:  "v1beta1",
		Resource: populatorToResource[populatorKind].resource,
	}

	latestPopulator, err := c.dynamicClient.Resource(gvr).Namespace(pvc.Namespace).Get(context.TODO(), cr.GetName(), metav1.GetOptions{})
	if err != nil {
		klog.V(5).Info("Failed to get CR for kind: ", populatorKind, "error: ", err)
		return err
	}

	err = updatePopulatorProgress(int64(progress), latestPopulator)
	if err != nil {
		klog.V(5).Info("Failed to update progress: ", err)
		return err
	}

	_, err = c.dynamicClient.Resource(gvr).Namespace(pvc.Namespace).Update(context.TODO(), latestPopulator, metav1.UpdateOptions{})
	if err != nil {
		klog.V(5).Info("Failed to update CR ", err)
		return err
	}

	if progress != 0 {
		klog.Info("Updated progress: ", progress)
	}

	return nil
}

func updatePopulatorProgress(progress int64, cr *unstructured.Unstructured) error {
	if err := unstructured.SetNestedField(cr.Object, fmt.Sprintf("%d", progress), "status", "progress"); err != nil {
		return err
	}

	return nil
}

func makePopulatePodSpec(pvcPrimeName, secretName string) corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  populatorContainerName,
				Ports: []corev1.ContainerPort{{Name: "metrics", ContainerPort: 8443}},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					RunAsNonRoot:             ptr.To(true),
					RunAsUser:                ptr.To[int64](qemuGroup),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
				},
				EnvFrom: []corev1.EnvFromSource{
					{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretName,
							},
						},
					},
				},
			},
		},
		SecurityContext: &corev1.PodSecurityContext{
			FSGroup: ptr.To[int64](qemuGroup),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
		RestartPolicy: corev1.RestartPolicyNever,
		Volumes: []corev1.Volume{
			{
				Name: populatorPodVolumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcPrimeName,
					},
				},
			},
		},
	}
}

func (c *controller) ensureFinalizer(ctx context.Context, pvc *corev1.PersistentVolumeClaim, finalizer string, want bool) error {
	finalizers := pvc.GetFinalizers()
	found := false
	foundIdx := -1
	for i, v := range finalizers {
		if finalizer == v {
			found = true
			foundIdx = i
			break
		}
	}
	if found == want {
		// Nothing to do in this case
		return nil
	}

	type patchOp struct {
		Op    string      `json:"op"`
		Path  string      `json:"path"`
		Value interface{} `json:"value,omitempty"`
	}

	var patch []patchOp

	if want {
		// Add the finalizer to the end of the list
		patch = []patchOp{
			{
				Op:    "test",
				Path:  "/metadata/finalizers",
				Value: finalizers,
			},
			{
				Op:    "add",
				Path:  "/metadata/finalizers/-",
				Value: finalizer,
			},
		}
	} else {
		// Remove the finalizer from the list index where it was found
		path := fmt.Sprintf("/metadata/finalizers/%d", foundIdx)
		patch = []patchOp{
			{
				Op:    "test",
				Path:  path,
				Value: finalizer,
			},
			{
				Op:   "remove",
				Path: path,
			},
		}
	}

	data, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	_, err = c.kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Patch(ctx, pvc.Name, types.JSONPatchType,
		data, metav1.PatchOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *controller) checkIntreeStorageClass(pvc *corev1.PersistentVolumeClaim, sc *storagev1.StorageClass) error {
	if !strings.HasPrefix(sc.Provisioner, "kubernetes.io/") {
		// This is not an in-tree StorageClass
		return nil
	}

	if pvc.Annotations != nil {
		if migrated := pvc.Annotations[volume.AnnMigratedTo]; migrated != "" {
			// The PVC is migrated to CSI
			return nil
		}
	}

	// The SC is in-tree & PVC is not migrated
	return fmt.Errorf("in-tree volume volume plugin %q cannot use volume populator", sc.Provisioner)
}

func buildHTTPClient() *http.Client {
	defaultTransport := http.DefaultTransport.(*http.Transport)

	transport := &http.Transport{
		Proxy:                 defaultTransport.Proxy,
		DialContext:           defaultTransport.DialContext,
		MaxIdleConns:          defaultTransport.MaxIdleConns,
		IdleConnTimeout:       defaultTransport.IdleConnTimeout,
		ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
		TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,

		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &http.Client{Transport: transport}
}

func getMetricsURL(pod *corev1.Pod) (string, error) {
	if pod == nil {
		return "", nil
	}
	port, err := getPodMetricsPort(pod)
	if err != nil || pod.Status.PodIP == "" {
		return "", err
	}
	url := fmt.Sprintf("https://%s:%d/metrics", pod.Status.PodIP, port)
	return url, nil
}

func getPodMetricsPort(pod *corev1.Pod) (int, error) {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == "metrics" {
				return int(port.ContainerPort), nil
			}
		}
	}
	return 0, fmt.Errorf("failed to find metrics port")
}

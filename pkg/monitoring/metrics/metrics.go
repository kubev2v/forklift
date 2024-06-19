package metrics

import (
	"context"
	"encoding/json"
	"fmt"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	prometheusLabelKey   = "prometheus.forklift.konveyor.io"
	prometheusLabelValue = "true"

	k8sAppLabelKey     = "app"
	forkliftLabelValue = "forklift"
)

func PatchMonitorinLable(namespace string, clientset *kubernetes.Clientset) (err error) {
	labelKey := "openshift.io/cluster-monitoring"
	labelValue := "true"

	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
	if err != nil {
		return
	}

	if val, exists := ns.Labels[labelKey]; exists && val == labelValue {
		return
	}

	payload := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				labelKey: labelValue,
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}

	_, err = clientset.CoreV1().Namespaces().Patch(context.TODO(), namespace, types.MergePatchType, payloadBytes, metav1.PatchOptions{})
	if err != nil {
		return
	}
	return
}

func CreateMetricsService(clientset *kubernetes.Clientset, namespace string, ownerRef *metav1.OwnerReference) error {
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "forklift-metrics",
			Namespace: namespace,
			Labels: map[string]string{
				k8sAppLabelKey:     forkliftLabelValue,
				prometheusLabelKey: prometheusLabelValue,
			},
			OwnerReferences: []metav1.OwnerReference{*ownerRef},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name: "metrics",
					Port: 2112,
					TargetPort: intstr.IntOrString{
						IntVal: 2112,
					},
				},
			},
			Selector: map[string]string{
				k8sAppLabelKey:     forkliftLabelValue,
				prometheusLabelKey: prometheusLabelValue,
			},
		},
	}

	_, err := clientset.CoreV1().Services(namespace).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create service: %v", err)
	}

	return nil
}

func CreateServiceMonitor(client client.Client, namespace string, ownerRef *metav1.OwnerReference) error {
	serviceMonitor := &promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "forklift-metrics",
			Namespace: namespace,
			Labels: map[string]string{
				k8sAppLabelKey:     forkliftLabelValue,
				prometheusLabelKey: prometheusLabelValue,
			},
			OwnerReferences: []metav1.OwnerReference{*ownerRef},
		},
		Spec: promv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					k8sAppLabelKey:     forkliftLabelValue,
					prometheusLabelKey: prometheusLabelValue,
				},
			},
			Endpoints: []promv1.Endpoint{
				{
					Port:     "metrics",
					Interval: "30s",
				},
			},
			NamespaceSelector: promv1.NamespaceSelector{
				MatchNames: []string{
					namespace,
				},
			},
		},
	}

	err := client.Create(context.TODO(), serviceMonitor)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ServiceMonitor: %v", err)
	}

	return nil
}

func GetDeploymentInfo(clientset *kubernetes.Clientset, namespace, deploymentName string) (*metav1.OwnerReference, error) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	ownerRef := &metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       deployment.Name,
		UID:        deployment.UID,
	}

	return ownerRef, nil
}

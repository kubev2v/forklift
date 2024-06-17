package rules

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konveyor/forklift-controller/pkg/monitoring/rules/recordingrules"
)

const (
	forkliftPrometheusRuleName = "prometheus-forklift-rules"

	prometheusLabelKey   = "prometheus.forklift.konveyor.io"
	prometheusLabelValue = "true"

	k8sAppLabelKey     = "app"
	forkliftLabelValue = "forklift"
)

func SetupRules(namespace string) error {
	err := recordingrules.Register(namespace)
	if err != nil {
		return err
	}

	return nil
}

func BuildPrometheusRule(namespace string, ownerRef *metav1.OwnerReference) (*promv1.PrometheusRule, error) {
	rules, err := operatorrules.BuildPrometheusRule(
		forkliftPrometheusRuleName,
		namespace,
		map[string]string{
			prometheusLabelKey: prometheusLabelValue,
			k8sAppLabelKey:     forkliftLabelValue,
		},
	)
	rules.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	if err != nil {
		return nil, err
	}

	return rules, nil
}

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
			Name:      "forklift-servicemonitor",
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

func CreateOrUpdatePrometheusRule(mgr client.Client, namespace string, promRule *promv1.PrometheusRule) error {
	existingPromRule := &promv1.PrometheusRule{}
	err := mgr.Get(context.TODO(), client.ObjectKey{
		Namespace: namespace,
		Name:      promRule.Name,
	}, existingPromRule)
	if err != nil {
		if errors.IsNotFound(err) {
			err = mgr.Create(context.TODO(), promRule)
			if err != nil {
				fmt.Printf("unable to create PrometheusRule: %v", err)
				return err
			}
		} else {
			fmt.Printf("unable to get PrometheusRule: %v", err)
			return err
		}
	} else {
		promRule.ResourceVersion = existingPromRule.ResourceVersion
		err = mgr.Update(context.TODO(), promRule)
		if err != nil {
			fmt.Printf("unable to update PrometheusRule: %v", err)
			return err
		}
	}
	return nil
}

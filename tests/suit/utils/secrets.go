package utils

import (
	"context"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	secretPollPeriod   = defaultPollPeriod
	secretPollInterval = defaultPollInterval
)

// NewSecretDefinition provides a function to initialize a Secret data type with the provided options
func NewSecretDefinition(labels, stringData map[string]string, data map[string][]byte, ns, prefix string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: prefix,
			Namespace:    ns,
			Labels:       labels,
		},
		StringData: stringData,
		Data:       data,
	}
}

// CreateSecretFromDefinition creates and returns a pointer ot a v1.Secret using a provided v1.Secret
func CreateSecretFromDefinition(c *kubernetes.Clientset, definition *v1.Secret) (secret *v1.Secret, err error) {
	secret, err = c.CoreV1().Secrets(definition.Namespace).Create(context.TODO(), definition, metav1.CreateOptions{})
	if err != nil {
		klog.Error(errors.Wrapf(err, "Encountered create error for secret \"%s/%s\"", definition.Namespace, definition.GenerateName))
	}
	return
}

// DeleteSecret ...
func DeleteSecret(clientSet *kubernetes.Clientset, namespace string, secret v1.Secret) error {
	e := wait.PollUntilContextTimeout(context.TODO(), secretPollInterval, secretPollPeriod, true, func(context.Context) (bool, error) {
		err := clientSet.CoreV1().Secrets(namespace).Delete(context.TODO(), secret.GetName(), metav1.DeleteOptions{})
		if err == nil || apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, nil //keep polling
	})
	return e
}

func GetSecret(clientSet *kubernetes.Clientset, namespace, name string) (*v1.Secret, error) {
	return clientSet.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

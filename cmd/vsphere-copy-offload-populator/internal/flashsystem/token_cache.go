package flashsystem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	StorageArraySecretName             = "storage-array-secret"
	StorageArraySecretDefaultNamespace = "openshift-mtv"
)

type CachedToken struct {
	Token     string    `json:"token"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TokenCache interface {
	ReadToken() (string, error)
	WriteToken(token string) error
}

type SecretTokenCache struct {
	client     kubernetes.Interface
	namespace  string
	secretName string
	dataKey    string
	log        klog.Logger
}

func NewSecretTokenCache(client kubernetes.Interface, managementIP string) *SecretTokenCache {
	namespace := StorageArraySecretDefaultNamespace
	if ns := os.Getenv("HOST_LEASE_NAMESPACE"); ns != "" {
		namespace = ns
	}

	sanitized := strings.NewReplacer(".", "-", ":", "-").Replace(managementIP)
	dataKey := "flashsystem-token-" + sanitized

	return &SecretTokenCache{
		client:     client,
		namespace:  namespace,
		secretName: StorageArraySecretName,
		dataKey:    dataKey,
		log:        klog.Background().WithName("flashsystem").WithName("token-cache"),
	}
}

func (c *SecretTokenCache) ReadToken() (string, error) {
	secret, err := c.client.CoreV1().Secrets(c.namespace).Get(
		context.Background(), c.secretName, metav1.GetOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to read token from secret %s/%s: %w",
			c.namespace, c.secretName, err)
	}

	raw, ok := secret.Data[c.dataKey]
	if !ok || len(raw) == 0 {
		return "", nil
	}

	var cached CachedToken
	if err := json.Unmarshal(raw, &cached); err != nil {
		c.log.Info("malformed cached token entry, treating as empty", "key", c.dataKey, "err", err)
		return "", nil
	}

	return cached.Token, nil
}

func (c *SecretTokenCache) WriteToken(token string) error {
	entry := CachedToken{
		Token:     token,
		UpdatedAt: time.Now().UTC(),
	}
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal token entry: %w", err)
	}

	patch := map[string]interface{}{
		"data": map[string]interface{}{
			c.dataKey: entryJSON,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal token patch: %w", err)
	}

	_, err = c.client.CoreV1().Secrets(c.namespace).Patch(
		context.Background(),
		c.secretName,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		if !k8serr.IsNotFound(err) {
			return fmt.Errorf("failed to patch secret %s/%s with token: %w",
				c.namespace, c.secretName, err)
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      c.secretName,
				Namespace: c.namespace,
			},
			Data: map[string][]byte{
				c.dataKey: entryJSON,
			},
		}
		if _, err := c.client.CoreV1().Secrets(c.namespace).Create(
			context.Background(), secret, metav1.CreateOptions{},
		); err != nil {
			return fmt.Errorf("failed to create secret %s/%s: %w",
				c.namespace, c.secretName, err)
		}
	}

	c.log.V(2).Info("token cached in shared secret", "key", c.dataKey)
	return nil
}

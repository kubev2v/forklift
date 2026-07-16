package flashsystem

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const tokenKey = "FLASHSYSTEM_AUTH_TOKEN"

// TokenCache allows sharing FlashSystem auth tokens across populator pod instances
// to avoid token invalidation storms (each user can only have one valid token).
type TokenCache interface {
	ReadToken() (string, error)
	WriteToken(token string) error
}

// SecretTokenCache stores the FlashSystem auth token in the existing populator-secret
// using a strategic merge patch to avoid overwriting other keys.
type SecretTokenCache struct {
	client     kubernetes.Interface
	namespace  string
	secretName string
}

func NewSecretTokenCache(client kubernetes.Interface, namespace, secretName string) *SecretTokenCache {
	return &SecretTokenCache{
		client:     client,
		namespace:  namespace,
		secretName: secretName,
	}
}

func (c *SecretTokenCache) ReadToken() (string, error) {
	secret, err := c.client.CoreV1().Secrets(c.namespace).Get(context.Background(), c.secretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to read token from secret %s/%s: %w", c.namespace, c.secretName, err)
	}
	return string(secret.Data[tokenKey]), nil
}

func (c *SecretTokenCache) WriteToken(token string) error {
	patch := map[string]interface{}{
		"stringData": map[string]string{
			tokenKey: token,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal token patch: %w", err)
	}
	_, err = c.client.CoreV1().Secrets(c.namespace).Patch(
		context.Background(), c.secretName, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch secret %s/%s with token: %w", c.namespace, c.secretName, err)
	}
	return nil
}

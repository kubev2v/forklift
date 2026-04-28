package aap

import (
	"context"
	"fmt"
	"strings"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetTokenFromSecret reads the AAP API token from a Kubernetes Secret referenced by ref.
// The Secret is loaded from defaultNamespace (typically the migration plan namespace for legacy hooks).
// If ref.Namespace is set, it must equal defaultNamespace.
func GetTokenFromSecret(ctx context.Context, k8sClient client.Client, defaultNamespace string, ref *core.ObjectReference) (string, error) {
	if ref == nil || strings.TrimSpace(ref.Name) == "" {
		return "", fmt.Errorf("tokenSecret must be set with a non-empty name")
	}
	if strings.TrimSpace(ref.Namespace) != "" && ref.Namespace != defaultNamespace {
		return "", fmt.Errorf(
			"tokenSecret namespace %q must be empty or match the plan namespace %q",
			ref.Namespace, defaultNamespace)
	}
	return GetTokenFromSecretName(ctx, k8sClient, defaultNamespace, ref.Name)
}

// GetTokenFromSecretName reads the token from a Secret by name in the given namespace.
func GetTokenFromSecretName(ctx context.Context, k8sClient client.Client, namespace, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("token secret name must be non-empty")
	}
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return "", fmt.Errorf("namespace must be non-empty")
	}
	return readTokenFromSecret(ctx, k8sClient, ns, name)
}

// readTokenFromSecret loads Secret ns/name and returns its token string.
func readTokenFromSecret(ctx context.Context, k8sClient client.Client, namespace, name string) (string, error) {
	secret := &core.Secret{}
	err := k8sClient.Get(
		ctx,
		types.NamespacedName{Namespace: namespace, Name: name},
		secret,
	)
	if err != nil {
		return "", liberr.Wrap(err, fmt.Sprintf("failed to get secret %s/%s", namespace, name))
	}
	return tokenStringFromSecretData(secret.Data, namespace, name)
}

func tokenStringFromSecretData(data map[string][]byte, ns, name string) (string, error) {
	tokenBytes, ok := data["token"]
	if !ok {
		return "", fmt.Errorf("secret %s/%s does not contain 'token' key", ns, name)
	}
	if strings.TrimSpace(string(tokenBytes)) == "" {
		return "", fmt.Errorf("secret %s/%s contains an empty 'token' value", ns, name)
	}
	return string(tokenBytes), nil
}

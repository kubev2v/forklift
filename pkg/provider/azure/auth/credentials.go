package auth

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
)

const (
	TenantID       = "tenantId"
	SubscriptionID = "subscriptionId"
	ClientID       = "clientId"
	ClientSecret   = "clientSecret"
	ResourceGroup  = "resourceGroup"
)

// Credentials holds the parsed Azure service principal fields from a Kubernetes secret.
type Credentials struct {
	TenantID       string
	SubscriptionID string
	ClientID       string
	ClientSecret   string
	ResourceGroup  string
}

// ExtractCredentials validates and extracts Azure credentials from a Kubernetes secret.
func ExtractCredentials(secret *core.Secret) (*Credentials, error) {
	if secret == nil {
		return nil, fmt.Errorf("secret is nil")
	}

	creds := &Credentials{}

	tenantIDBytes, found := secret.Data[TenantID]
	if !found || len(tenantIDBytes) == 0 {
		return nil, fmt.Errorf("tenantId not found in secret")
	}
	creds.TenantID = string(tenantIDBytes)

	subscriptionIDBytes, found := secret.Data[SubscriptionID]
	if !found || len(subscriptionIDBytes) == 0 {
		return nil, fmt.Errorf("subscriptionId not found in secret")
	}
	creds.SubscriptionID = string(subscriptionIDBytes)

	clientIDBytes, found := secret.Data[ClientID]
	if !found || len(clientIDBytes) == 0 {
		return nil, fmt.Errorf("clientId not found in secret")
	}
	creds.ClientID = string(clientIDBytes)

	clientSecretBytes, found := secret.Data[ClientSecret]
	if !found || len(clientSecretBytes) == 0 {
		return nil, fmt.Errorf("clientSecret not found in secret")
	}
	creds.ClientSecret = string(clientSecretBytes)

	if rg, found := secret.Data[ResourceGroup]; found && len(rg) > 0 {
		creds.ResourceGroup = string(rg)
	}

	return creds, nil
}

// NewClientSecretCredential creates an Azure SDK credential from the extracted credentials.
func NewClientSecretCredential(creds *Credentials) (*azidentity.ClientSecretCredential, error) {
	credential, err := azidentity.NewClientSecretCredential(
		creds.TenantID, creds.ClientID, creds.ClientSecret, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create Azure credentials")
	}
	return credential, nil
}

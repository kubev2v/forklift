package auth

import (
	"testing"

	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExtractCredentials_Valid(t *testing.T) {
	secret := &core.Secret{
		ObjectMeta: meta.ObjectMeta{Name: "test-secret"},
		Data: map[string][]byte{
			"tenantId":       []byte("tenant-123"),
			"subscriptionId": []byte("sub-456"),
			"clientId":       []byte("client-789"),
			"clientSecret":   []byte("secret-abc"),
			"resourceGroup":  []byte("my-rg"),
		},
	}

	creds, err := ExtractCredentials(secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.TenantID != "tenant-123" {
		t.Errorf("TenantID = %q, want %q", creds.TenantID, "tenant-123")
	}
	if creds.SubscriptionID != "sub-456" {
		t.Errorf("SubscriptionID = %q, want %q", creds.SubscriptionID, "sub-456")
	}
	if creds.ClientID != "client-789" {
		t.Errorf("ClientID = %q, want %q", creds.ClientID, "client-789")
	}
	if creds.ClientSecret != "secret-abc" {
		t.Errorf("ClientSecret = %q, want %q", creds.ClientSecret, "secret-abc")
	}
	if creds.ResourceGroup != "my-rg" {
		t.Errorf("ResourceGroup = %q, want %q", creds.ResourceGroup, "my-rg")
	}
}

func TestExtractCredentials_NilSecret(t *testing.T) {
	_, err := ExtractCredentials(nil)
	if err == nil {
		t.Fatal("expected error for nil secret")
	}
}

func TestExtractCredentials_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		data map[string][]byte
	}{
		{"missing tenantId", map[string][]byte{
			"subscriptionId": []byte("sub"),
			"clientId":       []byte("id"),
			"clientSecret":   []byte("sec"),
		}},
		{"missing subscriptionId", map[string][]byte{
			"tenantId":     []byte("t"),
			"clientId":     []byte("id"),
			"clientSecret": []byte("sec"),
		}},
		{"missing clientId", map[string][]byte{
			"tenantId":       []byte("t"),
			"subscriptionId": []byte("sub"),
			"clientSecret":   []byte("sec"),
		}},
		{"missing clientSecret", map[string][]byte{
			"tenantId":       []byte("t"),
			"subscriptionId": []byte("sub"),
			"clientId":       []byte("id"),
		}},
		{"empty tenantId", map[string][]byte{
			"tenantId":       []byte(""),
			"subscriptionId": []byte("sub"),
			"clientId":       []byte("id"),
			"clientSecret":   []byte("sec"),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := &core.Secret{
				ObjectMeta: meta.ObjectMeta{Name: "test"},
				Data:       tt.data,
			}
			_, err := ExtractCredentials(secret)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

func TestExtractCredentials_OptionalResourceGroup(t *testing.T) {
	secret := &core.Secret{
		ObjectMeta: meta.ObjectMeta{Name: "test"},
		Data: map[string][]byte{
			"tenantId":       []byte("t"),
			"subscriptionId": []byte("sub"),
			"clientId":       []byte("id"),
			"clientSecret":   []byte("sec"),
		},
	}

	creds, err := ExtractCredentials(secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.ResourceGroup != "" {
		t.Errorf("ResourceGroup = %q, want empty", creds.ResourceGroup)
	}
}

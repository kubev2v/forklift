package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/provider/azure/auth"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var log = logging.WithName("azure|client")

type Client struct {
	*plancontext.Context
	computeClient  ComputeAPI
	snapshotClient SnapshotAPI
	subscriptionID string
	resourceGroup  string
	snapshotRG     string
	snapshotSku    string
	targetRegion   string
}

func (r *Client) Connect() error {
	log.V(1).Info("Connecting Azure client")

	provider := r.Source.Provider
	secret, err := r.getProviderSecret(provider)
	if err != nil {
		return liberr.Wrap(err)
	}

	creds, err := auth.ExtractCredentials(secret)
	if err != nil {
		return liberr.Wrap(err)
	}
	if creds.ResourceGroup == "" {
		return fmt.Errorf("missing resourceGroup in provider secret")
	}

	r.subscriptionID = creds.SubscriptionID
	r.resourceGroup = creds.ResourceGroup
	r.snapshotRG = provider.Spec.Settings[api.AzureSnapshotRG]
	if r.snapshotRG == "" {
		r.snapshotRG = r.resourceGroup
	}
	r.snapshotSku = provider.Spec.Settings[api.AzureSnapshotSku]
	if r.snapshotSku == "" {
		r.snapshotSku = "Standard_LRS"
	}
	r.targetRegion = provider.Spec.Settings[api.AzureTargetRegion]

	credential, err := auth.NewClientSecretCredential(creds)
	if err != nil {
		return err
	}

	vmClient, err := armcompute.NewVirtualMachinesClient(r.subscriptionID, credential, nil)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.computeClient = vmClient

	snapClient, err := armcompute.NewSnapshotsClient(r.subscriptionID, credential, nil)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.snapshotClient = snapClient

	log.Info("Azure client connected",
		"subscription", r.subscriptionID,
		"resourceGroup", r.resourceGroup)
	return nil
}

func (r *Client) Close() {}

func (r *Client) getProviderSecret(provider *api.Provider) (*core.Secret, error) {
	secret := &core.Secret{}
	ref := provider.Spec.Secret
	err := r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		secret)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return secret, nil
}

func (r *Client) getComputeClient() (ComputeAPI, error) {
	if r.computeClient == nil {
		return nil, fmt.Errorf("compute client not initialized, call Connect() first")
	}
	return r.computeClient, nil
}

func (r *Client) getSnapshotClient() (SnapshotAPI, error) {
	if r.snapshotClient == nil {
		return nil, fmt.Errorf("snapshot client not initialized, call Connect() first")
	}
	return r.snapshotClient, nil
}

func (r *Client) getResourceGroup() string {
	return r.resourceGroup
}

func (r *Client) getSnapshotResourceGroup() string {
	if r.snapshotRG != "" {
		return r.snapshotRG
	}
	return r.resourceGroup
}

func (r *Client) getSnapshotSku() string {
	if r.snapshotSku != "" {
		return r.snapshotSku
	}
	return "Standard_LRS"
}

func (r *Client) IsCrossRegion() bool {
	return r.targetRegion != ""
}

func (r *Client) GetTargetRegion() string {
	return r.targetRegion
}

func extractDiskName(diskID string) string {
	if strings.Contains(diskID, "/") {
		parts := strings.Split(diskID, "/")
		return parts[len(parts)-1]
	}
	return diskID
}

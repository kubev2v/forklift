package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/provider/azure/auth"
	core "k8s.io/api/core/v1"
)

type Client struct {
	azureClient    AzureAPI
	subscriptionID string
	resourceGroup  string
}

func New(provider *api.Provider, secret *core.Secret) (*Client, error) {
	creds, err := auth.ExtractCredentials(secret)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	if creds.ResourceGroup == "" {
		return nil, fmt.Errorf("resourceGroup not found in secret")
	}

	credential, err := auth.NewClientSecretCredential(creds)
	if err != nil {
		return nil, err
	}

	vmClient, err := armcompute.NewVirtualMachinesClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create VM client")
	}

	vmSizesClient, err := armcompute.NewVirtualMachineSizesClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create VM sizes client")
	}

	diskClient, err := armcompute.NewDisksClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create disk client")
	}

	vnetClient, err := armnetwork.NewVirtualNetworksClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create VNet client")
	}

	subnetClient, err := armnetwork.NewSubnetsClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create subnet client")
	}

	return &Client{
		azureClient: &sdkClient{
			vmClient:      vmClient,
			vmSizesClient: vmSizesClient,
			diskClient:    diskClient,
			vnetClient:    vnetClient,
			subnetClient:  subnetClient,
		},
		subscriptionID: creds.SubscriptionID,
		resourceGroup:  creds.ResourceGroup,
	}, nil
}

// ExtractCredentials delegates to the shared auth package for backward compatibility.
func ExtractCredentials(secret *core.Secret) (tenantID, subscriptionID, clientID, clientSecret string, err error) {
	creds, e := auth.ExtractCredentials(secret)
	if e != nil {
		err = e
		return
	}
	return creds.TenantID, creds.SubscriptionID, creds.ClientID, creds.ClientSecret, nil
}

func (c *Client) GetResourceGroup() string {
	return c.resourceGroup
}

func (c *Client) GetSubscriptionID() string {
	return c.subscriptionID
}

func (c *Client) SetAzureClient(client AzureAPI) {
	c.azureClient = client
}

func NewWithClient(azureClient AzureAPI, subscriptionID, resourceGroup string) *Client {
	return &Client{
		azureClient:    azureClient,
		subscriptionID: subscriptionID,
		resourceGroup:  resourceGroup,
	}
}

func (c *Client) ListVirtualMachines(ctx context.Context) ([]*armcompute.VirtualMachine, error) {
	return c.azureClient.ListVirtualMachines(ctx, c.resourceGroup)
}

func (c *Client) GetVMInstanceView(ctx context.Context, vmName string) (*armcompute.VirtualMachineInstanceView, error) {
	return c.azureClient.GetVMInstanceView(ctx, c.resourceGroup, vmName)
}

func (c *Client) ListVMSizes(ctx context.Context, location string) ([]*armcompute.VirtualMachineSize, error) {
	return c.azureClient.ListVMSizes(ctx, location)
}

func (c *Client) ListDisks(ctx context.Context) ([]*armcompute.Disk, error) {
	return c.azureClient.ListDisks(ctx, c.resourceGroup)
}

func (c *Client) ListVirtualNetworks(ctx context.Context) ([]*armnetwork.VirtualNetwork, error) {
	return c.azureClient.ListVirtualNetworks(ctx, c.resourceGroup)
}

func (c *Client) ListSubnets(ctx context.Context, vnetName string) ([]*armnetwork.Subnet, error) {
	return c.azureClient.ListSubnets(ctx, c.resourceGroup, vnetName)
}

// sdkClient implements AzureAPI using real Azure SDK clients.
type sdkClient struct {
	vmClient      *armcompute.VirtualMachinesClient
	vmSizesClient *armcompute.VirtualMachineSizesClient
	diskClient    *armcompute.DisksClient
	vnetClient    *armnetwork.VirtualNetworksClient
	subnetClient  *armnetwork.SubnetsClient
}

func (s *sdkClient) ListVirtualMachines(ctx context.Context, resourceGroup string) ([]*armcompute.VirtualMachine, error) {
	var vms []*armcompute.VirtualMachine
	pager := s.vmClient.NewListPager(resourceGroup, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, liberr.Wrap(err, "failed to list virtual machines")
		}
		vms = append(vms, page.Value...)
	}
	return vms, nil
}

func (s *sdkClient) GetVMInstanceView(ctx context.Context, resourceGroup string, vmName string) (*armcompute.VirtualMachineInstanceView, error) {
	resp, err := s.vmClient.InstanceView(ctx, resourceGroup, vmName, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to get VM instance view")
	}
	return &resp.VirtualMachineInstanceView, nil
}

func (s *sdkClient) ListVMSizes(ctx context.Context, location string) ([]*armcompute.VirtualMachineSize, error) {
	var sizes []*armcompute.VirtualMachineSize
	pager := s.vmSizesClient.NewListPager(location, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, liberr.Wrap(err, "failed to list VM sizes")
		}
		sizes = append(sizes, page.Value...)
	}
	return sizes, nil
}

func (s *sdkClient) ListDisks(ctx context.Context, resourceGroup string) ([]*armcompute.Disk, error) {
	var disks []*armcompute.Disk
	pager := s.diskClient.NewListByResourceGroupPager(resourceGroup, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, liberr.Wrap(err, "failed to list disks")
		}
		disks = append(disks, page.Value...)
	}
	return disks, nil
}

func (s *sdkClient) ListVirtualNetworks(ctx context.Context, resourceGroup string) ([]*armnetwork.VirtualNetwork, error) {
	var vnets []*armnetwork.VirtualNetwork
	pager := s.vnetClient.NewListPager(resourceGroup, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, liberr.Wrap(err, "failed to list virtual networks")
		}
		vnets = append(vnets, page.Value...)
	}
	return vnets, nil
}

func (s *sdkClient) ListSubnets(ctx context.Context, resourceGroup string, vnetName string) ([]*armnetwork.Subnet, error) {
	var subnets []*armnetwork.Subnet
	pager := s.subnetClient.NewListPager(resourceGroup, vnetName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, liberr.Wrap(err, "failed to list subnets")
		}
		subnets = append(subnets, page.Value...)
	}
	return subnets, nil
}

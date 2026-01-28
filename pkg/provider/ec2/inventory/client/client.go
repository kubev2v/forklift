package client

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
)

// Secret fields
const (
	Region          = "region"
	AccessKeyID     = "accessKeyId"
	SecretAccessKey = "secretAccessKey"
)

// Client wraps AWS SDK client
type Client struct {
	ec2Client EC2API
	region    string
}

// New creates a new EC2 client from provider and secret
func New(provider *api.Provider, secret *core.Secret) (*Client, error) {
	region, accessKeyID, secretAccessKey, err := ExtractCredentials(secret)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	// Create AWS config
	cfg, err := createAWSConfig(context.TODO(), region, accessKeyID, secretAccessKey)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create AWS config")
	}

	return &Client{
		ec2Client: ec2.NewFromConfig(cfg),
		region:    region,
	}, nil
}

// ExtractCredentials extracts AWS credentials from secret
func ExtractCredentials(secret *core.Secret) (region, accessKeyID, secretAccessKey string, err error) {
	if secret == nil {
		err = fmt.Errorf("secret is nil")
		return
	}

	// Extract region (required)
	regionBytes, found := secret.Data[Region]
	if !found || len(regionBytes) == 0 {
		err = fmt.Errorf("region not found in secret")
		return
	}
	region = string(regionBytes)

	// Extract access key ID (optional if using IAM role)
	if keyIDBytes, found := secret.Data[AccessKeyID]; found {
		accessKeyID = string(keyIDBytes)
	}

	// Extract secret access key (optional if using IAM role)
	if secretKeyBytes, found := secret.Data[SecretAccessKey]; found {
		secretAccessKey = string(secretKeyBytes)
	}

	// Validate: if one is provided, both must be provided
	if (accessKeyID == "" && secretAccessKey != "") || (accessKeyID != "" && secretAccessKey == "") {
		err = fmt.Errorf("both accessKeyId and secretAccessKey must be provided together")
		return
	}

	return
}

// createAWSConfig creates AWS SDK configuration
func createAWSConfig(ctx context.Context, region, accessKeyID, secretAccessKey string) (aws.Config, error) {
	optFns := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	// If access keys are provided, use them; otherwise use default credential chain (IAM role)
	if accessKeyID != "" && secretAccessKey != "" {
		optFns = append(optFns, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		))
	}

	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return aws.Config{}, err
	}

	return cfg, nil
}

// GetRegion returns the configured region
// Used by inventory collector to discover available resources in the configured region.
func (c *Client) GetRegion() string {
	return c.region
}

// SetEC2Client sets the EC2 API client. Used for testing with mock clients.
func (c *Client) SetEC2Client(client EC2API) {
	c.ec2Client = client
}

// NewWithClient creates a new Client with a custom EC2API implementation.
// Used for testing with mock clients.
func NewWithClient(ec2Client EC2API, region string) *Client {
	return &Client{
		ec2Client: ec2Client,
		region:    region,
	}
}

// DescribeInstances fetches all EC2 instances in the configured region.
// Returns complete instance details including state, type, network, and storage configuration.
// Used by inventory collector to discover VMs available for migration.
func (c *Client) DescribeInstances(ctx context.Context) ([]ec2types.Instance, error) {
	var instances []ec2types.Instance

	// Paginate through all instances
	paginator := ec2.NewDescribeInstancesPaginator(c.ec2Client, &ec2.DescribeInstancesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, liberr.Wrap(err, "failed to describe instances")
		}

		for _, reservation := range output.Reservations {
			instances = append(instances, reservation.Instances...)
		}
	}

	return instances, nil
}

// DescribeVolumes fetches all EBS volumes in the configured region.
// Returns volume details including size, type, state, and attachments.
// Used by inventory collector to discover available storage resources.
func (c *Client) DescribeVolumes(ctx context.Context) ([]ec2types.Volume, error) {
	var volumes []ec2types.Volume

	// Paginate through all volumes
	paginator := ec2.NewDescribeVolumesPaginator(c.ec2Client, &ec2.DescribeVolumesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, liberr.Wrap(err, "failed to describe volumes")
		}

		volumes = append(volumes, output.Volumes...)
	}

	return volumes, nil
}

// DescribeVpcs fetches all VPCs
// A VPC is a logically isolated virtual network within AWS
func (c *Client) DescribeVpcs(ctx context.Context) ([]ec2types.Vpc, error) {
	output, err := c.ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, liberr.Wrap(err, "failed to describe VPCs")
	}

	return output.Vpcs, nil
}

// DescribeSubnets fetches all subnets
// A subnet is a range of IP addresses within a VPC.
// It's a subdivision of the VPC that allows users to segment the network.
func (c *Client) DescribeSubnets(ctx context.Context) ([]ec2types.Subnet, error) {
	var subnets []ec2types.Subnet

	// Paginate through all subnets
	paginator := ec2.NewDescribeSubnetsPaginator(c.ec2Client, &ec2.DescribeSubnetsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, liberr.Wrap(err, "failed to describe subnets")
		}

		subnets = append(subnets, output.Subnets...)
	}

	return subnets, nil
}

// DescribeSecurityGroups fetches all security groups
// A security group is a virtual firewall that controls the traffic for one or more instances.
func (c *Client) DescribeSecurityGroups(ctx context.Context) ([]ec2types.SecurityGroup, error) {
	var securityGroups []ec2types.SecurityGroup

	// Paginate through all security groups
	paginator := ec2.NewDescribeSecurityGroupsPaginator(c.ec2Client, &ec2.DescribeSecurityGroupsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, liberr.Wrap(err, "failed to describe security groups")
		}

		securityGroups = append(securityGroups, output.SecurityGroups...)
	}

	return securityGroups, nil
}

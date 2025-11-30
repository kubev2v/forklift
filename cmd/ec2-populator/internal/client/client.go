package client

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// Client wraps AWS EC2 client.
type Client struct {
	EC2    *ec2.Client
	region string
}

// New creates AWS client for the specified region.
// Loads credentials from environment (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY).
// The secret referenced by the Ec2VolumePopulator CR should contain these standard AWS env var names.
func New(ctx context.Context, region string) (*Client, error) {
	if region == "" {
		return nil, fmt.Errorf("region is required")
	}

	// Load AWS config using standard environment variables
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{
		EC2:    ec2.NewFromConfig(cfg),
		region: region,
	}, nil
}

// Region returns the configured AWS region.
func (c *Client) Region() string {
	return c.region
}

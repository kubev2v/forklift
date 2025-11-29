package client

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ec2client "github.com/kubev2v/forklift/pkg/provider/ec2/inventory/client"
)

// log provides structured logging for the EC2 client with the "ec2|client" name prefix.
// This is a package-level logger shared by all client instances.
var log = logging.WithName("ec2|client")

// Client provides AWS EC2 API operations: instance power management, snapshots, volumes.
// Maintains authenticated AWS connection using provider secret credentials for specific region.
type Client struct {
	*plancontext.Context             // Plan context with provider config, secrets, K8s client
	ec2Client            *ec2.Client // AWS SDK client for EC2 operations
	region               string      // AWS region (e.g., "us-east-1")
}

// Connect initializes AWS EC2 client with credentials from provider secret.
// Extracts AWS credentials and region, creates authenticated SDK config, initializes client.
// Supports explicit keys or IAM role (default credential chain). Must be called before EC2 operations.
func (r *Client) Connect() error {
	if r.Context == nil {
		return fmt.Errorf("context is nil")
	}
	if r.Source.Provider == nil {
		return fmt.Errorf("source provider is nil")
	}
	if r.Source.Secret == nil {
		return fmt.Errorf("source secret is nil")
	}

	region, accessKeyID, secretAccessKey, err := ec2client.ExtractCredentials(r.Source.Secret)
	if err != nil {
		return liberr.Wrap(err)
	}

	cfg, err := createAWSConfig(context.TODO(), region, accessKeyID, secretAccessKey)
	if err != nil {
		return liberr.Wrap(err, "failed to create AWS config")
	}

	r.ec2Client = ec2.NewFromConfig(cfg)
	r.region = region
	log.Info("EC2 client connected", "region", region)
	return nil
}

// Close releases the EC2 client. Safe to call multiple times.
// Must call Connect() again before performing EC2 operations.
func (r *Client) Close() {
	r.ec2Client = nil
}

// Disconnect is a no-op for EC2 - AWS SDK clients are stateless, no persistent connections.
func (r *Client) Disconnect() error {
	return nil
}

// DetachDisks is a no-op for EC2 - snapshots created from attached volumes after instance shutdown.
func (r *Client) DetachDisks(vmRef ref.Ref) error {
	return nil
}

// getEC2Client returns initialized EC2 client or error if not connected.
func (r *Client) getEC2Client() (*ec2.Client, error) {
	if r.ec2Client == nil {
		return nil, fmt.Errorf("EC2 client not initialized")
	}
	return r.ec2Client, nil
}

// createAWSConfig creates AWS SDK config with static credentials or default credential chain (IAM role).
// If keys empty, uses environment vars, ~/.aws/credentials, IAM role, or EKS pod identity.
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

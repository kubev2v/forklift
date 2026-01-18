package client

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ec2client "github.com/kubev2v/forklift/pkg/provider/ec2/inventory/client"
	core "k8s.io/api/core/v1"
)

// log provides structured logging for the EC2 client with the "ec2|client" name prefix.
// This is a package-level logger shared by all client instances.
var log = logging.WithName("ec2|client")

// Client provides AWS EC2 API operations: instance power management, snapshots, volumes.
// Supports both same-account and cross-account migrations.
// In cross-account mode, sourceClient handles snapshots/power, targetClient handles volumes.
type Client struct {
	*plancontext.Context        // Plan context with provider config, secrets, K8s client
	sourceClient         EC2API // Source account client (snapshots, power operations)
	targetClient         EC2API // Target account client (volume operations), same as source in same-account mode
	targetSTS            STSAPI // Target account STS client (for GetCallerIdentity)
	region               string // AWS region (e.g., "us-east-1")
	crossAccount         bool   // True if cross-account mode is enabled
}

// Connect initializes AWS EC2 clients with credentials from provider secret.
// If targetAccessKeyId and targetSecretAccessKey are provided in the secret,
// cross-account mode is enabled with separate clients for source and target accounts.
// Otherwise, same-account mode uses a single client for all operations.
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

	// Extract source credentials
	region, accessKeyID, secretAccessKey, err := ec2client.ExtractCredentials(r.Source.Secret)
	if err != nil {
		return liberr.Wrap(err)
	}

	cfg, err := createAWSConfig(context.TODO(), region, accessKeyID, secretAccessKey)
	if err != nil {
		return liberr.Wrap(err, "failed to create source AWS config")
	}

	r.sourceClient = ec2.NewFromConfig(cfg)
	r.region = region

	// Check for target account credentials (cross-account mode)
	targetKeyID, targetSecret := extractTargetCredentials(r.Source.Secret)
	if targetKeyID != "" && targetSecret != "" {
		// Cross-account mode
		targetCfg, err := createAWSConfig(context.TODO(), region, targetKeyID, targetSecret)
		if err != nil {
			return liberr.Wrap(err, "failed to create target AWS config")
		}
		r.targetClient = ec2.NewFromConfig(targetCfg)
		r.targetSTS = sts.NewFromConfig(targetCfg)
		r.crossAccount = true
		log.Info("EC2 client connected (cross-account mode)", "region", region)
	} else {
		// Same-account mode - target = source
		r.targetClient = r.sourceClient
		r.targetSTS = sts.NewFromConfig(cfg)
		r.crossAccount = false
		log.Info("EC2 client connected (same-account mode)", "region", region)
	}

	return nil
}

// IsCrossAccount returns true if cross-account mode is enabled.
func (r *Client) IsCrossAccount() bool {
	return r.crossAccount
}

// GetTargetAccountID retrieves the AWS account ID for the target account using STS.
func (r *Client) GetTargetAccountID() (string, error) {
	if r.targetSTS == nil {
		return "", fmt.Errorf("target STS client not initialized")
	}
	result, err := r.targetSTS.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Error(err, "Failed to get target account ID")
		return "", liberr.Wrap(err)
	}
	return *result.Account, nil
}

// extractTargetCredentials extracts optional target account credentials from the secret.
// Returns empty strings if not present.
func extractTargetCredentials(secret *core.Secret) (string, string) {
	if secret == nil || secret.Data == nil {
		return "", ""
	}
	targetKeyID := string(secret.Data["targetAccessKeyId"])
	targetSecret := string(secret.Data["targetSecretAccessKey"])
	return targetKeyID, targetSecret
}

// Close releases the EC2 clients. Safe to call multiple times.
// Must call Connect() again before performing EC2 operations.
func (r *Client) Close() {
	r.sourceClient = nil
	r.targetClient = nil
	r.crossAccount = false
}

// getSourceClient returns the source account EC2 client for snapshot and power operations.
func (r *Client) getSourceClient() (EC2API, error) {
	if r.sourceClient == nil {
		return nil, fmt.Errorf("source EC2 client not initialized")
	}
	return r.sourceClient, nil
}

// getTargetClient returns the target account EC2 client for volume operations.
// In same-account mode, this returns the same client as getSourceClient.
func (r *Client) getTargetClient() (EC2API, error) {
	if r.targetClient == nil {
		return nil, fmt.Errorf("target EC2 client not initialized")
	}
	return r.targetClient, nil
}

// SetSourceClient sets the source EC2 client. Used for testing with mock clients.
func (r *Client) SetSourceClient(client EC2API) {
	r.sourceClient = client
}

// SetTargetClient sets the target EC2 client. Used for testing with mock clients.
func (r *Client) SetTargetClient(client EC2API) {
	r.targetClient = client
}

// SetTargetSTS sets the target STS client. Used for testing with mock clients.
func (r *Client) SetTargetSTS(client STSAPI) {
	r.targetSTS = client
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

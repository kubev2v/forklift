package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TargetCredentialsMissing = "TargetCredentialsMissing"
	TargetCredentialsInvalid = "TargetCredentialsInvalid"
	ValidationFailed         = "ValidationFailed"
)

type Validator struct{}

func (v *Validator) Validate(provider *api.Provider, secret *core.Secret, _ client.Client) {
	if provider.Status.HasBlockerCondition() {
		return
	}
	if secret == nil {
		return
	}

	region := string(secret.Data["region"])
	targetKeyID := string(secret.Data["targetAccessKeyId"])
	targetSecretKey := string(secret.Data["targetSecretAccessKey"])
	if region == "" || targetKeyID == "" || targetSecretKey == "" {
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(libcnd.Condition{
			Type:     TargetCredentialsMissing,
			Status:   libcnd.True,
			Category: libcnd.Critical,
			Reason:   "TargetCredentialsRequired",
			Message:  "The provider secret must include 'region', 'targetAccessKeyId', and 'targetSecretAccessKey'. Recreate the provider with --auto-target-credentials or provide them manually.",
		})
		provider.Status.DeleteCondition(TargetCredentialsInvalid)
		return
	}
	provider.Status.DeleteCondition(TargetCredentialsMissing)

	_, err := callerAccount(region, targetKeyID, targetSecretKey)
	if err != nil {
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(libcnd.Condition{
			Type:     TargetCredentialsInvalid,
			Status:   libcnd.True,
			Category: libcnd.Critical,
			Reason:   "GetCallerIdentityFailed",
			Message:  fmt.Sprintf("The target access keys could not be used to identify an AWS account: %s", err.Error()),
		})
		return
	}
	provider.Status.DeleteCondition(TargetCredentialsInvalid)
}

func callerAccount(region, accessKeyID, secretAccessKey string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		),
	)
	if err != nil {
		return "", err
	}

	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	if out.Account == nil || *out.Account == "" {
		return "", fmt.Errorf("STS GetCallerIdentity returned an empty account")
	}
	return *out.Account, nil
}

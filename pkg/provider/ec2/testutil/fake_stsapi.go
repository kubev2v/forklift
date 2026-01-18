// Package testutil provides test utilities for EC2 provider unit tests.
package testutil

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	controllerclient "github.com/kubev2v/forklift/pkg/provider/ec2/controller/client"
)

// STSMethod represents an STS API method name for error injection.
type STSMethod string

// STS API method constants for type-safe error injection.
const (
	MethodGetCallerIdentity STSMethod = "GetCallerIdentity"
)

// FakeSTSAPI is a fake implementation of the STS API for testing.
// It implements controller/client.STSAPI interface.
type FakeSTSAPI struct {
	mu sync.RWMutex

	// AccountID is the account ID returned by GetCallerIdentity
	AccountID string

	// Arn is the ARN returned by GetCallerIdentity
	Arn string

	// UserID is the user ID returned by GetCallerIdentity
	UserID string

	// Error injection - map of method to error
	Errors map[STSMethod]error

	// Call tracking
	Calls []STSAPICall
}

// STSAPICall records a call to the fake STS API for verification in tests.
type STSAPICall struct {
	Method STSMethod
	Input  interface{}
}

// NewFakeSTSAPI creates a new FakeSTSAPI with empty state.
func NewFakeSTSAPI() *FakeSTSAPI {
	return &FakeSTSAPI{
		Errors: make(map[STSMethod]error),
		Calls:  []STSAPICall{},
	}
}

// Compile-time check to ensure FakeSTSAPI implements STSAPI
var _ controllerclient.STSAPI = (*FakeSTSAPI)(nil)

// GetCallerIdentity implements STSAPI.
func (f *FakeSTSAPI) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, STSAPICall{Method: MethodGetCallerIdentity, Input: params})

	if err := f.Errors[MethodGetCallerIdentity]; err != nil {
		return nil, err
	}

	return &sts.GetCallerIdentityOutput{
		Account: aws.String(f.AccountID),
		Arn:     aws.String(f.Arn),
		UserId:  aws.String(f.UserID),
	}, nil
}

// Reset clears all state, recorded calls, and errors.
func (f *FakeSTSAPI) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.AccountID = ""
	f.Arn = ""
	f.UserID = ""
	f.Errors = make(map[STSMethod]error)
	f.Calls = []STSAPICall{}
}

// SetAccountID sets the account ID to return.
func (f *FakeSTSAPI) SetAccountID(accountID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.AccountID = accountID
}

// SetArn sets the ARN to return.
func (f *FakeSTSAPI) SetArn(arn string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Arn = arn
}

// SetUserID sets the user ID to return.
func (f *FakeSTSAPI) SetUserID(userID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.UserID = userID
}

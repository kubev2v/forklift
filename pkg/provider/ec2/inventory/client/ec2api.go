package client

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// EC2API defines the EC2 operations used by the inventory client.
// This interface allows for mocking AWS EC2 calls in unit tests.
// The real AWS SDK ec2.Client implements this interface.
//
// The interface includes methods needed for paginators:
// - DescribeInstances (used by NewDescribeInstancesPaginator)
// - DescribeVolumes (used by NewDescribeVolumesPaginator)
// - DescribeSubnets (used by NewDescribeSubnetsPaginator)
// - DescribeSecurityGroups (used by NewDescribeSecurityGroupsPaginator)
// - DescribeVpcs (not paginated in our usage)
type EC2API interface {
	// Instance operations
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)

	// Volume operations
	DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)

	// Network operations
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
}

// Compile-time check to ensure *ec2.Client implements EC2API
var _ EC2API = (*ec2.Client)(nil)

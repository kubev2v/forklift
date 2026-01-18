package testutil

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/provider/testutil"
	core "k8s.io/api/core/v1"
)

// InstanceBuilder provides a fluent interface for building test EC2 instances.
type InstanceBuilder struct {
	instance ec2types.Instance
}

// NewInstanceBuilder creates a new InstanceBuilder with default values.
func NewInstanceBuilder(id, name string) *InstanceBuilder {
	return &InstanceBuilder{
		instance: ec2types.Instance{
			InstanceId:   aws.String(id),
			InstanceType: ec2types.InstanceTypeT2Micro,
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNameRunning,
				Code: aws.Int32(16), // Running
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String(name)},
			},
			Placement: &ec2types.Placement{
				AvailabilityZone: aws.String("us-east-1a"),
			},
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{},
			NetworkInterfaces:   []ec2types.InstanceNetworkInterface{},
		},
	}
}

// WithInstanceType sets the instance type.
func (b *InstanceBuilder) WithInstanceType(instanceType ec2types.InstanceType) *InstanceBuilder {
	b.instance.InstanceType = instanceType
	return b
}

// WithState sets the instance state.
func (b *InstanceBuilder) WithState(state ec2types.InstanceStateName) *InstanceBuilder {
	b.instance.State = &ec2types.InstanceState{Name: state}
	return b
}

// WithAvailabilityZone sets the availability zone.
func (b *InstanceBuilder) WithAvailabilityZone(az string) *InstanceBuilder {
	if b.instance.Placement == nil {
		b.instance.Placement = &ec2types.Placement{}
	}
	b.instance.Placement.AvailabilityZone = aws.String(az)
	return b
}

// WithVolume adds a block device mapping (EBS volume attachment).
func (b *InstanceBuilder) WithVolume(deviceName, volumeID string) *InstanceBuilder {
	b.instance.BlockDeviceMappings = append(b.instance.BlockDeviceMappings, ec2types.InstanceBlockDeviceMapping{
		DeviceName: aws.String(deviceName),
		Ebs: &ec2types.EbsInstanceBlockDevice{
			VolumeId: aws.String(volumeID),
			Status:   ec2types.AttachmentStatusAttached,
		},
	})
	return b
}

// WithNetworkInterface adds a network interface.
func (b *InstanceBuilder) WithNetworkInterface(subnetID, vpcID, privateIP string) *InstanceBuilder {
	b.instance.NetworkInterfaces = append(b.instance.NetworkInterfaces, ec2types.InstanceNetworkInterface{
		SubnetId:         aws.String(subnetID),
		VpcId:            aws.String(vpcID),
		PrivateIpAddress: aws.String(privateIP),
		Status:           ec2types.NetworkInterfaceStatusInUse,
	})
	return b
}

// WithTag adds a tag to the instance.
func (b *InstanceBuilder) WithTag(key, value string) *InstanceBuilder {
	b.instance.Tags = append(b.instance.Tags, ec2types.Tag{
		Key:   aws.String(key),
		Value: aws.String(value),
	})
	return b
}

// Build returns the constructed Instance.
func (b *InstanceBuilder) Build() ec2types.Instance {
	return b.instance
}

// VolumeBuilder provides a fluent interface for building test EBS volumes.
type VolumeBuilder struct {
	volume ec2types.Volume
}

// NewVolumeBuilder creates a new VolumeBuilder with default values.
func NewVolumeBuilder(id string) *VolumeBuilder {
	return &VolumeBuilder{
		volume: ec2types.Volume{
			VolumeId:         aws.String(id),
			VolumeType:       ec2types.VolumeTypeGp3,
			Size:             aws.Int32(100),
			State:            ec2types.VolumeStateAvailable,
			AvailabilityZone: aws.String("us-east-1a"),
			Tags:             []ec2types.Tag{},
			Attachments:      []ec2types.VolumeAttachment{},
		},
	}
}

// WithVolumeType sets the volume type.
func (b *VolumeBuilder) WithVolumeType(volumeType ec2types.VolumeType) *VolumeBuilder {
	b.volume.VolumeType = volumeType
	return b
}

// WithSize sets the volume size in GiB.
func (b *VolumeBuilder) WithSize(sizeGiB int32) *VolumeBuilder {
	b.volume.Size = aws.Int32(sizeGiB)
	return b
}

// WithState sets the volume state.
func (b *VolumeBuilder) WithState(state ec2types.VolumeState) *VolumeBuilder {
	b.volume.State = state
	return b
}

// WithAvailabilityZone sets the availability zone.
func (b *VolumeBuilder) WithAvailabilityZone(az string) *VolumeBuilder {
	b.volume.AvailabilityZone = aws.String(az)
	return b
}

// WithAttachment adds an instance attachment.
func (b *VolumeBuilder) WithAttachment(instanceID, device string) *VolumeBuilder {
	b.volume.Attachments = append(b.volume.Attachments, ec2types.VolumeAttachment{
		InstanceId: aws.String(instanceID),
		Device:     aws.String(device),
		State:      ec2types.VolumeAttachmentStateAttached,
		VolumeId:   b.volume.VolumeId,
	})
	b.volume.State = ec2types.VolumeStateInUse
	return b
}

// WithTag adds a tag to the volume.
func (b *VolumeBuilder) WithTag(key, value string) *VolumeBuilder {
	b.volume.Tags = append(b.volume.Tags, ec2types.Tag{
		Key:   aws.String(key),
		Value: aws.String(value),
	})
	return b
}

// WithSnapshotID sets the snapshot the volume was created from.
func (b *VolumeBuilder) WithSnapshotID(snapshotID string) *VolumeBuilder {
	b.volume.SnapshotId = aws.String(snapshotID)
	return b
}

// Build returns the constructed Volume.
func (b *VolumeBuilder) Build() ec2types.Volume {
	return b.volume
}

// SnapshotBuilder provides a fluent interface for building test EBS snapshots.
type SnapshotBuilder struct {
	snapshot ec2types.Snapshot
}

// NewSnapshotBuilder creates a new SnapshotBuilder with default values.
func NewSnapshotBuilder(id string) *SnapshotBuilder {
	return &SnapshotBuilder{
		snapshot: ec2types.Snapshot{
			SnapshotId: aws.String(id),
			State:      ec2types.SnapshotStateCompleted,
			Progress:   aws.String("100%"),
			VolumeSize: aws.Int32(100),
			Tags:       []ec2types.Tag{},
		},
	}
}

// WithVolumeID sets the source volume ID.
func (b *SnapshotBuilder) WithVolumeID(volumeID string) *SnapshotBuilder {
	b.snapshot.VolumeId = aws.String(volumeID)
	return b
}

// WithState sets the snapshot state.
func (b *SnapshotBuilder) WithState(state ec2types.SnapshotState) *SnapshotBuilder {
	b.snapshot.State = state
	return b
}

// WithProgress sets the snapshot progress.
func (b *SnapshotBuilder) WithProgress(progress string) *SnapshotBuilder {
	b.snapshot.Progress = aws.String(progress)
	return b
}

// WithVolumeSize sets the volume size in GiB.
func (b *SnapshotBuilder) WithVolumeSize(sizeGiB int32) *SnapshotBuilder {
	b.snapshot.VolumeSize = aws.Int32(sizeGiB)
	return b
}

// WithTag adds a tag to the snapshot.
func (b *SnapshotBuilder) WithTag(key, value string) *SnapshotBuilder {
	b.snapshot.Tags = append(b.snapshot.Tags, ec2types.Tag{
		Key:   aws.String(key),
		Value: aws.String(value),
	})
	return b
}

// WithForkliftTags adds standard Forklift migration tags.
func (b *SnapshotBuilder) WithForkliftTags(vmID, vmName, volumeID string) *SnapshotBuilder {
	b.snapshot.Tags = append(b.snapshot.Tags,
		ec2types.Tag{Key: aws.String("forklift.konveyor.io/vmID"), Value: aws.String(vmID)},
		ec2types.Tag{Key: aws.String("forklift.konveyor.io/vm-name"), Value: aws.String(vmName)},
		ec2types.Tag{Key: aws.String("forklift.konveyor.io/volume"), Value: aws.String(volumeID)},
	)
	return b
}

// Build returns the constructed Snapshot.
func (b *SnapshotBuilder) Build() ec2types.Snapshot {
	return b.snapshot
}

// VpcBuilder provides a fluent interface for building test VPCs.
type VpcBuilder struct {
	vpc ec2types.Vpc
}

// NewVpcBuilder creates a new VpcBuilder with default values.
func NewVpcBuilder(id string) *VpcBuilder {
	return &VpcBuilder{
		vpc: ec2types.Vpc{
			VpcId:     aws.String(id),
			CidrBlock: aws.String("10.0.0.0/16"),
			State:     ec2types.VpcStateAvailable,
			Tags:      []ec2types.Tag{},
		},
	}
}

// WithCidrBlock sets the CIDR block.
func (b *VpcBuilder) WithCidrBlock(cidr string) *VpcBuilder {
	b.vpc.CidrBlock = aws.String(cidr)
	return b
}

// WithTag adds a tag to the VPC.
func (b *VpcBuilder) WithTag(key, value string) *VpcBuilder {
	b.vpc.Tags = append(b.vpc.Tags, ec2types.Tag{
		Key:   aws.String(key),
		Value: aws.String(value),
	})
	return b
}

// Build returns the constructed Vpc.
func (b *VpcBuilder) Build() ec2types.Vpc {
	return b.vpc
}

// SubnetBuilder provides a fluent interface for building test subnets.
type SubnetBuilder struct {
	subnet ec2types.Subnet
}

// NewSubnetBuilder creates a new SubnetBuilder with default values.
func NewSubnetBuilder(id, vpcID string) *SubnetBuilder {
	return &SubnetBuilder{
		subnet: ec2types.Subnet{
			SubnetId:         aws.String(id),
			VpcId:            aws.String(vpcID),
			CidrBlock:        aws.String("10.0.1.0/24"),
			AvailabilityZone: aws.String("us-east-1a"),
			State:            ec2types.SubnetStateAvailable,
			Tags:             []ec2types.Tag{},
		},
	}
}

// WithCidrBlock sets the CIDR block.
func (b *SubnetBuilder) WithCidrBlock(cidr string) *SubnetBuilder {
	b.subnet.CidrBlock = aws.String(cidr)
	return b
}

// WithAvailabilityZone sets the availability zone.
func (b *SubnetBuilder) WithAvailabilityZone(az string) *SubnetBuilder {
	b.subnet.AvailabilityZone = aws.String(az)
	return b
}

// WithTag adds a tag to the subnet.
func (b *SubnetBuilder) WithTag(key, value string) *SubnetBuilder {
	b.subnet.Tags = append(b.subnet.Tags, ec2types.Tag{
		Key:   aws.String(key),
		Value: aws.String(value),
	})
	return b
}

// Build returns the constructed Subnet.
func (b *SubnetBuilder) Build() ec2types.Subnet {
	return b.subnet
}

// SecurityGroupBuilder provides a fluent interface for building test security groups.
type SecurityGroupBuilder struct {
	sg ec2types.SecurityGroup
}

// NewSecurityGroupBuilder creates a new SecurityGroupBuilder with default values.
func NewSecurityGroupBuilder(id, vpcID string) *SecurityGroupBuilder {
	return &SecurityGroupBuilder{
		sg: ec2types.SecurityGroup{
			GroupId:   aws.String(id),
			GroupName: aws.String(fmt.Sprintf("sg-%s", id)),
			VpcId:     aws.String(vpcID),
			Tags:      []ec2types.Tag{},
		},
	}
}

// WithGroupName sets the group name.
func (b *SecurityGroupBuilder) WithGroupName(name string) *SecurityGroupBuilder {
	b.sg.GroupName = aws.String(name)
	return b
}

// WithDescription sets the description.
func (b *SecurityGroupBuilder) WithDescription(desc string) *SecurityGroupBuilder {
	b.sg.Description = aws.String(desc)
	return b
}

// WithTag adds a tag to the security group.
func (b *SecurityGroupBuilder) WithTag(key, value string) *SecurityGroupBuilder {
	b.sg.Tags = append(b.sg.Tags, ec2types.Tag{
		Key:   aws.String(key),
		Value: aws.String(value),
	})
	return b
}

// Build returns the constructed SecurityGroup.
func (b *SecurityGroupBuilder) Build() ec2types.SecurityGroup {
	return b.sg
}

// EC2 Secret helpers

// NewEC2Secret creates a Kubernetes Secret with EC2 credentials.
func NewEC2Secret(name, namespace, region string) *core.Secret {
	return testutil.NewSecretBuilder().
		WithName(name).
		WithNamespace(namespace).
		WithData("region", region).
		WithData("accessKeyId", "AKIAIOSFODNN7EXAMPLE").
		WithData("secretAccessKey", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY").
		Build()
}

// NewEC2SecretWithTargetAccount creates a Secret with both source and target account credentials.
func NewEC2SecretWithTargetAccount(name, namespace, region string) *core.Secret {
	return testutil.NewSecretBuilder().
		WithName(name).
		WithNamespace(namespace).
		WithData("region", region).
		WithData("accessKeyId", "AKIAIOSFODNN7EXAMPLE").
		WithData("secretAccessKey", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY").
		WithData("targetAccessKeyId", "AKIATARGETACCOUNT123").
		WithData("targetSecretAccessKey", "targetSecretKey/EXAMPLE/KEY").
		Build()
}

// EC2 Provider helpers

// NewEC2ProviderWithSettings creates an EC2 Provider with common settings.
func NewEC2ProviderWithSettings(name, namespace, targetAZ string) *api.Provider {
	return testutil.NewProviderBuilder().
		WithName(name).
		WithNamespace(namespace).
		WithType(api.EC2).
		WithSecretRef(name+"-secret", namespace).
		WithSetting("target-az", targetAZ).
		Build()
}

// Sample test data factories

// NewSampleInstance creates a sample EC2 instance for testing.
func NewSampleInstance() ec2types.Instance {
	return NewInstanceBuilder("i-1234567890abcdef0", "test-vm").
		WithInstanceType(ec2types.InstanceTypeT3Medium).
		WithState(ec2types.InstanceStateNameRunning).
		WithAvailabilityZone("us-east-1a").
		WithVolume("/dev/sda1", "vol-0123456789abcdef0").
		WithVolume("/dev/sdb", "vol-0123456789abcdef1").
		WithNetworkInterface("subnet-12345", "vpc-12345", "10.0.1.100").
		Build()
}

// NewSampleVolume creates a sample EBS volume for testing.
func NewSampleVolume() ec2types.Volume {
	return NewVolumeBuilder("vol-0123456789abcdef0").
		WithVolumeType(ec2types.VolumeTypeGp3).
		WithSize(100).
		WithState(ec2types.VolumeStateInUse).
		WithAvailabilityZone("us-east-1a").
		WithAttachment("i-1234567890abcdef0", "/dev/sda1").
		Build()
}

// NewSampleSnapshot creates a sample EBS snapshot for testing.
func NewSampleSnapshot() ec2types.Snapshot {
	return NewSnapshotBuilder("snap-0123456789abcdef0").
		WithVolumeID("vol-0123456789abcdef0").
		WithState(ec2types.SnapshotStateCompleted).
		WithProgress("100%").
		WithVolumeSize(100).
		WithForkliftTags("i-1234567890abcdef0", "test-vm", "vol-0123456789abcdef0").
		Build()
}

// NewSampleVpc creates a sample VPC for testing.
func NewSampleVpc() ec2types.Vpc {
	return NewVpcBuilder("vpc-12345").
		WithCidrBlock("10.0.0.0/16").
		WithTag("Name", "test-vpc").
		Build()
}

// NewSampleSubnet creates a sample subnet for testing.
func NewSampleSubnet() ec2types.Subnet {
	return NewSubnetBuilder("subnet-12345", "vpc-12345").
		WithCidrBlock("10.0.1.0/24").
		WithAvailabilityZone("us-east-1a").
		WithTag("Name", "test-subnet").
		Build()
}

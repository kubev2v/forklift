// Package testutil provides test utilities for EC2 provider unit tests.
// It includes fake EC2 API implementations and test fixtures.
package testutil

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	controllerclient "github.com/kubev2v/forklift/pkg/provider/ec2/controller/client"
	inventoryclient "github.com/kubev2v/forklift/pkg/provider/ec2/inventory/client"
)

// EC2Method represents an EC2 API method name for error injection.
type EC2Method string

// EC2 API method constants for type-safe error injection.
const (
	MethodDescribeInstances       EC2Method = "DescribeInstances"
	MethodStopInstances           EC2Method = "StopInstances"
	MethodStartInstances          EC2Method = "StartInstances"
	MethodCreateSnapshot          EC2Method = "CreateSnapshot"
	MethodDeleteSnapshot          EC2Method = "DeleteSnapshot"
	MethodDescribeSnapshots       EC2Method = "DescribeSnapshots"
	MethodModifySnapshotAttribute EC2Method = "ModifySnapshotAttribute"
	MethodDescribeVolumes         EC2Method = "DescribeVolumes"
	MethodCreateVolume            EC2Method = "CreateVolume"
	MethodDeleteVolume            EC2Method = "DeleteVolume"
	MethodDescribeVpcs            EC2Method = "DescribeVpcs"
	MethodDescribeSubnets         EC2Method = "DescribeSubnets"
	MethodDescribeSecurityGroups  EC2Method = "DescribeSecurityGroups"
)

// FakeEC2API is a fake implementation of the EC2 API for testing.
// It implements both controller/client.EC2API and inventory/client.EC2API interfaces.
// It stores state in memory and allows error injection for testing error handling.
type FakeEC2API struct {
	mu sync.Mutex

	// In-memory state
	Instances      map[string]ec2types.Instance
	Volumes        map[string]ec2types.Volume
	Snapshots      map[string]ec2types.Snapshot
	Vpcs           map[string]ec2types.Vpc
	Subnets        map[string]ec2types.Subnet
	SecurityGroups map[string]ec2types.SecurityGroup

	// Snapshot sharing permissions: snapshotID -> []accountID
	SnapshotPermissions map[string][]string

	// Error injection - map of method to error
	// Example: fake.Errors[MethodCreateSnapshot] = errors.New("failed")
	Errors map[EC2Method]error

	// Call tracking for verification
	Calls []APICall

	// ID counter for generating unique IDs
	idCounter int
}

// APICall records a call to the fake API for verification in tests.
type APICall struct {
	Method EC2Method
	Input  interface{}
}

// NewFakeEC2API creates a new FakeEC2API with empty state.
func NewFakeEC2API() *FakeEC2API {
	return &FakeEC2API{
		Instances:           make(map[string]ec2types.Instance),
		Volumes:             make(map[string]ec2types.Volume),
		Snapshots:           make(map[string]ec2types.Snapshot),
		Vpcs:                make(map[string]ec2types.Vpc),
		Subnets:             make(map[string]ec2types.Subnet),
		SecurityGroups:      make(map[string]ec2types.SecurityGroup),
		SnapshotPermissions: make(map[string][]string),
		Errors:              make(map[EC2Method]error),
		Calls:               []APICall{},
	}
}

// Compile-time checks to ensure FakeEC2API implements both interfaces
var _ controllerclient.EC2API = (*FakeEC2API)(nil)
var _ inventoryclient.EC2API = (*FakeEC2API)(nil)

// recordCall records an API call for later verification.
func (f *FakeEC2API) recordCall(method EC2Method, input interface{}) {
	f.Calls = append(f.Calls, APICall{Method: method, Input: input})
}

// getError returns the error for a method call from the Errors map.
func (f *FakeEC2API) getError(method EC2Method) error {
	return f.Errors[method]
}

// Reset clears all state and recorded calls.
func (f *FakeEC2API) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Instances = make(map[string]ec2types.Instance)
	f.Volumes = make(map[string]ec2types.Volume)
	f.Snapshots = make(map[string]ec2types.Snapshot)
	f.Vpcs = make(map[string]ec2types.Vpc)
	f.Subnets = make(map[string]ec2types.Subnet)
	f.SecurityGroups = make(map[string]ec2types.SecurityGroup)
	f.SnapshotPermissions = make(map[string][]string)
	f.Errors = make(map[EC2Method]error)
	f.Calls = []APICall{}
	f.idCounter = 0
}

// AddInstance adds an instance to the fake state.
func (f *FakeEC2API) AddInstance(instance ec2types.Instance) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if instance.InstanceId != nil {
		f.Instances[*instance.InstanceId] = instance
	}
}

// AddVolume adds a volume to the fake state.
func (f *FakeEC2API) AddVolume(volume ec2types.Volume) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if volume.VolumeId != nil {
		f.Volumes[*volume.VolumeId] = volume
	}
}

// AddSnapshot adds a snapshot to the fake state.
func (f *FakeEC2API) AddSnapshot(snapshot ec2types.Snapshot) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if snapshot.SnapshotId != nil {
		f.Snapshots[*snapshot.SnapshotId] = snapshot
	}
}

// AddVpc adds a VPC to the fake state.
func (f *FakeEC2API) AddVpc(vpc ec2types.Vpc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if vpc.VpcId != nil {
		f.Vpcs[*vpc.VpcId] = vpc
	}
}

// AddSubnet adds a subnet to the fake state.
func (f *FakeEC2API) AddSubnet(subnet ec2types.Subnet) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if subnet.SubnetId != nil {
		f.Subnets[*subnet.SubnetId] = subnet
	}
}

// AddSecurityGroup adds a security group to the fake state.
func (f *FakeEC2API) AddSecurityGroup(sg ec2types.SecurityGroup) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if sg.GroupId != nil {
		f.SecurityGroups[*sg.GroupId] = sg
	}
}

// SetInstanceState changes the state of an instance.
func (f *FakeEC2API) SetInstanceState(instanceID string, state ec2types.InstanceStateName) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if instance, ok := f.Instances[instanceID]; ok {
		instance.State = &ec2types.InstanceState{Name: state}
		f.Instances[instanceID] = instance
	}
}

// SetSnapshotState changes the state of a snapshot.
func (f *FakeEC2API) SetSnapshotState(snapshotID string, state ec2types.SnapshotState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if snapshot, ok := f.Snapshots[snapshotID]; ok {
		snapshot.State = state
		f.Snapshots[snapshotID] = snapshot
	}
}

// SetVolumeState changes the state of a volume.
func (f *FakeEC2API) SetVolumeState(volumeID string, state ec2types.VolumeState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if volume, ok := f.Volumes[volumeID]; ok {
		volume.State = state
		f.Volumes[volumeID] = volume
	}
}

// DescribeInstances implements EC2API.
func (f *FakeEC2API) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodDescribeInstances, params)

	if err := f.getError(MethodDescribeInstances); err != nil {
		return nil, err
	}

	var instances []ec2types.Instance

	if params != nil && len(params.InstanceIds) > 0 {
		// Filter by specific IDs
		for _, id := range params.InstanceIds {
			if instance, ok := f.Instances[id]; ok {
				instances = append(instances, instance)
			}
		}
		// If specific IDs were requested but none found, this would be an error in real AWS
		if len(instances) == 0 && len(params.InstanceIds) > 0 {
			return nil, fmt.Errorf("InvalidInstanceID.NotFound: The instance ID '%s' does not exist", params.InstanceIds[0])
		}
	} else {
		// Return all instances
		for _, instance := range f.Instances {
			instances = append(instances, instance)
		}
	}

	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{
			{Instances: instances},
		},
	}, nil
}

// StopInstances implements EC2API.
func (f *FakeEC2API) StopInstances(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodStopInstances, params)

	if err := f.getError(MethodStopInstances); err != nil {
		return nil, err
	}

	var stoppingInstances []ec2types.InstanceStateChange
	for _, id := range params.InstanceIds {
		if instance, ok := f.Instances[id]; ok {
			previousState := instance.State
			instance.State = &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopping}
			f.Instances[id] = instance

			stoppingInstances = append(stoppingInstances, ec2types.InstanceStateChange{
				InstanceId:    aws.String(id),
				CurrentState:  instance.State,
				PreviousState: previousState,
			})
		}
	}

	return &ec2.StopInstancesOutput{
		StoppingInstances: stoppingInstances,
	}, nil
}

// StartInstances implements EC2API.
func (f *FakeEC2API) StartInstances(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodStartInstances, params)

	if err := f.getError(MethodStartInstances); err != nil {
		return nil, err
	}

	var startingInstances []ec2types.InstanceStateChange
	for _, id := range params.InstanceIds {
		if instance, ok := f.Instances[id]; ok {
			previousState := instance.State
			instance.State = &ec2types.InstanceState{Name: ec2types.InstanceStateNamePending}
			f.Instances[id] = instance

			startingInstances = append(startingInstances, ec2types.InstanceStateChange{
				InstanceId:    aws.String(id),
				CurrentState:  instance.State,
				PreviousState: previousState,
			})
		}
	}

	return &ec2.StartInstancesOutput{
		StartingInstances: startingInstances,
	}, nil
}

// CreateSnapshot implements EC2API.
func (f *FakeEC2API) CreateSnapshot(ctx context.Context, params *ec2.CreateSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.CreateSnapshotOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodCreateSnapshot, params)

	if err := f.getError(MethodCreateSnapshot); err != nil {
		return nil, err
	}

	snapshotID := fmt.Sprintf("snap-%s", f.generateID())
	snapshot := ec2types.Snapshot{
		SnapshotId: aws.String(snapshotID),
		VolumeId:   params.VolumeId,
		State:      ec2types.SnapshotStatePending,
		Progress:   aws.String("0%"),
	}

	// Copy tags from input
	if params.TagSpecifications != nil {
		for _, tagSpec := range params.TagSpecifications {
			if tagSpec.ResourceType == ec2types.ResourceTypeSnapshot {
				snapshot.Tags = tagSpec.Tags
			}
		}
	}

	f.Snapshots[snapshotID] = snapshot

	return &ec2.CreateSnapshotOutput{
		SnapshotId: aws.String(snapshotID),
		VolumeId:   params.VolumeId,
		State:      ec2types.SnapshotStatePending,
		Progress:   aws.String("0%"),
		Tags:       snapshot.Tags,
	}, nil
}

// DeleteSnapshot implements EC2API.
func (f *FakeEC2API) DeleteSnapshot(ctx context.Context, params *ec2.DeleteSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodDeleteSnapshot, params)

	if err := f.getError(MethodDeleteSnapshot); err != nil {
		return nil, err
	}

	if params.SnapshotId != nil {
		if _, ok := f.Snapshots[*params.SnapshotId]; !ok {
			return nil, fmt.Errorf("InvalidSnapshot.NotFound: The snapshot '%s' does not exist", *params.SnapshotId)
		}
		delete(f.Snapshots, *params.SnapshotId)
		delete(f.SnapshotPermissions, *params.SnapshotId)
	}

	return &ec2.DeleteSnapshotOutput{}, nil
}

// DescribeSnapshots implements EC2API.
func (f *FakeEC2API) DescribeSnapshots(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodDescribeSnapshots, params)

	if err := f.getError(MethodDescribeSnapshots); err != nil {
		return nil, err
	}

	var snapshots []ec2types.Snapshot

	if params != nil && len(params.SnapshotIds) > 0 {
		// Filter by specific IDs
		for _, id := range params.SnapshotIds {
			if snapshot, ok := f.Snapshots[id]; ok {
				snapshots = append(snapshots, snapshot)
			} else {
				return nil, fmt.Errorf("InvalidSnapshot.NotFound: The snapshot '%s' does not exist", id)
			}
		}
	} else if params != nil && len(params.Filters) > 0 {
		// Filter by tags
		for _, snapshot := range f.Snapshots {
			if matchesFilters(snapshot.Tags, params.Filters) {
				snapshots = append(snapshots, snapshot)
			}
		}
	} else {
		// Return all snapshots
		for _, snapshot := range f.Snapshots {
			snapshots = append(snapshots, snapshot)
		}
	}

	return &ec2.DescribeSnapshotsOutput{
		Snapshots: snapshots,
	}, nil
}

// ModifySnapshotAttribute implements EC2API.
func (f *FakeEC2API) ModifySnapshotAttribute(ctx context.Context, params *ec2.ModifySnapshotAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifySnapshotAttributeOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodModifySnapshotAttribute, params)

	if err := f.getError(MethodModifySnapshotAttribute); err != nil {
		return nil, err
	}

	if params.SnapshotId == nil {
		return nil, fmt.Errorf("SnapshotId is required")
	}

	snapshotID := *params.SnapshotId
	if _, ok := f.Snapshots[snapshotID]; !ok {
		return nil, fmt.Errorf("InvalidSnapshot.NotFound: The snapshot '%s' does not exist", snapshotID)
	}

	// Handle permission modifications
	if params.CreateVolumePermission != nil {
		// Add permissions
		for _, perm := range params.CreateVolumePermission.Add {
			if perm.UserId != nil {
				f.SnapshotPermissions[snapshotID] = append(f.SnapshotPermissions[snapshotID], *perm.UserId)
			}
		}
		// Remove permissions
		for _, perm := range params.CreateVolumePermission.Remove {
			if perm.UserId != nil {
				perms := f.SnapshotPermissions[snapshotID]
				for i, p := range perms {
					if p == *perm.UserId {
						f.SnapshotPermissions[snapshotID] = append(perms[:i], perms[i+1:]...)
						break
					}
				}
			}
		}
	}

	return &ec2.ModifySnapshotAttributeOutput{}, nil
}

// DescribeVolumes implements EC2API.
func (f *FakeEC2API) DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodDescribeVolumes, params)

	if err := f.getError(MethodDescribeVolumes); err != nil {
		return nil, err
	}

	var volumes []ec2types.Volume

	if params != nil && len(params.VolumeIds) > 0 {
		// Filter by specific IDs
		for _, id := range params.VolumeIds {
			if volume, ok := f.Volumes[id]; ok {
				volumes = append(volumes, volume)
			} else {
				return nil, fmt.Errorf("InvalidVolume.NotFound: The volume '%s' does not exist", id)
			}
		}
	} else if params != nil && len(params.Filters) > 0 {
		// Filter by tags
		for _, volume := range f.Volumes {
			if matchesFilters(volume.Tags, params.Filters) {
				volumes = append(volumes, volume)
			}
		}
	} else {
		// Return all volumes
		for _, volume := range f.Volumes {
			volumes = append(volumes, volume)
		}
	}

	return &ec2.DescribeVolumesOutput{
		Volumes: volumes,
	}, nil
}

// CreateVolume implements EC2API.
func (f *FakeEC2API) CreateVolume(ctx context.Context, params *ec2.CreateVolumeInput, optFns ...func(*ec2.Options)) (*ec2.CreateVolumeOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodCreateVolume, params)

	if err := f.getError(MethodCreateVolume); err != nil {
		return nil, err
	}

	volumeID := fmt.Sprintf("vol-%s", f.generateID())
	volume := ec2types.Volume{
		VolumeId:         aws.String(volumeID),
		AvailabilityZone: params.AvailabilityZone,
		VolumeType:       params.VolumeType,
		State:            ec2types.VolumeStateCreating,
	}

	if params.SnapshotId != nil {
		volume.SnapshotId = params.SnapshotId
	}

	// Copy tags from input
	if params.TagSpecifications != nil {
		for _, tagSpec := range params.TagSpecifications {
			if tagSpec.ResourceType == ec2types.ResourceTypeVolume {
				volume.Tags = tagSpec.Tags
			}
		}
	}

	f.Volumes[volumeID] = volume

	return &ec2.CreateVolumeOutput{
		VolumeId:         aws.String(volumeID),
		AvailabilityZone: params.AvailabilityZone,
		VolumeType:       params.VolumeType,
		State:            ec2types.VolumeStateCreating,
		SnapshotId:       params.SnapshotId,
		Tags:             volume.Tags,
	}, nil
}

// DeleteVolume implements EC2API.
func (f *FakeEC2API) DeleteVolume(ctx context.Context, params *ec2.DeleteVolumeInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVolumeOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodDeleteVolume, params)

	if err := f.getError(MethodDeleteVolume); err != nil {
		return nil, err
	}

	if params.VolumeId != nil {
		if _, ok := f.Volumes[*params.VolumeId]; !ok {
			return nil, fmt.Errorf("InvalidVolume.NotFound: The volume '%s' does not exist", *params.VolumeId)
		}
		delete(f.Volumes, *params.VolumeId)
	}

	return &ec2.DeleteVolumeOutput{}, nil
}

// DescribeVpcs implements EC2API.
func (f *FakeEC2API) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodDescribeVpcs, params)

	if err := f.getError(MethodDescribeVpcs); err != nil {
		return nil, err
	}

	var vpcs []ec2types.Vpc
	for _, vpc := range f.Vpcs {
		vpcs = append(vpcs, vpc)
	}

	return &ec2.DescribeVpcsOutput{
		Vpcs: vpcs,
	}, nil
}

// DescribeSubnets implements EC2API.
func (f *FakeEC2API) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodDescribeSubnets, params)

	if err := f.getError(MethodDescribeSubnets); err != nil {
		return nil, err
	}

	var subnets []ec2types.Subnet
	for _, subnet := range f.Subnets {
		subnets = append(subnets, subnet)
	}

	return &ec2.DescribeSubnetsOutput{
		Subnets: subnets,
	}, nil
}

// DescribeSecurityGroups implements EC2API.
func (f *FakeEC2API) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCall(MethodDescribeSecurityGroups, params)

	if err := f.getError(MethodDescribeSecurityGroups); err != nil {
		return nil, err
	}

	var securityGroups []ec2types.SecurityGroup
	for _, sg := range f.SecurityGroups {
		securityGroups = append(securityGroups, sg)
	}

	return &ec2.DescribeSecurityGroupsOutput{
		SecurityGroups: securityGroups,
	}, nil
}

// Helper functions

// generateID generates a unique ID for this fake instance.
// Must be called while holding f.mu lock.
func (f *FakeEC2API) generateID() string {
	f.idCounter++
	return fmt.Sprintf("%08d", f.idCounter)
}

// matchesFilters checks if tags match the provided filters.
// Only tag-based filters are supported: "tag:<key>" and "tag-key".
// Non-tag filters (e.g., "instance-state-name", "volume-id") are treated
// as non-matching and will cause the filter to fail.
func matchesFilters(tags []ec2types.Tag, filters []ec2types.Filter) bool {
	for _, filter := range filters {
		if filter.Name == nil {
			continue
		}

		filterName := *filter.Name
		matched := false

		// Handle tag filters (tag:key-name)
		if len(filterName) > 4 && filterName[:4] == "tag:" {
			tagKey := filterName[4:]
			for _, tag := range tags {
				if tag.Key != nil && *tag.Key == tagKey {
					for _, value := range filter.Values {
						if tag.Value != nil && *tag.Value == value {
							matched = true
							break
						}
					}
				}
				if matched {
					break
				}
			}
		} else if filterName == "tag-key" {
			// Match any tag with the given key
			for _, tag := range tags {
				for _, value := range filter.Values {
					if tag.Key != nil && *tag.Key == value {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
		}

		if !matched {
			return false
		}
	}
	return true
}

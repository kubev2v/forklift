package client

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// PowerOff initiates graceful shutdown of an EC2 instance (Force=false).
// Returns immediately after initiating shutdown; use PoweredOff() to verify completion.
func (r *Client) PowerOff(vmRef ref.Ref) error {
	client, err := r.getSourceClient()
	if err != nil {
		if connErr := r.Connect(); connErr != nil {
			return connErr
		}
		client = r.sourceClient
	}

	log.Info("Stopping EC2 instance", "vm", vmRef.Name, "id", vmRef.ID)

	ctx := context.Background()
	input := &ec2.StopInstancesInput{
		InstanceIds: []string{vmRef.ID},
		Force:       aws.Bool(false),
	}

	_, err = client.StopInstances(ctx, input)
	if err != nil {
		log.Error(err, "Failed to stop EC2 instance", "vm", vmRef.Name, "id", vmRef.ID)
		return liberr.Wrap(err)
	}

	log.Info("EC2 instance stop initiated", "vm", vmRef.Name, "id", vmRef.ID)
	return nil
}

// PowerOn initiates startup of a stopped EC2 instance.
// Returns immediately after initiating start; instance transitions through Pending to Running.
func (r *Client) PowerOn(vmRef ref.Ref) error {
	client, err := r.getSourceClient()
	if err != nil {
		if connErr := r.Connect(); connErr != nil {
			return connErr
		}
		client = r.sourceClient
	}

	log.Info("Starting EC2 instance", "vm", vmRef.Name, "id", vmRef.ID)

	ctx := context.Background()
	input := &ec2.StartInstancesInput{
		InstanceIds: []string{vmRef.ID},
	}

	_, err = client.StartInstances(ctx, input)
	if err != nil {
		log.Error(err, "Failed to start EC2 instance", "vm", vmRef.Name, "id", vmRef.ID)
		return liberr.Wrap(err)
	}

	log.Info("EC2 instance start initiated", "vm", vmRef.Name, "id", vmRef.ID)
	return nil
}

// PowerState queries and maps EC2 instance state to VM power state enum.
// Maps Running/Pending to On, Stopped/Stopping/Terminated to Off, others to Unknown.
func (r *Client) PowerState(vmRef ref.Ref) (planapi.VMPowerState, error) {
	client, err := r.getSourceClient()
	if err != nil {
		if connErr := r.Connect(); connErr != nil {
			return planapi.VMPowerStateUnknown, connErr
		}
		client = r.sourceClient
	}

	ctx := context.Background()
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{vmRef.ID},
	}

	result, err := client.DescribeInstances(ctx, input)
	if err != nil {
		log.Error(err, "Failed to describe EC2 instance", "vm", vmRef.Name, "id", vmRef.ID)
		return planapi.VMPowerStateUnknown, liberr.Wrap(err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return planapi.VMPowerStateUnknown, fmt.Errorf("EC2 instance not found: %s", vmRef.ID)
	}

	instance := result.Reservations[0].Instances[0]
	state := instance.State.Name

	log.V(3).Info("EC2 instance state",
		"vm", vmRef.Name,
		"id", vmRef.ID,
		"state", state)

	switch state {
	case ec2types.InstanceStateNameRunning:
		return planapi.VMPowerStateOn, nil
	case ec2types.InstanceStateNameStopped:
		return planapi.VMPowerStateOff, nil
	case ec2types.InstanceStateNameStopping:
		return planapi.VMPowerStateOff, nil
	case ec2types.InstanceStateNamePending:
		return planapi.VMPowerStateOn, nil
	case ec2types.InstanceStateNameShuttingDown, ec2types.InstanceStateNameTerminated:
		return planapi.VMPowerStateOff, nil
	default:
		return planapi.VMPowerStateUnknown, nil
	}
}

// PoweredOff verifies an instance has fully stopped and is safe for snapshot creation.
// Only returns true for Stopped or Terminated states; false for Running/Pending/Stopping.
// Used during migration to ensure data consistency before snapshotting.
func (r *Client) PoweredOff(vmRef ref.Ref) (bool, error) {
	client, err := r.getSourceClient()
	if err != nil {
		if connErr := r.Connect(); connErr != nil {
			return false, connErr
		}
		client = r.sourceClient
	}

	ctx := context.Background()
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{vmRef.ID},
	}

	result, err := client.DescribeInstances(ctx, input)
	if err != nil {
		log.Error(err, "Failed to describe EC2 instance", "vm", vmRef.Name, "id", vmRef.ID)
		return false, liberr.Wrap(err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return false, fmt.Errorf("EC2 instance not found: %s", vmRef.ID)
	}

	instance := result.Reservations[0].Instances[0]
	state := instance.State.Name

	// Only return true when instance is fully stopped or terminated
	// The PhaseWaitForPowerOff phase will keep waiting until this returns true
	switch state {
	case ec2types.InstanceStateNameStopped:
		// Instance is fully stopped - safe to proceed with snapshots
		return true, nil
	case ec2types.InstanceStateNameTerminated, ec2types.InstanceStateNameShuttingDown:
		// Instance is terminated/shutting-down - consider it "off" to proceed
		// (snapshot creation will fail appropriately if instance doesn't exist)
		return true, nil
	default:
		// Instance is running, pending, stopping, or unknown - keep waiting
		return false, nil
	}
}

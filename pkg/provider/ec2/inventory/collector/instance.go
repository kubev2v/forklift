package collector

import (
	"context"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// collectInstances collects EC2 instances
func (r *Collector) collectInstances(ctx context.Context) error {
	instances, err := r.client.DescribeInstances(ctx)
	if err != nil {
		return err
	}

	r.log.V(1).Info("Collected instances", "count", len(instances))

	for _, awsInstance := range instances {
		m := &model.Instance{}

		// Set minimal indexed fields
		if awsInstance.InstanceId != nil {
			m.UID = *awsInstance.InstanceId
		} else {
			continue // Skip instances without ID
		}

		m.Name = getNameFromTags(awsInstance.Tags)
		if m.Name == "" {
			m.Name = m.UID // Use instance ID as name if no Name tag
		}

		m.Kind = "Instance"
		m.Provider = string(r.provider.UID)

		// Set EC2-specific indexed fields
		m.InstanceType = string(awsInstance.InstanceType)
		if awsInstance.State != nil {
			m.State = string(awsInstance.State.Name)
		}
		m.Platform = getPlatform(awsInstance.Platform, awsInstance.PlatformDetails)

		// Store complete AWS instance as JSON
		if err := m.SetObject(awsInstance); err != nil {
			r.log.Error(err, "Failed to marshal instance", "instanceId", m.UID)
			continue
		}

		// Increment revision
		existing := &model.Instance{}
		existing.UID = m.UID
		if err := r.db.Get(existing); err == nil {
			m.Revision = existing.Revision + 1
		} else {
			m.Revision = 1
		}

		// Insert or update in database
		if err := r.db.Insert(m); err != nil {
			r.log.Error(err, "Failed to insert instance", "instanceId", m.UID)
			continue
		}
	}

	return nil
}

// getNameFromTags extracts Name tag from AWS tags
func getNameFromTags(tags []ec2types.Tag) string {
	for _, tag := range tags {
		if tag.Key != nil && *tag.Key == "Name" && tag.Value != nil {
			return *tag.Value
		}
	}
	return ""
}

// getPlatform determines platform (linux or windows)
func getPlatform(platform interface{}, platformDetails *string) string {
	// Check platform field first
	if platform != nil {
		if p, ok := platform.(string); ok && p == "windows" {
			return "windows"
		}
	}

	// Check platformDetails for more info
	if platformDetails != nil {
		details := *platformDetails
		if strings.Contains(details, "Windows") {
			return "windows"
		}
	}

	return "linux" // Default to linux
}

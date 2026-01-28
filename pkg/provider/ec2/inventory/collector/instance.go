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

	var created, updated, unchanged int
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

		// Store complete AWS instance object
		m.Object = awsInstance

		// Check if record exists and has changed
		existing := &model.Instance{}
		existing.UID = m.UID
		if err := r.db.Get(existing); err == nil {
			// Record exists - check if it changed
			if !existing.HasChanged(m) {
				unchanged++
				continue // No change, skip DB write
			}
			// Changed - update with incremented revision
			m.Revision = existing.Revision + 1
			if err := r.db.Update(m); err != nil {
				r.log.Error(err, "Failed to update instance", "instanceId", m.UID)
				continue
			}
			updated++
		} else {
			// New record - insert
			m.Revision = 1
			if err := r.db.Insert(m); err != nil {
				r.log.Error(err, "Failed to insert instance", "instanceId", m.UID)
				continue
			}
			created++
		}
	}

	r.log.V(1).Info("Instances processed", "created", created, "updated", updated, "unchanged", unchanged)
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

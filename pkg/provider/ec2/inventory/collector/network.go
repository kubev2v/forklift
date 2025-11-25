package collector

import (
	"context"

	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// collectNetworks collects VPCs and Subnets
func (r *Collector) collectNetworks(ctx context.Context) error {
	// Collect VPCs
	vpcs, err := r.client.DescribeVpcs(ctx)
	if err != nil {
		return err
	}

	r.log.V(1).Info("Collected VPCs", "count", len(vpcs))

	for _, awsVpc := range vpcs {
		m := &model.Network{}

		if awsVpc.VpcId != nil {
			m.UID = *awsVpc.VpcId
		} else {
			continue
		}

		m.Name = getNameFromTags(awsVpc.Tags)
		if m.Name == "" {
			m.Name = m.UID
		}

		m.Kind = "Network"
		m.Provider = string(r.provider.UID)
		m.NetworkType = "vpc"
		if awsVpc.CidrBlock != nil {
			m.CIDR = *awsVpc.CidrBlock
		}

		if err := m.SetObject(awsVpc); err != nil {
			r.log.Error(err, "Failed to marshal VPC", "vpcId", m.UID)
			continue
		}

		existing := &model.Network{}
		existing.UID = m.UID
		if err := r.db.Get(existing); err == nil {
			m.Revision = existing.Revision + 1
		} else {
			m.Revision = 1
		}

		if err := r.db.Insert(m); err != nil {
			r.log.Error(err, "Failed to insert VPC", "vpcId", m.UID)
			continue
		}
	}

	// Collect Subnets
	subnets, err := r.client.DescribeSubnets(ctx)
	if err != nil {
		return err
	}

	r.log.V(1).Info("Collected Subnets", "count", len(subnets))

	for _, awsSubnet := range subnets {
		m := &model.Network{}

		if awsSubnet.SubnetId != nil {
			m.UID = *awsSubnet.SubnetId
		} else {
			continue
		}

		m.Name = getNameFromTags(awsSubnet.Tags)
		if m.Name == "" {
			m.Name = m.UID
		}

		m.Kind = "Network"
		m.Provider = string(r.provider.UID)
		m.NetworkType = "subnet"
		if awsSubnet.CidrBlock != nil {
			m.CIDR = *awsSubnet.CidrBlock
		}

		if err := m.SetObject(awsSubnet); err != nil {
			r.log.Error(err, "Failed to marshal Subnet", "subnetId", m.UID)
			continue
		}

		existing := &model.Network{}
		existing.UID = m.UID
		if err := r.db.Get(existing); err == nil {
			m.Revision = existing.Revision + 1
		} else {
			m.Revision = 1
		}

		if err := r.db.Insert(m); err != nil {
			r.log.Error(err, "Failed to insert Subnet", "subnetId", m.UID)
			continue
		}
	}

	return nil
}

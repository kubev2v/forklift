# Provider Compatibility Reference

| Metadata | Value |
|----------|-------|
| **Last Updated** | January 27, 2026 |
| **Applies To** | Forklift v2.11 |
| **Maintainer** | Forklift Team |
| **Update Policy** | Update when provider support changes or new features are added |

This documentation provides comprehensive compatibility information for all Forklift provider types and features.

## Supported Providers

Forklift supports migration from the following source platforms:

| Provider | Type Constant | Description |
|----------|---------------|-------------|
| **VMware vSphere** | `vsphere` | VMware vCenter and ESXi hosts |
| **Red Hat Virtualization (oVirt)** | `ovirt` | oVirt/RHV management servers |
| **OpenStack** | `openstack` | OpenStack cloud platforms |
| **OpenShift Virtualization** | `openshift` | KubeVirt VMs on OpenShift clusters |
| **OVA** | `ova` | Open Virtual Appliance files (VMware exports) |
| **Amazon EC2** | `ec2` | AWS EC2 instances with EBS volumes |
| **Hyper-V** | `hyperv` | Microsoft Hyper-V servers |

## Documentation Index

### Provider Configuration

- [Provider Secrets](./provider-secrets.md) - Authentication credentials and secret fields for each provider type
- [Provider Settings](./provider-settings.md) - Provider-specific configuration in `spec.settings`

### Migration Planning

- [Plan Fields](./plan-fields.md) - Migration plan specification fields and provider support
- [VM Fields](./vm-fields.md) - Per-VM configuration options within a plan
- [Migration Features](./migration-features.md) - Migration types, guest conversion, and storage features

### Provider-Specific Guides

- [EC2 Bill of Materials](./ec2-bill-of-materials.md) - AWS API calls, resources, and cost estimation for EC2 migrations

### Operator Configuration

- [ForkliftController Settings](./forkliftcontroller-settings.md) - Operator CR settings and feature gates

### Inventory

- [Inventory Resources](./inventory-resources.md) - Available inventory resource types by provider

## Quick Reference: Feature Support Matrix

### Migration Types

| Migration Type | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|----------------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| Cold | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Warm | Yes | Yes* | No | No | No | No | No |
| Live | No | No | No | Yes* | No | No | No |
| Conversion-only | Yes | No | No | No | No | No | No |

*oVirt warm migration requires feature gate `FEATURE_OVIRT_WARM_MIGRATION`<br>
*OpenShift live migration requires feature gate `FEATURE_OCP_LIVE_MIGRATION` and KubeVirt `DecentralizedLiveMigration` feature on both clusters

### Guest Conversion

| Feature | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|---------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| Requires virt-v2v | Yes* | No | No | No | Yes | Yes* | Yes |
| Driver injection | Yes | No | No | No | Yes | Yes | Yes |
| Windows support | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Linux support | Yes | Yes | Yes | Yes | Yes | Yes | Yes |

*vSphere and EC2 support `skipGuestConversion` to bypass virt-v2v; use `useCompatibilityMode` for SATA/E1000E devices or ensure VirtIO drivers are pre-installed. OVA and HyperV always require virt-v2v.

### Key Features

| Feature | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|---------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| Static IP preservation | Yes | No | No | No | No | No | No |
| Shared disk migration | Yes | Yes | No | No | No | No | No |
| LUKS encryption | Yes | Yes | No | No | No | No | No |
| Migration hooks | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Naming templates | Yes | No | No | Partial | No | No | No |
| Storage offload (XCOPY) | Yes | No | No | No | No | No | No |

## Related Documentation

- [Template Support Matrix](../template-support-matrix.md) - Detailed PVC/volume/network naming templates
- [Migration Hooks](../hooks.md) - Pre and post-migration hook configuration
- [Debugging Guide](../debugging.md) - Troubleshooting migrations

## External Resources

- [Forklift Documentation (Red Hat)](https://docs.redhat.com/en/documentation/migration_toolkit_for_virtualization/)
- [KubeVirt User Guide](https://kubevirt.io/user-guide/)
- [VMware VDDK](https://developer.vmware.com/web/sdk/7.0/vddk) (vSphere migrations)
- [oVirt Documentation](https://www.ovirt.org/documentation/)
- [OpenStack Documentation](https://docs.openstack.org/)
- [AWS EBS CSI Driver](https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html) (EC2 migrations)

#!/usr/bin/env python3
"""
ForkliftController CRD validation script.

Ensures that ForkliftController CRD spec properties correspond to variables
used in the operator tasks and templates in operator/roles/forkliftcontroller/

This validation maintains synchronization between the CRD schema and 
the variables actually used by the operator.
"""

import os
import sys
import yaml
import argparse
import re
from pathlib import Path


def load_crd_properties(crd_file):
    """Extract all property names from the ForkliftController CRD spec."""
    try:
        with open(crd_file, 'r') as f:
            crd_data = yaml.safe_load(f)
    except Exception as e:
        print(f"Error loading CRD file {crd_file}: {e}")
        sys.exit(1)
    
    # Navigate to the spec properties
    try:
        properties = crd_data['spec']['versions'][0]['schema']['openAPIV3Schema']['properties']['spec']['properties']
        return set(properties.keys())
    except KeyError as e:
        print(f"Error navigating CRD structure: {e}")
        sys.exit(1)


def load_tasks_variables(tasks_file):
    """Extract variable references from the Ansible tasks file."""
    try:
        with open(tasks_file, 'r') as f:
            content = f.read()
    except Exception as e:
        print(f"Error loading tasks file {tasks_file}: {e}")
        sys.exit(1)
    
    variables = set()
    
    # Pattern 1: Variables with filters like "feature_ui_plugin|bool"
    filter_pattern = r'([a-zA-Z_][a-zA-Z0-9_]*)\|'
    for match in re.finditer(filter_pattern, content):
        variables.add(match.group(1))
    
    # Pattern 2: Simple variable references in when conditions 
    when_pattern = r'when:\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:\||$)'
    for match in re.finditer(when_pattern, content, re.MULTILINE):
        variables.add(match.group(1))
        
    # Pattern 3: Variables in set_fact tasks
    fact_pattern = r'set_fact:\s*\n\s*([a-zA-Z_][a-zA-Z0-9_]+):'
    for match in re.finditer(fact_pattern, content, re.MULTILINE):
        variables.add(match.group(1))
    
    # Additionally, we need to load the defaults file to get variables used in templates
    # Since tasks reference templates which use the actual CRD variables
    defaults_file = tasks_file.parent.parent / 'defaults' / 'main.yml'
    if defaults_file.exists():
        with open(defaults_file, 'r') as f:
            defaults_data = yaml.safe_load(f)
        
        # Add variables from defaults that should correspond to CRD properties
        defaults_variables = set(defaults_data.keys())
        
        # Note: excluded_fields are now applied globally at the end of this function
        # to filter out calculated/derived variables that shouldn't be in CRD
        variables.update(defaults_variables)
    
    # Filter out Ansible built-in variables, internal variables, and false positives
    ansible_builtin = {
        'api_groups', 'cluster_version', 'ocp_version', 'k8s_cluster_version',
        'console_operator', 'console_plugins', 'finalize', 'webhook_state',
        'validation_state', 'ui_plugin_state', 'volume_populator_state',
        'resource_kind', 'feature_label', 'loop_var',
        # Jinja expression components (false positives)
        'first', 'last', 'not', 'in', 'and', 'or', 'is', 'defined', 'undefined', 'false', 'true',
        'gitVersion', 'kubernetes', 'default', 'selectattr', 'equalto', 'map',
        'attribute', 'version', 'state', 'Completed', 'history', 'status',
        'resources', 'bool', 'lookup', 'template', 'present', 'absent'
    }
    
    # Also filter out the calculated/derived variables that shouldn't be in CRD
    # (apply the same exclusions that were applied to defaults)
    excluded_fields = {
        'app_name',
        'app_namespace', 
        'forklift_operator_version',
        'forklift_resources',
        # Service and component names
        'controller_configmap_name',
        'controller_service_name',
        'controller_deployment_name',
        'controller_container_name',
        'ovirt_osmap_configmap_name',
        'vsphere_osmap_configmap_name', 
        'virt_customize_configmap_name',
        'profiler_volume_path',
        # Inventory service fields
        'inventory_volume_path',
        'inventory_container_name',
        'inventory_service_name',
        'inventory_route_name',
        'inventory_route_timeout',
        'inventory_tls_secret_name',
        'inventory_issuer_name',
        'inventory_certificate_name',
        # Services configuration
        'services_service_name',
        'services_route_name',
        'services_tls_secret_name', 
        'services_issuer_name',
        'services_certificate_name',
        # Validation service configuration
        'validation_configmap_name',
        'validation_service_name',
        'validation_deployment_name',
        'validation_container_name',
        'validation_extra_volume_name',
        'validation_extra_volume_mountpath',
        'validation_tls_secret_name',
        'validation_issuer_name',
        'validation_certificate_name',
        'validation_state',
        # UI Plugin configuration
        'ui_plugin_console_name',
        'ui_plugin_display_name',
        'ui_plugin_service_name',
        'ui_plugin_deployment_name',
        'ui_plugin_container_name',
        'ui_plugin_state',
        # API service configuration
        'api_service_name',
        'api_deployment_name',
        'api_container_name',
        'api_tls_secret_name',
        'api_issuer_name', 
        'api_certificate_name',
        # CLI Download service configuration (calculated/derived values)
        'cli_download_container_name',
        'cli_download_deployment_name', 
        'cli_download_route_name',
        'cli_download_service_name',
        'cli_download_state',
        # Populator configuration
        'populator_controller_deployment_name',
        'populator_controller_container_name',
        # VDDK configuration
        'vddk_build_config_name',
        'vddk_image_stream_name',
        # Metrics configuration
        'metric_service_name',
        'metric_servicemonitor_name',
        'metric_interval',
        'metric_port_name',
        'metrics_rule_name',
        # OVA Proxy configuration
        'ova_proxy_container_name',
        'ova_proxy_certificate_name',
        'ova_proxy_issuer_name',
        'ova_proxy_route_name',
        'ova_proxy_route_timeout',
        'ova_proxy_service_name',
        'ova_proxy_subapp_name',
        'ova_proxy_tls_secret_name',
        # Additional internal variables that should not be CRD properties  
        'app_namespace',
        'forklift_resources',
        'ui_plugin_console_name',
        'ui_plugin_service_name', 
        'validation_service_name',
        # Transfer network temporary/internal variables (not user-facing)
        'transfer_network_namespace',
        'transfer_network_name',
        'transfer_network_default_route',
        'transfer_nad',
        'controller_transfer_network_is_json_string',
        'controller_transfer_network_parsed',
    }
    
    return variables - ansible_builtin - excluded_fields


def validate_forklift_controller_crd(crd_file, tasks_file):
    """Validate that ForkliftController CRD properties correspond to variables used in tasks."""
    print(f"Validating ForkliftController CRD against operator tasks:")
    print(f"  CRD: {crd_file}")
    print(f"  Tasks: {tasks_file}")
    print()
    
    crd_properties = load_crd_properties(crd_file)
    tasks_variables = load_tasks_variables(tasks_file)
    
    # Properties that are allowed to be in CRD even if not directly used in tasks
    # These are valid CRD fields that may be used indirectly or are configuration values
    allowed_crd_only_properties = {
        'controller_transfer_network',
        'inventory_route_timeout',
        'metric_interval',
        'ova_proxy_route_timeout',
        'ovirt_osmap_configmap_name',
        'validation_extra_volume_mountpath',
        'validation_extra_volume_name',
        'virt_customize_configmap_name',
        'vsphere_osmap_configmap_name',
    }
    
    print(f"Found {len(crd_properties)} properties in ForkliftController CRD spec")
    print(f"Found {len(tasks_variables)} relevant variables in tasks and templates")
    print()
    
    # Check for differences, excluding allowed CRD-only properties
    missing_in_tasks = crd_properties - tasks_variables - allowed_crd_only_properties
    missing_in_crd = tasks_variables - crd_properties
    
    success = True
    
    if missing_in_tasks:
        success = False
        print("❌ ForkliftController CRD properties not used in operator tasks:")
        for prop in sorted(missing_in_tasks):
            print(f"  - {prop}")
        print()
    
    if missing_in_crd:
        success = False
        print("❌ Variables used in tasks but missing from ForkliftController CRD:")
        for field in sorted(missing_in_crd):
            print(f"  - {field}")
        print()
    
    if success:
        print("✅ ForkliftController CRD validation passed!")
        print(f"   {len(crd_properties)} properties match between CRD and operator tasks")
        return True
    else:
        print("❌ ForkliftController CRD validation failed!")
        print()
        print("To fix this:")
        print("1. Add missing properties to the ForkliftController CRD spec in:")
        print(f"   {crd_file}")
        print("2. Add missing variables to operator tasks/templates in:")
        print(f"   {tasks_file.parent}")
        print("3. Remove extra fields that don't belong in either place")
        return False


def main():
    parser = argparse.ArgumentParser(description='Validate ForkliftController CRD against operator tasks')
    parser.add_argument('--crd-file', 
                       default='operator/config/crd/bases/forklift.konveyor.io_forkliftcontrollers.yaml',
                       help='Path to ForkliftController CRD file')
    parser.add_argument('--tasks-file',
                       default='operator/roles/forkliftcontroller/tasks/main.yml', 
                       help='Path to operator tasks file')
    
    args = parser.parse_args()
    
    # Convert to absolute paths
    root_dir = Path(__file__).parent.parent
    crd_file = root_dir / args.crd_file
    tasks_file = root_dir / args.tasks_file
    
    # Check files exist
    if not crd_file.exists():
        print(f"❌ CRD file not found: {crd_file}")
        sys.exit(1)
        
    if not tasks_file.exists():
        print(f"❌ Tasks file not found: {tasks_file}")
        sys.exit(1)
    
    # Perform validation
    success = validate_forklift_controller_crd(crd_file, tasks_file)
    
    sys.exit(0 if success else 1)


if __name__ == '__main__':
    main()

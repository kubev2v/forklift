package ec2

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

const (
	// awsCredsSecretNamespace is the namespace where the AWS credentials secret is stored
	awsCredsSecretNamespace = "kube-system"
	// awsCredsSecretName is the name of the AWS credentials secret
	awsCredsSecretName = "aws-creds"
	// workerNodeLabel is the label used to identify worker nodes
	workerNodeLabel = "node-role.kubernetes.io/worker"
	// topologyZoneLabel is the label used to identify the availability zone
	topologyZoneLabel = "topology.kubernetes.io/zone"
)

// AWSClusterCredentials holds the AWS credentials fetched from the cluster
type AWSClusterCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

// FetchAWSCredentialsFromCluster fetches AWS credentials from the kube-system/aws-creds secret
// This secret is typically created by the OpenShift installer on AWS clusters
func FetchAWSCredentialsFromCluster(configFlags *genericclioptions.ConfigFlags) (*AWSClusterCredentials, error) {
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	// Fetch the aws-creds secret from kube-system namespace
	secret, err := k8sClient.CoreV1().Secrets(awsCredsSecretNamespace).Get(
		context.Background(),
		awsCredsSecretName,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get AWS credentials secret '%s/%s': %v. "+
			"This secret is typically created by the OpenShift installer on AWS clusters",
			awsCredsSecretNamespace, awsCredsSecretName, err)
	}

	// Extract credentials from the secret
	accessKeyID, ok := secret.Data["aws_access_key_id"]
	if !ok {
		return nil, fmt.Errorf("aws_access_key_id not found in secret '%s/%s'",
			awsCredsSecretNamespace, awsCredsSecretName)
	}

	secretAccessKey, ok := secret.Data["aws_secret_access_key"]
	if !ok {
		return nil, fmt.Errorf("aws_secret_access_key not found in secret '%s/%s'",
			awsCredsSecretNamespace, awsCredsSecretName)
	}

	klog.V(2).Infof("Successfully fetched AWS credentials from cluster secret '%s/%s'",
		awsCredsSecretNamespace, awsCredsSecretName)

	return &AWSClusterCredentials{
		AccessKeyID:     string(accessKeyID),
		SecretAccessKey: string(secretAccessKey),
	}, nil
}

// FetchTargetAZFromCluster detects the target availability zone from worker node labels
// It returns the first AZ found on a worker node
func FetchTargetAZFromCluster(configFlags *genericclioptions.ConfigFlags) (string, error) {
	k8sClient, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return "", fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	// List worker nodes
	nodes, err := k8sClient.CoreV1().Nodes().List(
		context.Background(),
		metav1.ListOptions{
			LabelSelector: workerNodeLabel,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to list worker nodes: %v", err)
	}

	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("no worker nodes found with label '%s'", workerNodeLabel)
	}

	// Find the first node with a topology zone label
	for _, node := range nodes.Items {
		if zone, ok := node.Labels[topologyZoneLabel]; ok && zone != "" {
			klog.V(2).Infof("Detected target availability zone '%s' from worker node '%s'",
				zone, node.Name)
			return zone, nil
		}
	}

	return "", fmt.Errorf("no worker node found with topology zone label '%s'", topologyZoneLabel)
}

// AutoPopulateTargetOptions auto-fetches EC2 target credentials, availability zone, and region
// from the cluster when any of the target values are empty. It modifies the provided pointers
// to populate the auto-detected values.
func AutoPopulateTargetOptions(configFlags *genericclioptions.ConfigFlags, targetAccessKeyID, targetSecretKey, targetAZ, targetRegion *string) error {
	// Fetch AWS credentials from cluster secret (kube-system/aws-creds) if not provided
	if *targetAccessKeyID == "" || *targetSecretKey == "" {
		clusterCreds, err := FetchAWSCredentialsFromCluster(configFlags)
		if err != nil {
			return fmt.Errorf("failed to auto-fetch target credentials: %v", err)
		}
		if *targetAccessKeyID == "" {
			*targetAccessKeyID = clusterCreds.AccessKeyID
			fmt.Printf("Auto-detected target access key ID from cluster secret\n")
		}
		if *targetSecretKey == "" {
			*targetSecretKey = clusterCreds.SecretAccessKey
			fmt.Printf("Auto-detected target secret access key from cluster secret\n")
		}
	}

	// Auto-detect target-az from worker nodes if not provided
	if *targetAZ == "" {
		detectedAZ, err := FetchTargetAZFromCluster(configFlags)
		if err != nil {
			return fmt.Errorf("failed to auto-detect target availability zone: %v", err)
		}
		*targetAZ = detectedAZ
		fmt.Printf("Auto-detected target availability zone: %s\n", detectedAZ)

		// Also set target region from target-az if not provided
		if *targetRegion == "" && len(detectedAZ) > 1 {
			// Extract region from AZ (e.g., "us-east-1a" -> "us-east-1")
			*targetRegion = detectedAZ[:len(detectedAZ)-1]
			fmt.Printf("Auto-detected target region: %s\n", *targetRegion)
		}
	}

	return nil
}

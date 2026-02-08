package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/yaml"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan/status"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// Start starts a migration plan or outputs the Migration CR if dry-run is enabled
func Start(configFlags *genericclioptions.ConfigFlags, name, namespace string, cutoverTime *time.Time, useUTC bool, dryRun bool, outputFormat string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the plan
	plan, err := c.Resource(client.PlansGVR).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get plan: %v", err)
	}

	// Check if the plan is ready
	planReady, err := status.IsPlanReady(plan)
	if err != nil {
		return err
	}
	if !planReady {
		return fmt.Errorf("migration plan '%s' is not ready", name)
	}

	// Check if the plan has running migrations
	runningMigration, _, err := status.GetRunningMigration(c, namespace, plan, client.MigrationsGVR)
	if err != nil {
		return err
	}
	if runningMigration != nil {
		return fmt.Errorf("migration plan '%s' already has a running migration", name)
	}

	// Check if the plan has already succeeded
	planStatus, err := status.GetPlanStatus(plan)
	if err != nil {
		return err
	}
	if planStatus == status.StatusSucceeded {
		return fmt.Errorf("migration plan '%s' has already succeeded", name)
	}

	// Check if the plan is a warm migration
	warm, _, err := unstructured.NestedBool(plan.Object, "spec", "warm")
	if err != nil {
		return fmt.Errorf("failed to check if plan is warm: %v", err)
	}

	// Handle cutover time based on plan type
	if !warm && cutoverTime != nil {
		fmt.Fprintf(os.Stderr, "Warning: Cutover time is specified but plan '%s' is not a warm migration. Ignoring cutover time.\n", name)
		cutoverTime = nil
	} else if warm && cutoverTime == nil {
		// For warm migrations without specified cutover, default to now + 1 hour
		defaultTime := time.Now().Add(1 * time.Hour)
		cutoverTime = &defaultTime
		fmt.Fprintf(os.Stderr, "Warning: No cutover time specified for warm migration. Setting default cutover time to %s (1 hour from now).\n", output.FormatTimestamp(*cutoverTime, useUTC))
	}

	// Extract the plan's UID
	planUID := string(plan.GetUID())

	// Create a migration object using structured type
	migration := &forkliftv1beta1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", name),
			Namespace:    namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: forkliftv1beta1.SchemeGroupVersion.String(),
					Kind:       "Plan",
					Name:       name,
					UID:        types.UID(planUID),
				},
			},
		},
		Spec: forkliftv1beta1.MigrationSpec{
			Plan: corev1.ObjectReference{
				Name:      name,
				Namespace: namespace,
				UID:       types.UID(planUID),
			},
		},
	}
	migration.Kind = "Migration"
	migration.APIVersion = forkliftv1beta1.SchemeGroupVersion.String()

	// Set cutover time if applicable (for warm migrations)
	if warm && cutoverTime != nil {
		// Convert time.Time to *metav1.Time
		metaTime := metav1.NewTime(*cutoverTime)
		migration.Spec.Cutover = &metaTime
	}

	// Handle dry-run mode
	if dryRun {
		return OutputMigrationCR(migration, outputFormat)
	}

	// Convert Migration object to Unstructured
	unstructuredMigration, err := runtime.DefaultUnstructuredConverter.ToUnstructured(migration)
	if err != nil {
		return fmt.Errorf("failed to convert Migration to Unstructured: %v", err)
	}
	migrationUnstructured := &unstructured.Unstructured{Object: unstructuredMigration}

	// Create the migration in the specified namespace
	_, err = c.Resource(client.MigrationsGVR).Namespace(namespace).Create(context.TODO(), migrationUnstructured, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create migration: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Migration started for plan '%s' in namespace '%s'\n", name, namespace)
	if warm && cutoverTime != nil {
		fmt.Fprintf(os.Stderr, "Cutover scheduled for: %s\n", output.FormatTimestamp(*cutoverTime, useUTC))
	}
	return nil
}

// OutputMigrationCR serializes a Migration CR to the specified format and outputs it to stdout
func OutputMigrationCR(migration *forkliftv1beta1.Migration, outputFormat string) error {
	if migration == nil {
		return fmt.Errorf("migration CR is nil")
	}

	var output []byte
	var err error

	switch outputFormat {
	case "yaml":
		output, err = yaml.Marshal(migration)
		if err != nil {
			return fmt.Errorf("failed to marshal Migration to YAML: %v", err)
		}
		// Always prefix YAML with document separator for proper multi-document format
		fmt.Print("---\n")
		fmt.Print(string(output))
		return nil
	case "json":
		output, err = json.MarshalIndent(migration, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal Migration to JSON: %v", err)
		}
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	fmt.Print(string(output))
	return nil
}

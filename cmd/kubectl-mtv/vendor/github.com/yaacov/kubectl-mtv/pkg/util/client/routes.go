package client

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// CanAccessRoutesInNamespace checks if the user has permissions to list routes in the given namespace
func CanAccessRoutesInNamespace(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string) bool {
	return CanAccessResource(ctx, configFlags, namespace, RouteGVR, "list")
}

// GetForkliftInventoryRoute attempts to find a route with the forklift inventory service labels
func GetForkliftInventoryRoute(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string) (*unstructured.Unstructured, error) {
	// Get dynamic client
	c, err := GetDynamicClient(configFlags)
	if err != nil {
		return nil, err
	}

	// Create label selector for forklift inventory route
	labelSelector := "app=forklift,service=forklift-inventory"

	// Try to discover the MTV operator namespace from CRD annotations
	mtvNamespace := GetMTVOperatorNamespace(ctx, configFlags)

	// Check if we have access to the discovered MTV namespace
	if CanAccessRoutesInNamespace(ctx, configFlags, mtvNamespace) {
		// Try to find the route in the MTV operator namespace
		routes, err := c.Resource(RouteGVR).Namespace(mtvNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})

		// If we find a route in MTV operator namespace, return it
		if err == nil && len(routes.Items) > 0 {
			return &routes.Items[0], nil
		}
	}

	// If we couldn't find the route in MTV operator namespace or didn't have permissions,
	// try the provided namespace
	routes, err := c.Resource(RouteGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	// Return the first matching route if found
	if len(routes.Items) > 0 {
		return &routes.Items[0], nil
	}

	return nil, fmt.Errorf("no matching route found")
}

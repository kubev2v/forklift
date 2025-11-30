package controller

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GetAWSObject extracts nested AWS API object from provider inventory resource.
// Inventory stores AWS resources as unstructured with actual API response nested under "object" key.
// Used by builder, validator, migrator to access AWS resource data for migration.
func GetAWSObject(resource *unstructured.Unstructured) (map[string]interface{}, error) {
	awsObj, found, _ := unstructured.NestedMap(resource.Object, "object")
	if !found || awsObj == nil {
		return nil, fmt.Errorf("no AWS object found in inventory data")
	}
	return awsObj, nil
}

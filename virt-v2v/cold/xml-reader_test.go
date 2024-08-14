package main

import (
	"testing"
)

const expecetErrorTemplate = "Expected no error, got %v"
const noErrorTemplate = "Expected error, got nil"

// TestReadXMLFile tests the ReadXMLFile function.
func TestReadXMLFile(t *testing.T) {
	// Test Case 1: Valid XML file
	t.Run("Valid XML file", func(t *testing.T) {
		filePath := "testdata/valid_config.xml"

		_, err := ReadXMLFile(filePath)
		if err != nil {
			t.Fatalf(expecetErrorTemplate, err)
		}
	})

	// Test Case 2: Empty file path
	t.Run("Empty file path", func(t *testing.T) {
		_, err := ReadXMLFile("")
		if err == nil {
			t.Fatalf(noErrorTemplate)
		}
	})

	// Test Case 3: Non-existent file
	t.Run("Non-existent file", func(t *testing.T) {
		_, err := ReadXMLFile("non_existent_file.xml")
		if err == nil {
			t.Fatalf(noErrorTemplate)
		}
	})
}

// TestGetOperationSystemFromConfig tests the GetOperationSystemFromConfig function.
func TestGetOperationSystemFromConfig(t *testing.T) {
	// Test Case 1: Valid XML data
	t.Run("Valid XML data", func(t *testing.T) {
		filePath := "testdata/valid_config.xml"

		xmlData, err := ReadXMLFile(filePath)
		if err != nil {
			t.Fatalf(expecetErrorTemplate, err)
		}

		expectedOS := "http://redhat.com/rhel/8.2"

		osID, err := GetOperationSystemFromConfig(xmlData)
		if err != nil {
			t.Fatalf(expecetErrorTemplate, err)
		}
		if osID != expectedOS {
			t.Fatalf("Expected %v, got %v", expectedOS, osID)
		}
	})

	// Test Case 2: Invalid XML data
	t.Run("Invalid XML data", func(t *testing.T) {
		xmlData := []byte(`<domain><metadata><libosinfo><libosinfo:os></libosinfo></metadata></domain>`)

		_, err := GetOperationSystemFromConfig(xmlData)
		if err == nil {
			t.Fatalf(noErrorTemplate)
		}
	})

	// Test Case 3: Missing OS ID in XML
	t.Run("Missing OS ID", func(t *testing.T) {
		xmlData := []byte(`<domain><metadata><libosinfo><libosinfo:os /></libosinfo></metadata></domain>`)

		_, err := GetOperationSystemFromConfig(xmlData)
		if err == nil {
			t.Fatalf(noErrorTemplate)
		}
	})
}

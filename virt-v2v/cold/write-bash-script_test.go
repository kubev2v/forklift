package main

import (
	"os"
	"testing"
)

func TestWriteBashScript(t *testing.T) {
	// Define a sample bash script
	script := `#!/bin/bash
echo "Hello, World!"
`

	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "testscript-*.sh")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tmpfile.Name()) // Clean up

	// Close the file so WriteBashScript can write to it
	tmpfile.Close()

	// Call WriteBashScript to write the script to the temporary file
	err = WriteBashScript(script, tmpfile.Name())
	if err != nil {
		t.Fatalf("WriteBashScript failed: %v", err)
	}

	// Read the content of the file
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to read the file: %v", err)
	}

	// Check if the content matches the script
	if string(content) != script {
		t.Errorf("File content does not match the script.\nExpected:\n%s\nGot:\n%s", script, string(content))
	}
}

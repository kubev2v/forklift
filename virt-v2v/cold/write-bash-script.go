package main

import (
	"os"
)

// writeBashScript writes the given bash script to the specified file path.
func WriteBashScript(script, path string) error {
	// Write the script to the specified file path
	err := os.WriteFile(path, []byte(script), 0755)
	if err != nil {
		return err
	}
	return nil
}

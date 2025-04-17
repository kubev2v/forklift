package utils

import (
	"fmt"
	"os"
	"os/exec"
)

func ApplyYAMLFile(filePath string) error {
	fmt.Printf("Applying YAML file: %s\n", filePath)
	cmd := exec.Command("kubectl", "apply", "-f", filePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

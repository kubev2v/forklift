package yaml

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"text/template"
)

func ApplyTemplatedYAML(
	kubeconfig, fileOrURL string,
	vars map[string]string,
	leftDelim, rightDelim string,
) error {
	raw, err := os.ReadFile(fileOrURL)
	if err != nil {
		return fmt.Errorf("read %s: %w", fileOrURL, err)
	}

	tmpl, err := template.
		New("m").
		Delims(leftDelim, rightDelim).
		Parse(string(raw))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	cmd := exec.Command(
		"kubectl",
		"--kubeconfig", kubeconfig,
		"apply", "-f", "-",
	)
	cmd.Stdin = &buf
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply: %w", err)
	}
	return nil
}

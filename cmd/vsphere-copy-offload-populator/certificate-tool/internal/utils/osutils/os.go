package osutils

import (
	"k8s.io/klog/v2"
	"os"
	"os/exec"
)

func ExecCommand(name string, args ...string) error {
	klog.Infof("Executing: %s %v", name, args)
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

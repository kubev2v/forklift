package main

import (
	"fmt"
	"os"
	"os/exec"
)

// defineVmExec defines the domain from an XML file
// Some of the commands such as `virt-win-reg` do not allow multiple disk input.
// So this is a workaround for it as the `virt-win-reg` gets all the disks from the domain instead from the args.
func defineVmExec(xmlPath string) error {
	defineCmd := exec.Command("virsh", "define", xmlPath)
	defineCmd.Stdout = os.Stdout
	defineCmd.Stderr = os.Stderr
	fmt.Println("exec:", defineCmd)
	if err := defineCmd.Run(); err != nil {
		return fmt.Errorf("error executing virsh define command: %w", err)
	}
	return nil
}

// undefineVmExec undefines the domain
func undefineVmExec(domain string) error {
	undefineCmd := exec.Command("virsh", "undefine", domain)
	undefineCmd.Stdout = os.Stdout
	undefineCmd.Stderr = os.Stderr
	fmt.Println("exec:", undefineCmd)
	if err := undefineCmd.Run(); err != nil {
		return fmt.Errorf("error executing virsh define command: %w", err)
	}
	return nil
}

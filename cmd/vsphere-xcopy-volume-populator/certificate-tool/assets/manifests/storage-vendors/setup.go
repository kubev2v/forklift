package storage_vendors

import (
	"fmt"
	"golang.org/x/term"
	"log"
	"os/exec"
	"strings"
	"syscall"
)

var sudoPassword string
var configFiles = "./storage-vendors"
var valuesPath = configFiles + "/primera-values.yaml"

// RunCommand runs a shell command and tries without sudo first, then retries with sudo if needed.
func RunCommand(cmdStr string, args ...string) (string, error) {
	fmt.Println("Executing:", cmdStr)
	cmd := exec.Command(cmdStr, args...)
	output, err := cmd.CombinedOutput()
	fmt.Println("Output:", string(output))
	if err != nil {
		fmt.Println("⚠️ Command failed, retrying with sudo...")
		cmd = exec.Command("bash", "-c", fmt.Sprintf("echo '%s' | sudo -S %s", sudoPassword, cmdStr))
		cmd.Stdin = strings.NewReader(sudoPassword + "\n")
		output, err = cmd.CombinedOutput()
		fmt.Println("Sudo Output:", string(output))
	}
	return string(output), err
}

// AskForSudoPassword prompts the user for their sudo password before execution
func AskForSudoPassword() error {
	fmt.Print("Enter Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}
	sudoPassword = string(bytePassword)
	return nil
}

func setup(deleteCluster bool) {
	// Ask for sudo password at the beginning
	err := AskForSudoPassword()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if deleteCluster {
			fmt.Println("🔄 Cleaning up: Deleting Kind cluster...")
			RunCommand("kind delete cluster --name copy-offload-test")
			fmt.Println("✅ Kind cluster deleted!")
		}
	}()

	// 1. Install Kind (if not installed)
	fmt.Println("🔄 Checking if Kind is installed...")
	_, err = RunCommand("kind version")
	if err != nil {
		fmt.Println("⚠️  Kind not found. Installing...")
		_, err = RunCommand("curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64 && chmod +x ./kind && mv ./kind /usr/local/bin/")
		if err != nil {
			log.Fatalf("❌ Failed to install Kind: %v", err)
		}
		fmt.Println("✅ Kind installed successfully!")
	}

	// 2. Create a Kind cluster
	fmt.Println("🔄 Creating Kind cluster...")
	_, err = RunCommand("kind create cluster --name copy-offload-test")
	if err != nil {
		log.Fatalf("❌ Failed to create Kind cluster: %v", err)
	}
	fmt.Println("✅ Kind cluster created!")

	// 3. Get the container ID for the Kind node
	fmt.Println("🔄 Getting Kind container ID...")
	containerID, err := RunCommand("docker ps --filter name=copy-offload-test-control-plane --format '{{.ID}}'")
	if err != nil || strings.TrimSpace(containerID) == "" {
		log.Fatalf("❌ Failed to get Kind container ID: %v", err)
	}
	containerID = strings.TrimSpace(containerID)
	fmt.Println("✅ Kind container ID:", containerID)

	// 4. Install multipath-tools inside the Kind container
	fmt.Println("🔄 Updating package lists inside the Kind container...")
	_, err = RunCommand(fmt.Sprintf("docker exec %s apt update", containerID))
	if err != nil {
		log.Fatalf("❌ Failed to update package lists: %v", err)
	}

	fmt.Println("🔄 Installing multipath-tools inside the Kind container...")
	_, err = RunCommand(fmt.Sprintf("docker exec %s apt install -y multipath-tools", containerID))
	if err != nil {
		log.Fatalf("❌ Failed to install multipath-tools: %v", err)
	}
	fmt.Println("✅ Multipath-tools installed!")

	// 5. Modify `/lib/systemd/system/multipathd.service` to remove `ConditionVirtualization=!container`
	fmt.Println("🔄 Modifying multipathd service file...")
	_, err = RunCommand(fmt.Sprintf("docker exec %s sed -i '/ConditionVirtualization=!container/d' /lib/systemd/system/multipathd.service", containerID))
	if err != nil {
		log.Fatalf("❌ Failed to modify multipathd service file: %v", err)
	}
	fmt.Println("✅ ConditionVirtualization=!container removed!")

	// 6. Reload systemd daemon
	fmt.Println("🔄 Reloading systemd daemon...")
	_, err = RunCommand(fmt.Sprintf("docker exec %s systemctl daemon-reload", containerID))
	if err != nil {
		log.Fatalf("❌ Failed to reload systemd daemon: %v", err)
	}
	fmt.Println("✅ Systemd daemon reloaded!")

	// 7. Restart and check multipathd status
	fmt.Println("🔄 Restarting multipathd service...")
	_, err = RunCommand(fmt.Sprintf("docker exec %s systemctl restart multipathd", containerID))
	if err != nil {
		log.Fatalf("❌ Failed to restart multipathd: %v", err)
	}
	fmt.Println("✅ Multipathd restarted!")

	// 8. Install HPE CSI Driver using Helm
	fmt.Println("🔄 Adding HPE Helm repo...")
	RunCommand("helm repo add hpe https://hpe-storage.github.io/co-deployments")
	RunCommand("helm repo update")

	helmInstallCmd := fmt.Sprintf("helm install hpe-csi hpe/hpe-csi-driver --namespace kube-system -f %v", valuesPath)
	RunCommand(helmInstallCmd)
	fmt.Println("✅ HPE CSI Driver installed!")

	// 9. Waiting for all pods to be ready
	fmt.Println("🔄 Waiting for all pods to be ready....")
	_, err = RunCommand(fmt.Sprintf("kubectl wait pod \\\n--all \\\n--for=condition=Ready \\\n--namespace=kube-system --timeout=2m"))

	// 10. Creating secret, storage-class, pvc
	fmt.Println("🔄 Creating backend secret")
	var backendSecretPath = configFiles + "/storage-secret.yaml"
	_, err = RunCommand(fmt.Sprintf("kubectl apply -f %v", backendSecretPath))
	if err != nil {
		log.Fatalf("❌ Failed to create secret: %v", err)
	}
	fmt.Println("✅ Backend secret created!")

	fmt.Println("🔄 Creating storage class")
	var storageClass = configFiles + "/hpe-3par-xfs-fs-storageclass.yaml"
	_, err = RunCommand(fmt.Sprintf("kubectl apply -f %v", storageClass))
	if err != nil {
		log.Fatalf("❌ Failed to create storage class: %v", err)
	}
	fmt.Println("✅ storage class created!")

	fmt.Println("🔄 Creating pvc")
	var pvc = configFiles + "/pvc-hpe-3par-xfs.yaml"
	_, err = RunCommand(fmt.Sprintf("kubectl apply -f %v", pvc))
	if err != nil {
		log.Fatalf("❌ Failed to create pvc: %v", err)
	}
	fmt.Println("✅ pvc created!")

	// 9. Continuously print pod status until 'q' is entered
	fmt.Println("🔄 Fetching pvc status (Press 'q' to quit)...")
	fmt.Println("🚀 Kind cluster with multipathd and HPE CSI Driver is ready!")
}

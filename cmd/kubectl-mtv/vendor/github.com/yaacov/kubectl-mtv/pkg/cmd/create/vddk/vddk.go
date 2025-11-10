package vddk

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// detectContainerRuntime checks for available container runtime (podman or docker).
// Returns the command name and true if found, or empty string and false if neither is available.
func detectContainerRuntime() (string, error) {
	// Try podman first (preferred)
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman", nil
	}

	// Fall back to docker
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker", nil
	}

	return "", fmt.Errorf("neither podman nor docker is installed or available in PATH.\n" +
		"Please install one of the following:\n" +
		"  - Podman: https://podman.io/getting-started/installation\n" +
		"  - Docker: https://docs.docker.com/get-docker/")
}

// selectContainerRuntime determines which container runtime to use based on the provided preference.
// If runtimePreference is "auto" or empty, it auto-detects. Otherwise, it validates the specified runtime.
func selectContainerRuntime(runtimePreference string) (string, error) {
	// Normalize the preference
	if runtimePreference == "" {
		runtimePreference = "auto"
	}

	// Auto-detect if requested
	if runtimePreference == "auto" {
		return detectContainerRuntime()
	}

	// Validate explicit runtime choice
	if runtimePreference != "podman" && runtimePreference != "docker" {
		return "", fmt.Errorf("invalid runtime '%s': must be 'auto', 'podman', or 'docker'", runtimePreference)
	}

	// Check if the specified runtime is available
	if _, err := exec.LookPath(runtimePreference); err != nil {
		return "", fmt.Errorf("specified runtime '%s' is not installed or available in PATH.\n"+
			"Please install it or use --runtime=auto to auto-detect an available runtime", runtimePreference)
	}

	return runtimePreference, nil
}

// defaultDockerfile is the default Dockerfile content used when no custom Dockerfile is provided
const defaultDockerfile = `FROM registry.access.redhat.com/ubi8/ubi-minimal
USER 1001
COPY vmware-vix-disklib-distrib /vmware-vix-disklib-distrib
RUN mkdir -p /opt
ENTRYPOINT ["cp", "-r", "/vmware-vix-disklib-distrib", "/opt"]
`

// BuildImage builds (and optionally pushes) a VDDK image for MTV.
func BuildImage(tarGzPath, tag, buildDir, runtimePreference, platform, dockerfilePath string, verbosity int, push bool) error {
	// Select container runtime based on preference
	runtime, err := selectContainerRuntime(runtimePreference)
	if err != nil {
		return err
	}
	fmt.Printf("Using container runtime: %s\n", runtime)
	fmt.Printf("Target platform: %s\n", platform)

	if buildDir == "" {
		tmp, err := os.MkdirTemp("", "vddk-build-*")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer os.RemoveAll(tmp)
		buildDir = tmp
	}
	fmt.Printf("Using build directory: %s\n", buildDir)

	// Unpack tar.gz
	fmt.Println("Extracting VDDK tar.gz...")
	if err := extractTarGz(tarGzPath, buildDir, verbosity); err != nil {
		return fmt.Errorf("failed to extract tar.gz: %w", err)
	}

	// Find the extracted directory
	var distribDir string
	files, _ := os.ReadDir(buildDir)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "vmware-vix-disklib-distrib") && f.IsDir() {
			distribDir = f.Name()
			break
		}
	}
	if distribDir == "" {
		return fmt.Errorf("could not find vmware-vix-disklib-distrib directory after extraction")
	}

	// Determine Dockerfile content
	var df string
	if dockerfilePath != "" {
		// Read custom Dockerfile from provided path
		fmt.Printf("Using custom Dockerfile from: %s\n", dockerfilePath)
		dockerfileBytes, err := os.ReadFile(dockerfilePath)
		if err != nil {
			return fmt.Errorf("failed to read custom Dockerfile from %s: %w", dockerfilePath, err)
		}
		df = string(dockerfileBytes)
	} else {
		// Use default Dockerfile
		df = defaultDockerfile
	}

	// Write Dockerfile to build directory
	dockerfile := filepath.Join(buildDir, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte(df), 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	// Print Dockerfile if verbosity > 1 (debug level)
	if verbosity > 1 {
		fmt.Println("Dockerfile contents:")
		fmt.Println("---")
		fmt.Print(df)
		fmt.Println("---")
	}

	// Build image
	fmt.Printf("Building image with %s...\n", runtime)
	// Construct build command with platform
	buildArgs := []string{"build"}
	if platform != "" {
		// Use linux/<platform> format for container images
		buildArgs = append(buildArgs, "--platform", fmt.Sprintf("linux/%s", platform))
	}
	buildArgs = append(buildArgs, "-t", tag, ".")

	// Print command if verbose
	if verbosity > 0 {
		fmt.Printf("Running: %s %s\n", runtime, strings.Join(buildArgs, " "))
	}

	buildCmd := exec.Command(runtime, buildArgs...)
	buildCmd.Dir = buildDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("%s build failed: %w", runtime, err)
	}

	// Optionally push
	if push {
		fmt.Printf("Pushing image with %s...\n", runtime)

		// Print command if verbose
		if verbosity > 0 {
			fmt.Printf("Running: %s push %s\n", runtime, tag)
		}

		pushCmd := exec.Command(runtime, "push", tag)
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr
		if err := pushCmd.Run(); err != nil {
			return fmt.Errorf("%s push failed: %w", runtime, err)
		}
	}

	fmt.Println("VDDK image build complete.")
	return nil
}

func extractTarGz(tarGzPath, destDir string, verbosity int) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Use system tar command to extract
	args := []string{"-xzf", tarGzPath, "-C", destDir}

	// Print command if verbose
	if verbosity > 0 {
		fmt.Printf("Running: tar %s\n", strings.Join(args, " "))
	}

	cmd := exec.Command("tar", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tar extraction failed: %w", err)
	}

	return nil
}

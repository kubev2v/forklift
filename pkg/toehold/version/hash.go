package version

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// Default OVF / template hardware when Spec.Resources leaves CPU or memory unset.
const (
	DefaultCPU       int32 = 2
	DefaultMemoryMiB int32 = 4096
)

type diskHashInput struct {
	ContainerImage string `json:"containerImage"`
	SSHPublicKey   string `json:"sshPublicKey,omitempty"`
}

type configHashInput struct {
	Network   string `json:"network"`
	CPU       int32  `json:"cpu"`
	MemoryMiB int32  `json:"memoryMiB"`
}

// DiskHash fingerprints the base containerdisk image and guest customization
// that produces the disk artifact. sshPublicKey is the authorized_keys entry
// virt-customize installs; an empty value means no key is injected.
func DiskHash(spec api.ToeholdTemplateSpec, sshPublicKey string) string {
	return hash(diskHashInput{
		ContainerImage: spec.BaseDisk.ContainerImage,
		SSHPublicKey:   sshPublicKey,
	})
}

// ConfigHash fingerprints OVF hardware and network configuration.
func ConfigHash(spec api.ToeholdTemplateSpec) string {
	cpu := spec.Resources.CPU
	if cpu == 0 {
		cpu = DefaultCPU
	}
	mem := spec.Resources.MemoryMiB
	if mem == 0 {
		mem = DefaultMemoryMiB
	}
	return hash(configHashInput{
		Network:   spec.Network,
		CPU:       cpu,
		MemoryMiB: mem,
	})
}

func hash(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		sum := sha256.Sum256([]byte(fmt.Sprint(v)))
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

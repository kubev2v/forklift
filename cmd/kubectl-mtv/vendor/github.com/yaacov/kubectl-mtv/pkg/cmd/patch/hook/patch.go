package hook

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// PatchHookOptions contains the options for patching a hook.
type PatchHookOptions struct {
	ConfigFlags *genericclioptions.ConfigFlags
	Name        string
	Namespace   string

	Image           string
	ImageChanged    bool
	ServiceAccount  string
	SAChanged       bool
	Playbook        string
	PlaybookChanged bool
	Deadline        int64
	DeadlineChanged bool

	AAPJobTemplateID        int
	AAPJobTemplateIDChanged bool
	ClearAAP                bool
}

// PatchHook patches an existing hook resource.
func PatchHook(opts PatchHookOptions) error {
	klog.V(2).Infof("Patching hook '%s' in namespace '%s'", opts.Name, opts.Namespace)

	dynamicClient, err := client.GetDynamicClient(opts.ConfigFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	existing, err := dynamicClient.Resource(client.HooksGVR).Namespace(opts.Namespace).Get(context.TODO(), opts.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get hook '%s': %v", opts.Name, err)
	}

	if err := validatePatchOptions(existing, opts); err != nil {
		return err
	}

	specPatch := map[string]interface{}{}

	if opts.ImageChanged {
		specPatch["image"] = opts.Image
	}
	if opts.SAChanged {
		specPatch["serviceAccount"] = opts.ServiceAccount
	}
	if opts.PlaybookChanged {
		playbook := opts.Playbook
		if playbook != "" && !isBase64Encoded(playbook) {
			playbook = base64.StdEncoding.EncodeToString([]byte(playbook))
		}
		specPatch["playbook"] = playbook
	}
	if opts.DeadlineChanged {
		if opts.Deadline < 0 {
			return fmt.Errorf("deadline must be non-negative, got: %d", opts.Deadline)
		}
		specPatch["deadline"] = opts.Deadline
	}

	if opts.AAPJobTemplateIDChanged {
		if opts.AAPJobTemplateID <= 0 {
			return fmt.Errorf("--aap-job-template-id must be a positive integer")
		}
		specPatch["aap"] = map[string]interface{}{
			"jobTemplateId": opts.AAPJobTemplateID,
		}
	}
	if opts.ClearAAP {
		specPatch["aap"] = nil
	}

	if len(specPatch) == 0 {
		return fmt.Errorf("no changes specified; use flags to specify what to patch")
	}

	patchMap := map[string]interface{}{
		"spec": specPatch,
	}
	patchData, err := json.Marshal(patchMap)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %v", err)
	}

	klog.V(4).Infof("Applying patch to hook '%s': %s", opts.Name, string(patchData))

	_, err = dynamicClient.Resource(client.HooksGVR).Namespace(opts.Namespace).Patch(
		context.TODO(),
		opts.Name,
		types.MergePatchType,
		patchData,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch hook '%s': %v", opts.Name, err)
	}

	fmt.Printf("hook/%s patched\n", opts.Name)
	return nil
}

func validatePatchOptions(existing *unstructured.Unstructured, opts PatchHookOptions) error {
	_, hasAAP, _ := unstructured.NestedMap(existing.Object, "spec", "aap")

	settingAAP := opts.AAPJobTemplateIDChanged
	settingLocal := opts.ImageChanged || opts.PlaybookChanged
	clearing := opts.ClearAAP

	if settingAAP && settingLocal {
		return fmt.Errorf("--aap-job-template-id is mutually exclusive with --image and --playbook")
	}
	if settingAAP && clearing {
		return fmt.Errorf("--aap-job-template-id and --clear-aap are mutually exclusive")
	}

	if settingLocal && hasAAP && !clearing {
		return fmt.Errorf("hook currently has AAP configuration; use --clear-aap to switch to a local hook")
	}

	if clearing {
		resultImage := existingField(existing, "spec", "image")
		if opts.ImageChanged {
			resultImage = opts.Image
		}
		resultPlaybook := existingField(existing, "spec", "playbook")
		if opts.PlaybookChanged {
			resultPlaybook = opts.Playbook
		}
		if resultImage == "" && resultPlaybook == "" {
			return fmt.Errorf("clearing AAP requires at least --image or --playbook for the resulting local hook")
		}
	}

	if settingAAP && !hasAAP {
		existingImage, _, _ := unstructured.NestedString(existing.Object, "spec", "image")
		if existingImage != "" && !opts.ImageChanged {
			klog.V(2).Infof("Switching to AAP hook; existing image '%s' will remain on the spec (clear with --image '')", existingImage)
		}
	}

	return nil
}

func existingField(obj *unstructured.Unstructured, fields ...string) string {
	val, _, _ := unstructured.NestedString(obj.Object, fields...)
	return val
}

func isBase64Encoded(s string) bool {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\r", "")

	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil && len(s)%4 == 0
}

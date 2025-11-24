package k8s

import (
	"bytes"
	"fmt"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"
	"net/http"
	"os"
	"strings"
	"text/template"

	"k8s.io/client-go/kubernetes"
)

type TemplateParams struct {
	TestNamespace      string
	TestImageLabel     string
	TestLabels         string
	TestPopulatorImage string
	PodNamespace       string
	VmdkPath           string
	StorageVendor      string
	StorageClassName   string
}

// ProcessTemplate reads a file and processes it as a Go template using the provided variables.
func ProcessTemplate(filePath string, vars map[string]string, leftDelim, rightDelim string) ([]byte, error) {
	rawData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	tmpl, err := template.New("template").Delims(leftDelim, rightDelim).Parse(string(rawData))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.Bytes(), nil
}

func Decode[T any](data []byte) (*T, error) {
	dec := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 1024)

	var obj T
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("decode %T: %w", obj, err)
	}
	return &obj, nil
}

// ToMap returns the map[string]string your templates expect.
func (p *TemplateParams) ToMap() map[string]string {
	return map[string]string{
		"TEST_NAMESPACE":       p.TestNamespace,
		"TEST_IMAGE_LABEL":     p.TestImageLabel,
		"TEST_LABELS":          p.TestLabels,
		"TEST_POPULATOR_IMAGE": p.TestPopulatorImage,
		"POD_NAMESPACE":        p.PodNamespace,
		"VMDK_PATH":            p.VmdkPath,
		"STORAGE_VENDOR":       p.StorageVendor,
		"STORAGE_CLASS_NAME":   p.StorageClassName,
	}
}

// LoadAndDecode loads a fileOrURL, runs your Go template over it,
// then decodes into the typed K8s object T.
func LoadAndDecode[T any](
	fileOrURL string,
	params *TemplateParams,
	leftDelim, rightDelim string,
) (*T, error) {
	raw, err := os.ReadFile(fileOrURL)
	if err != nil && (strings.HasPrefix(fileOrURL, "http://") || strings.HasPrefix(fileOrURL, "https://")) {
		var resp *http.Response
		resp, err = http.Get(fileOrURL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		raw, err = io.ReadAll(resp.Body)
	}
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", fileOrURL, err)
	}

	tmpl, err := template.
		New("m").
		Delims(leftDelim, rightDelim).
		Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params.ToMap()); err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}

	return Decode[T](buf.Bytes())
}

func ApplyResource[T any](
	fileOrURL string,
	params *TemplateParams,
	leftDelim, rightDelim string,
	ensure func(clientset *kubernetes.Clientset, namespace string, obj *T) error,
	clientset *kubernetes.Clientset,
	namespace string,
) error {
	obj, err := LoadAndDecode[T](fileOrURL, params, leftDelim, rightDelim)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", fileOrURL, err)
	}
	if err := ensure(clientset, namespace, obj); err != nil {
		return fmt.Errorf("ensuring %T : %w", obj, err)
	}
	return nil
}

func ApplyPVCFromTemplate(clientset *kubernetes.Clientset, namespace, pvcName, size, storageClassName, yamlPath string) error {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return fmt.Errorf("read PVC template: %w", err)
	}
	var pvc corev1.PersistentVolumeClaim
	if err := yaml.Unmarshal(data, &pvc); err != nil {
		return fmt.Errorf("unmarshal PVC template: %w", err)
	}
	pvc.Name = pvcName
	pvc.Namespace = namespace
	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(size)
	pvc.Spec.StorageClassName = &storageClassName
	return EnsurePersistentVolumeClaim(clientset, namespace, &pvc)
}

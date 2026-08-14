package nutanix

import (
	"bytes"
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	core "k8s.io/api/core/v1"
)

func TestConfigMapSetsCDICertKeys(t *testing.T) {
	cacert := []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----")
	secret := &core.Secret{
		Data: map[string][]byte{
			"ca.crt": cacert,
		},
	}
	configMap := &core.ConfigMap{}
	builder := &Builder{}

	err := builder.ConfigMap(ref.Ref{}, secret, configMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(configMap.BinaryData["ca.pem"], cacert) {
		t.Fatalf("expected ca.pem to match provider CA")
	}
	if !bytes.Equal(configMap.BinaryData["tls.crt"], cacert) {
		t.Fatalf("expected tls.crt to match provider CA for CDI nbdkit cainfo")
	}
}

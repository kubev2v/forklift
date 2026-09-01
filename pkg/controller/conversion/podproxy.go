package conversion

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	ocp "github.com/kubev2v/forklift/pkg/lib/client/openshift"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	k8snet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const inspectionProxyTimeout = 5 * time.Second

// inspectionPodDo talks to the deep-inspection HTTP server through the
// destination API server's pods/proxy subresource. Tests replace this
// so they do not need a real REST config or Pod CIDR routing.
var inspectionPodDo inspectionPodDoFunc = proxyInspectionPod

type inspectionPodDoFunc func(ctx context.Context, localClient client.Client, spec api.ConversionSpec, method, namespace, name, path string) (statusCode int, body []byte, err error)

// destinationRESTConfig builds a REST config for the cluster that hosts the
// Conversion pod. Empty Destination uses the management cluster.
func destinationRESTConfig(localClient client.Client, spec api.ConversionSpec) (*rest.Config, error) {
	if spec.Destination.Name == "" {
		cfg, err := config.GetConfig()
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		return cfg, nil
	}

	provider := &api.Provider{}
	err := localClient.Get(context.TODO(), client.ObjectKey{
		Namespace: spec.Destination.Namespace,
		Name:      spec.Destination.Name,
	}, provider)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	var secret *core.Secret
	if !provider.IsHost() {
		secret = &core.Secret{}
		err = localClient.Get(context.TODO(), client.ObjectKey{
			Namespace: provider.Spec.Secret.Namespace,
			Name:      provider.Spec.Secret.Name,
		}, secret)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
	}

	cfg := ocp.RestCfg(provider, secret)
	if cfg == nil {
		return nil, fmt.Errorf("failed to build destination REST config for provider %s/%s",
			provider.Namespace, provider.Name)
	}
	return cfg, nil
}

// proxyInspectionPod issues method+path against the pod via pods/proxy so the
// management-cluster controller never dials a destination Pod IP.
func proxyInspectionPod(ctx context.Context, localClient client.Client, spec api.ConversionSpec, method, namespace, name, path string) (int, []byte, error) {
	cfg, err := destinationRESTConfig(localClient, spec)
	if err != nil {
		return 0, nil, err
	}
	cfg = rest.CopyConfig(cfg)
	cfg.Timeout = inspectionProxyTimeout
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return 0, nil, err
	}
	path = strings.TrimPrefix(path, "/")
	var statusCode int
	body, err := cs.CoreV1().RESTClient().Verb(method).
		Namespace(namespace).
		Resource("pods").
		Name(k8snet.JoinSchemeNamePort("", name, strconv.Itoa(inspectionResultPort))).
		SubResource("proxy").
		Suffix(path).
		Do(ctx).
		StatusCode(&statusCode).
		Raw()
	return statusCode, body, err
}

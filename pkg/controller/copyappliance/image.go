package copyappliance

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
)

// openShiftInternalRegistry is the in-cluster OpenShift image registry host:port.
const openShiftInternalRegistry = "image-registry.openshift-image-registry.svc:5000"

// isPullSpec reports whether image names a registry host. The setup pod reads
// the image with no credential of its own and no cluster-local resolution, so a
// reference without a registry would be defaulted to Docker Hub.
func isPullSpec(image string) bool {
	host, _, found := strings.Cut(image, "/")
	return found && (host == "localhost" || strings.ContainsAny(host, ".:"))
}

// isOpenShiftInternalRegistry reports whether pullSpec's registry is exactly
// openShiftInternalRegistry.
func isOpenShiftInternalRegistry(pullSpec string) bool {
	ref, err := name.ParseReference(pullSpec)
	if err != nil {
		return false
	}
	return ref.Context().RegistryStr() == openShiftInternalRegistry
}

// resolveImage returns the metadata for the appliance's container image and the
// digest-pinned reference it was read from. Only the manifest and the config
// are fetched; the layers are read on demand.
//
// The returned spec is by digest so that whoever reads the layers later gets
// the same image this metadata came from, even if the tag has moved since.
func resolveImage(ctx context.Context, image string) (img v1.Image, spec string, err error) {
	img, err = registryImage(ctx, image)
	if err != nil {
		return
	}
	digest, err := img.Digest()
	if err != nil {
		err = liberr.Wrap(err, "image", image)
		return
	}
	ref, err := name.ParseReference(image)
	if err != nil {
		err = liberr.Wrap(err, "image", image)
		return
	}
	spec = ref.Context().Digest(digest.String()).Name()
	return
}

// registryImage returns the metadata for a pull spec, read with the ambient
// keychain over the system roots. For the OpenShift internal registry it uses
// the pod SA token and service-ca.crt instead.
func registryImage(ctx context.Context, spec string) (img v1.Image, err error) {
	ref, err := name.ParseReference(spec)
	if err != nil {
		err = liberr.Wrap(err, "image", spec)
		return
	}
	opts := []remote.Option{remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain)}
	if isOpenShiftInternalRegistry(spec) {
		opts = []remote.Option{remote.WithContext(ctx)}
		if token, e := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token"); e == nil && len(token) > 0 {
			opts = append(opts, remote.WithAuth(&authn.Basic{Username: "image-puller", Password: string(token)}))
		}
		pem, e := os.ReadFile(settings.ServiceCAFile)
		if e != nil {
			err = liberr.Wrap(e, "image", spec)
			return
		}
		pool, _ := x509.SystemCertPool()
		if pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			err = liberr.New("no certificates in service-ca.crt", "image", spec)
			return
		}
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
		opts = append(opts, remote.WithTransport(tr))
	}
	img, err = remote.Image(ref, opts...)
	if err != nil {
		err = liberr.Wrap(err, "image", spec)
		return
	}
	return
}

// streamImage writes the image into the appliance's podman store as a docker
// archive on the load command's standard input.
func streamImage(client *SSHClient, img v1.Image, ref name.Tag) (err error) {
	reader, writer := io.Pipe()
	go func() {
		// A failure part way through arrives at the load side as a read error,
		// rather than as a truncated archive that podman would reject for the
		// wrong reason.
		_ = writer.CloseWithError(tarball.Write(ref, img, writer))
	}()
	// Closing the read half is what unblocks the writer when the load command
	// gives up before the archive is finished.
	defer func() {
		_ = reader.Close()
	}()

	err = client.RunWithStdin(AppliancePodmanLoadCommand, reader)
	return
}

// makeTag makes a tag for the image from its digest.
func makeTag(img v1.Image) (tag name.Tag, err error) {
	digest, err := img.Digest()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	tag, err = name.NewTag(fmt.Sprintf("%s:%s", ApplianceContainerImageName, digest.Hex), name.WithDefaultRegistry(""))
	if err != nil {
		err = liberr.Wrap(err, "digest", digest.String())
		return
	}
	return
}

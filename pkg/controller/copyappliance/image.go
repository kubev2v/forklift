package copyappliance

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// isPullSpec reports whether image names a registry host. The setup pod reads
// the image with no credential of its own and no cluster-local resolution, so a
// reference without a registry would be defaulted to Docker Hub.
func isPullSpec(image string) bool {
	host, _, found := strings.Cut(image, "/")
	return found && (host == "localhost" || strings.ContainsAny(host, ".:"))
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
// keychain over the system roots.
func registryImage(ctx context.Context, spec string) (img v1.Image, err error) {
	ref, err := name.ParseReference(spec)
	if err != nil {
		err = liberr.Wrap(err, "image", spec)
		return
	}
	img, err = remote.Image(ref,
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(authn.DefaultKeychain))
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

package copyappliance

import (
	"bytes"
	"context"
	"io"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// testRegistry serves a registry out of memory and returns the host to address
// it by. It is on the loopback address, which go-containerregistry reaches over
// plain HTTP.
func testRegistry(t *testing.T, handler http.Handler) string {
	t.Helper()
	if handler == nil {
		handler = registry.New(registry.Logger(stdlog.New(io.Discard, "", 0)))
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://")
}

// pushImage puts a synthetic image in the registry and returns the spec that
// pulls it back.
func pushImage(t *testing.T, host string) (spec string, img v1.Image) {
	t.Helper()
	img, err := random.Image(256, 2)
	if err != nil {
		t.Fatalf("random image: %v", err)
	}
	ref, err := name.NewTag(host + "/forklift/copy-appliance:latest")
	if err != nil {
		t.Fatalf("tag: %v", err)
	}
	err = remote.Write(ref, img)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	return ref.Name(), img
}

// digestOf is an image's manifest digest, which is how two images are told
// apart here.
func digestOf(t *testing.T, img v1.Image) string {
	t.Helper()
	digest, err := img.Digest()
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	return digest.String()
}

// A tag moves. The setup pod is handed the reference the controller read the
// manifest from, so that the layers it streams are the ones the digest in
// Status.ExporterImage was taken over.
func TestResolveImage(t *testing.T) {
	t.Run("the resolved spec names the digest", func(t *testing.T) {
		spec, want := pushImage(t, testRegistry(t, nil))

		img, resolved, err := resolveImage(context.TODO(), spec)
		if err != nil {
			t.Fatalf("resolveImage: %v", err)
		}

		if digestOf(t, img) != digestOf(t, want) {
			t.Errorf("digest = %s, want %s", digestOf(t, img), digestOf(t, want))
		}
		if !strings.HasSuffix(resolved, "@"+digestOf(t, want)) {
			t.Errorf("resolved spec = %q, want it pinned to %s", resolved, digestOf(t, want))
		}
	})

	t.Run("an image that is not there fails naming the spec", func(t *testing.T) {
		spec := testRegistry(t, nil) + "/forklift/copy-appliance:never-pushed"

		_, _, err := resolveImage(context.TODO(), spec)

		if err == nil {
			t.Fatal("resolveImage succeeded for an image that was never pushed")
		}
		if !errorMentions(t, err, spec) {
			t.Errorf("error = %q, want it to name the image", err)
		}
	})
}

// The setup pod reads the image with no credential and no cluster-local
// resolution, so a reference the registry cannot be read off of would be
// defaulted to Docker Hub and fail as a pull of someone else's image.
func TestIsPullSpec(t *testing.T) {
	tests := []struct {
		image string
		want  bool
	}{
		{"quay.io/kubev2v/nbd-container:latest", true},
		{"registry:5000/kubev2v/nbd-container:latest", true},
		{"localhost/kubev2v/nbd-container:latest", true},
		{"nbd-container:latest", false},
		{"kubev2v/nbd-container:latest", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.image, func(t *testing.T) {
			if got := isPullSpec(tc.image); got != tc.want {
				t.Errorf("isPullSpec(%q) = %v, want %v", tc.image, got, tc.want)
			}
		})
	}
}

// The appliance is asked about the reference by name, so the reference has to
// change when the image behind the tag does. Otherwise a rebuilt appliance
// image is never picked up: the store still answers to the old name.
func TestMakeTag(t *testing.T) {
	first, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("random image: %v", err)
	}
	second, err := random.Image(512, 1)
	if err != nil {
		t.Fatalf("random image: %v", err)
	}

	ref, err := makeTag(first)
	if err != nil {
		t.Fatalf("makeTag: %v", err)
	}
	if ref.Repository.Name() != ApplianceContainerImageName {
		t.Errorf("repository = %q, want %q", ref.Repository.Name(), ApplianceContainerImageName)
	}
	digest, err := first.Digest()
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if !strings.HasPrefix(digest.Hex, ref.TagStr()) || ref.TagStr() == "" {
		t.Errorf("tag = %q, want a prefix of the digest %q", ref.TagStr(), digest.Hex)
	}

	other, err := makeTag(second)
	if err != nil {
		t.Fatalf("makeTag: %v", err)
	}
	if other.Name() == ref.Name() {
		t.Errorf("both images are %q, want two different references", ref.Name())
	}
}

func TestStreamImage(t *testing.T) {
	t.Run("what the appliance is fed reads back as the image", func(t *testing.T) {
		_, server, client := applianceLogin(t)
		img, err := random.Image(4096, 3)
		if err != nil {
			t.Fatalf("random image: %v", err)
		}
		ref, err := makeTag(img)
		if err != nil {
			t.Fatalf("makeTag: %v", err)
		}

		err = streamImage(client, img, ref)
		if err != nil {
			t.Fatalf("streamImage: %v", err)
		}

		archive, ran := server.Stdin(AppliancePodmanLoadCommand)
		if !ran {
			t.Fatalf("ran %v, want %q", server.Ran(), AppliancePodmanLoadCommand)
		}
		opener := func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(archive)), nil
		}
		// The name the archive files the image under is the name the appliance
		// is asked about on the next pass, and it is recorded from ref.Name().
		// If those two ever disagree the probe never matches and every pass
		// sends the image again.
		manifest, err := tarball.LoadManifest(opener)
		if err != nil {
			t.Fatalf("read back the manifest: %v", err)
		}
		if len(manifest) != 1 || !slices.Contains(manifest[0].RepoTags, ref.Name()) {
			t.Errorf("archive holds %+v, want one image tagged %q", manifest, ref.Name())
		}
		// By position rather than by tag. tarball's own lookup re-parses the
		// RepoTag with a default registry and would not find a reference that
		// deliberately has none; podman matches the name as written.
		loaded, err := tarball.Image(opener, nil)
		if err != nil {
			t.Fatalf("read back the archive: %v", err)
		}
		config, err := loaded.ConfigName()
		if err != nil {
			t.Fatalf("config: %v", err)
		}
		want, err := img.ConfigName()
		if err != nil {
			t.Fatalf("config: %v", err)
		}
		if config != want {
			t.Errorf("config = %s, want %s", config, want)
		}
		layers, err := loaded.Layers()
		if err != nil {
			t.Fatalf("layers: %v", err)
		}
		if len(layers) != 3 {
			t.Errorf("layers = %d, want 3", len(layers))
		}
	})

	// A load that fails is the end of the deploy, so it has to say what the
	// appliance said rather than that a pipe broke.
	t.Run("a load the appliance refuses carries its output into the error", func(t *testing.T) {
		_, _, client := applianceLogin(t, AppliancePodmanLoadCommand)
		img, err := random.Image(4096, 2)
		if err != nil {
			t.Fatalf("random image: %v", err)
		}
		ref, err := makeTag(img)
		if err != nil {
			t.Fatalf("makeTag: %v", err)
		}

		err = streamImage(client, img, ref)
		if err == nil {
			t.Fatal("streamImage succeeded against an appliance that refused the load")
		}
		if !errorMentions(t, err, commandFailureOutput) {
			t.Errorf("error = %q, want it to carry %q", err, commandFailureOutput)
		}
	})
}

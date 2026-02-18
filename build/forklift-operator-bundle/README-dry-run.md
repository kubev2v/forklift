# Dry-run bundle (staged components only)

The **dry-run** bundle is for testing staged components only. It does **not** replace the upstream (`Containerfile`) or downstream (`Containerfile-downstream`) methodology.

- **Containerfile:** `build/forklift-operator-bundle/Containerfile-dry-run`
- **Based on:** **Containerfile-downstream** (same manifest source, relatedImages injection, labels). Only the image refs change to `registry.stage.redhat.io`.
- **Image defaults:** `registry.stage.redhat.io/migration-toolkit-virtualization` (mtv-* image names)
- **Versioning:** From **build/release.conf only** (required; no defaults so it stays in sync with downstream)

## Why downstream-based

Dry-run is built from the **downstream** Containerfile (with staged image refs) rather than upstream so that:

- The bundle is **identical in shape** to what will ship (same `.downstream_manifests`, same CSV).
- **relatedImages** are injected the same way (disconnected / MTV-3558).
- You validate the real release path; only the registry/tag changes when promoting to production.

## Tekton pipeline

- **Pipeline:** `.tekton/forklift-operator-bundle-dry-run-2-10-push.yaml`
- **Triggers on:** push to `release-2.10` when `Containerfile-dry-run`, `build/release.conf`, or `operator/config/manifests/` change.
- **Output image:** `quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator-2-10/forklift-operator-bundle-dry-run-2-10:{{revision}}`
- **Build-args file:** `build/release.conf` (provides MTV_VERSION, RELEASE, CHANNEL, DEFAULT_CHANNEL, OCP_VERSIONS, CPE, REGISTRY, REVISION)
- **Build-args:** Staged image refs with `:{{revision}}` (registry.stage.redhat.io/.../mtv-*:{{revision}})

## Local build

Requires **build/release.conf** (no version defaults in the dry-run Containerfile):

```bash
buildah build \
  -f build/forklift-operator-bundle/Containerfile-dry-run \
  --build-arg-file build/release.conf \
  --build-arg REVISION=local \
  --build-arg CONTROLLER_IMAGE=registry.stage.redhat.io/migration-toolkit-virtualization/mtv-controller-rhel9:your-tag \
  # ... other *_IMAGE args as needed ...
  -t forklift-operator-bundle-dry-run:local .
```

## Validating the bundle (FBC / certification)

The dry-run pipeline validates the built bundle so it fits FBC and certification expectations:

- **ecosystem-cert-preflight-checks** – Runs after the bundle image is built (when `skip-checks` is not set). Uses the same Konflux task as other bundle pipelines; no change to the implementation.
- **Bundle structure** – The image is a standard OLM bundle (manifests/ + metadata/). You can validate it the same way as any other bundle:
  - **operator-sdk:** `operator-sdk bundle validate <dry-run-bundle-image>` (e.g. the pipeline output image by digest or tag).
  - **Optional suites:** `operator-sdk bundle validate <image> --select-optional suite=operatorframework` (or operatorhub / good-practices as needed).

No changes are required to FBC or to how you run `operator-sdk bundle validate`. The dry-run output is a normal bundle image; point your validation at that image (or at the pipeline result IMAGE_URL/IMAGE_DIGEST). To run validation in-pipeline, add a task that runs `operator-sdk bundle validate` against `$(tasks.build-image-index.results.IMAGE_URL)@$(tasks.build-image-index.results.IMAGE_DIGEST)` if you want that in Tekton as well.

## Summary

| Build type   | Containerfile            | Images default        | Versioning              |
|-------------|--------------------------|------------------------|--------------------------|
| Upstream    | Containerfile            | quay.io/kubev2v        | Optional (defaults)      |
| Dry run     | Containerfile-dry-run    | registry.stage.redhat.io | build/release.conf only |
| Downstream  | Containerfile-downstream | registry.redhat.io / mtv-candidate | build/release.conf      |

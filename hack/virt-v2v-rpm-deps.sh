#!/usr/bin/env bash

set -e

bazeldnf_repos="--repofile rpm/stream9-repo.yaml"
if [ "${CUSTOM_REPO}" ]; then
    bazeldnf_repos="--repofile ${CUSTOM_REPO} ${bazeldnf_repos}"
fi

# get latest repo data from repo.yaml
bazel run \
    //:bazeldnf -- fetch \
    ${bazeldnf_repos}

# These are the packages we really depend on.
virt_v2v="
  tar
  virt-v2v
  virtio-win
"

# Here are the dependencies that virt-v2v requires but does not specify as
# dependency (either directly or indirectly).
v2v_missing="
  file
"

# bazeldnf cannot handle properly alternative packages and keeps swapping e.g.
# libcurl and libcurl-minimal, language alternatives or kernel core on each
# run. To make the list of packages stable we pick our preferred alternatives
# here. We don't really have a strong preference for either of those and the
# choice is mostly arbitrary with package size taken into consideration.
alternative_picks="
  coreutils-single
  curl-minimal
  glibc-langpack-en
  kernel-core
  libcurl-minimal
  libverto-libev
  selinux-policy-targeted
"
bazel run \
        //:bazeldnf -- rpmtree \
        --public \
        --workspace virt-v2v/WORKSPACE \
        --buildfile virt-v2v/BUILD.bazel \
        --name virt-v2v \
        --basesystem centos-stream-release \
	--force-ignore-with-dependencies 'python' \
        ${bazeldnf_repos} \
        $virt_v2v $v2v_missing $alternative_picks


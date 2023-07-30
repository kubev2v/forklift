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
imageio="gcc python3-pip python3-devel libxml2-devel openssl-devel libcurl-devel qemu-img"

bazel run \
        //:bazeldnf -- rpmtree \
        --public \
        --nobest \
        --buildfile cmd/ovirt-populator/BUILD.bazel \
        --name deps \
        --basesystem centos-stream-release \
        ${bazeldnf_repos} \
        $imageio


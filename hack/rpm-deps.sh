#!/usr/bin/env bash

bazeldnf_repos="--repofile rpm/repo.yaml"
if [ "${CUSTOM_REPO}" ]; then
    bazeldnf_repos="--repofile ${CUSTOM_REPO} ${bazeldnf_repos}"
fi

# get latest repo data from repo.yaml
bazel run \
    //:bazeldnf -- fetch \
    ${bazeldnf_repos}

virt_v2v="
  qemu-guest-agent
  qemu-img
  qemu-kvm
  virt-v2v
  virtio-win
"

bazel run \
        //:bazeldnf -- rpmtree \
        --public --nobest \
        --name virt-v2v \
        --basesystem centos-stream-release \
        ${bazeldnf_repos} \
        $virt_v2v


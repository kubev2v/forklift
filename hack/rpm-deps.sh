#!/usr/bin/env bash

set -e 

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

ovirt_imageio="
  python3-devel
  python3-ovirt-engine-sdk4
  ovirt-imageio-common
  qemu-img
"

bazel run \
        //:bazeldnf -- rpmtree \
        --public --nobest \
        --name virt-v2v \
        --basesystem centos-stream-release \
        ${bazeldnf_repos} \
        $virt_v2v

bazel run \
        //:bazeldnf -- rpmtree \
        --public --nobest \
        --name ovirt-imageio \
        --basesystem centos-stream-release \
        ${bazeldnf_repos} \
        $ovirt_imageio

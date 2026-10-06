FROM registry.fedoraproject.org/fedora:44 AS appliance

RUN dnf install -y --setopt=install_weak_deps=False \
        qemu-img \
        libguestfs \
        libguestfs-xfs \
        supermin \
    && dnf clean all

RUN mkdir -p /usr/lib64/guestfs/appliance && \
    cd /usr/lib64/guestfs/appliance && \
    LIBGUESTFS_BACKEND=direct libguestfs-make-fixed-appliance . && \
    qemu-img convert -c -O qcow2 root root.qcow2 && \
    mv -vf root.qcow2 root

FROM registry.access.redhat.com/ubi9/go-toolset:1.25.9-1778604137 AS gobuild
USER 0
WORKDIR /build
COPY . .
ENV GOFLAGS="-mod=vendor"
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -ldflags="-w -s" -o /toehold-uploader ./cmd/toehold-uploader

FROM registry.fedoraproject.org/fedora:44

RUN dnf install -y --setopt=install_weak_deps=False \
        qemu-img \
        libguestfs-tools \
        libguestfs-xfs \
        libguestfs \
        virt-customize \
        catatonit \
    && dnf clean all

ENV LIBGUESTFS_BACKEND=direct

COPY --from=appliance /usr/lib64/guestfs/appliance /usr/lib64/guestfs/appliance
COPY --from=gobuild /toehold-uploader /usr/local/bin/toehold-uploader
COPY build/toehold/scripts/extract-appliance-root.sh /usr/local/bin/extract-appliance-root
COPY build/toehold/scripts/toehold-build.sh /usr/local/bin/toehold-build
COPY build/toehold/scripts/bake-appliance-tarball.sh /usr/local/bin/bake-appliance-tarball
COPY build/toehold/scripts/toehold-publish-guestinfo.sh /usr/local/bin/toehold-publish-guestinfo.sh
COPY build/toehold/scripts/toehold-podman.sh /usr/local/bin/toehold-podman.sh
COPY build/toehold/systemd/ /usr/share/toehold/systemd/
RUN chmod +x /usr/local/bin/extract-appliance-root \
        /usr/local/bin/toehold-build \
        /usr/local/bin/toehold-uploader \
        /usr/local/bin/bake-appliance-tarball \
        /usr/local/bin/toehold-publish-guestinfo.sh \
        /usr/local/bin/toehold-podman.sh \
    && bake-appliance-tarball

ENTRYPOINT ["/usr/libexec/catatonit/catatonit", "/usr/local/bin/toehold-build"]

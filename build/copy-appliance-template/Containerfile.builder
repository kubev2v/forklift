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
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -ldflags="-w -s" -o /copy-appliance-template-uploader ./cmd/copy-appliance-template-uploader

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
COPY --from=gobuild /copy-appliance-template-uploader /usr/local/bin/copy-appliance-template-uploader
COPY build/copy-appliance-template/scripts/extract-appliance-root.sh /usr/local/bin/extract-appliance-root
COPY build/copy-appliance-template/scripts/copy-appliance-template-build.sh /usr/local/bin/copy-appliance-template-build
COPY build/copy-appliance-template/scripts/bake-appliance-tarball.sh /usr/local/bin/bake-appliance-tarball
COPY build/copy-appliance-template/scripts/copy-appliance-template-publish-guestinfo.sh /usr/local/bin/copy-appliance-template-publish-guestinfo.sh
COPY build/copy-appliance-template/scripts/copy-appliance-template-podman.sh /usr/local/bin/copy-appliance-template-podman.sh
COPY build/copy-appliance-template/systemd/ /usr/share/copy-appliance-template/systemd/
RUN chmod +x /usr/local/bin/extract-appliance-root \
        /usr/local/bin/copy-appliance-template-build \
        /usr/local/bin/copy-appliance-template-uploader \
        /usr/local/bin/bake-appliance-tarball \
        /usr/local/bin/copy-appliance-template-publish-guestinfo.sh \
        /usr/local/bin/copy-appliance-template-podman.sh \
    && bake-appliance-tarball

ENTRYPOINT ["/usr/libexec/catatonit/catatonit", "/usr/local/bin/copy-appliance-template-build"]

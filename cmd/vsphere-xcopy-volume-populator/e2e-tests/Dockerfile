FROM golang:1.24 AS builder

WORKDIR /forklift

# Copy go module files from the context (project root)
COPY go.mod go.sum ./
RUN go mod vendor

# Copy go module files for the populator
WORKDIR /forklift/cmd/vsphere-xcopy-volume-populator
COPY go.mod go.sum ./
RUN go mod vendor

# Return to the project root for subsequent operations
WORKDIR /forklift

# Copy the necessary application source code.
COPY cmd/ ./cmd/
COPY pkg/ ./pkg/
COPY tests/ ./tests/

# Build the test binary
WORKDIR /forklift/cmd/vsphere-xcopy-volume-populator
RUN CGO_ENABLED=0 go test -c -o /tests.test ./e2e-tests/tests

# Final stage
FROM golang:1.24

# Define versions and checksums for security and reproducibility
ENV OC_CLIENT_VERSION=4.14.8
ENV OC_CLIENT_SHA256=2c992ec3beecc9bf305a2a35035262afe515de2661737cc5e14d112ee1d9c7c6
ENV ANSIBLE_VERSION=9.5.1
ENV PYVMOMI_VERSION=8.0.3.0.1

# Set up and activate a virtual environment to safely install Python packages
ENV VIRTUAL_ENV=/opt/venv
ENV PATH="$VIRTUAL_ENV/bin:$PATH"

# Install dependencies, including Ansible, oc, and kubectl
RUN apt-get update -y && apt-get install -y --no-install-recommends \
    python3-pip \
    python3-venv \
    wget \
    ca-certificates \
    && python3 -m venv ${VIRTUAL_ENV} \
    && pip3 install --no-cache-dir \
        ansible==${ANSIBLE_VERSION} \
        pyvmomi==${PYVMOMI_VERSION} \
        packaging \
        requests \
    && ansible-galaxy collection install community.vmware \
    && rm -rf /var/lib/apt/lists/* && \
    wget -q https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${OC_CLIENT_VERSION}/openshift-client-linux-${OC_CLIENT_VERSION}.tar.gz -O openshift-client-linux.tar.gz && \
    echo "${OC_CLIENT_SHA256}  openshift-client-linux.tar.gz" | sha256sum --check --strict && \
    tar -zxvf openshift-client-linux.tar.gz && \
    mv oc /usr/local/bin/oc && \
    mv kubectl /usr/local/bin/kubectl && \
    rm openshift-client-linux.tar.gz README.md

WORKDIR /forklift

# Copy the test binary from the builder stage
COPY --from=builder /tests.test .

# Copy the test runner script and make it executable
COPY cmd/vsphere-xcopy-volume-populator/e2e-tests/run-tests.sh /usr/local/bin/run-tests.sh
RUN chmod +x /usr/local/bin/run-tests.sh

# Copy ansible playbooks
COPY cmd/vsphere-xcopy-volume-populator/e2e-tests/ansible /ansible

# Copy helper scripts
COPY cmd/vsphere-xcopy-volume-populator/e2e-tests/helpers /helpers

# Create a non-root user for security, and change ownership of the relevant files.
RUN groupadd --gid 1001 runner && \
    useradd --uid 1001 --gid 1001 --shell /bin/bash --create-home runner && \
    mkdir -p /forklift/cmd/vsphere-xcopy-volume-populator/e2e-tests/logs && \
    chown -R runner:runner /forklift /ansible /helpers && \
    mkdir -p /home/runner/.ssh && \
    chown runner:runner /home/runner/.ssh

# Switch to the non-root user
USER runner

# Set the default SSH key path for the runner user
ENV SSH_KEY_PATH=/home/runner/.ssh/id_rsa

ENTRYPOINT ["/usr/local/bin/run-tests.sh"]

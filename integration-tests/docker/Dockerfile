# Integration test image: a real subscription-manager environment per UBI
# major, with the daemon under test built from source by the suite. This is
# deliberately separate from the .devcontainer image, which is the CentOS
# Stream based RPM build environment.
ARG UBI_VERSION=10

FROM registry.access.redhat.com/ubi${UBI_VERSION}/ubi:latest

# Install Go, Python 3, pip, git, openssl, and system utilities.
# UBI 8 ships Python 3.6 only; enable the python39 module there and make it
# the default python3 (the integration test dependencies require a newer
# Python).
RUN . /etc/os-release && \
    if [ "${VERSION_ID%%.*}" = "8" ]; then \
        dnf module enable -y python39 && \
        dnf install -y golang python39 python39-pip git make openssl diffutils procps-ng && \
        ln -sf /usr/bin/python3.9 /usr/bin/python3 && \
        ln -sf /usr/bin/pip3.9 /usr/bin/pip3; \
    else \
        dnf install -y golang python3 python3-pip git make openssl diffutils procps-ng; \
    fi && \
    dnf clean all

COPY integration-tests/requirements.txt /tmp/requirements.txt
RUN pip3 install --no-cache-dir -r /tmp/requirements.txt && rm -f /tmp/requirements.txt

# Set up environment
ENV IS_IN_CONTAINER=true

WORKDIR /workspace/host-metering

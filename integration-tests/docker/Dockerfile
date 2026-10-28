# Integration test image: a real subscription-manager environment, selectable
# per base (UBI 8/9/10 or CentOS Stream 9/10), with the daemon under test
# built from source by the suite. This is deliberately separate from the
# .devcontainer image, which is the CentOS Stream based RPM build environment.
ARG BASE_IMAGE=registry.access.redhat.com/ubi10/ubi:latest

FROM ${BASE_IMAGE}

# The base images ship /etc/rhsm-host and /etc/pki/entitlement-host as pointers
# to host subscription secrets mounted at /run/secrets. subscription-manager
# treats those directories as proof of "container mode" and then refuses every
# command with "subscription-manager is operating in container mode", while its
# dnf plugin enables the host's RHEL subscription repos (which answer 403
# without entitlements). This image is a real rhsm test environment, not a UBI
# container inheriting the host subscription, so drop the pointers - before any
# dnf use, or the plugin still injects the host repos. Neither path is owned by
# a package; both come from the base image.
RUN rm -f /etc/rhsm-host /etc/pki/entitlement-host

# Install Go, Python 3, pip, git, openssl, and system utilities.
# subscription-manager is explicit because the suite builds the daemon from
# source, so no RPM Requires pulls it in.
# UBI 8 ships Python 3.6 only; enable the python39 module there and make it
# the default python3 (the integration test dependencies require a newer
# Python).
RUN . /etc/os-release && \
    if [ "${VERSION_ID%%.*}" = "8" ]; then \
        dnf module enable -y python39 && \
        dnf install -y golang python39 python39-pip git make openssl diffutils procps-ng subscription-manager && \
        ln -sf /usr/bin/python3.9 /usr/bin/python3 && \
        ln -sf /usr/bin/pip3.9 /usr/bin/pip3; \
    else \
        dnf install -y golang python3 python3-pip git make openssl diffutils procps-ng subscription-manager; \
    fi && \
    dnf clean all

COPY integration-tests/requirements.txt /tmp/requirements.txt
RUN pip3 install --no-cache-dir -r /tmp/requirements.txt && rm -f /tmp/requirements.txt

# subscription-manager's graceful TLS teardown waits for the server's
# close_notify, which the containerized Candlepin sends only at its keep-alive
# expiry (60s on the UBI builds). Every server-touching command would stall for
# a minute after producing output, so bound the wait the way newer builds do.
COPY integration-tests/docker/patch_rhsm_close.py /tmp/patch_rhsm_close.py
RUN python3 /tmp/patch_rhsm_close.py && rm -f /tmp/patch_rhsm_close.py

# Set up environment
ENV IS_IN_CONTAINER=true

WORKDIR /workspace/host-metering

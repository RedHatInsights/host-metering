ARG BASE_IMAGE=quay.io/centos/centos:stream10
FROM ${BASE_IMAGE}

# RPM build toolchain for `make rpm` (Go, rpmbuild, SELinux policy build).
# selinux-policy-devel, needed to build the SELinux policy module, is not
# available in the UBI repositories, hence the CentOS Stream default base;
# override with --build-arg BASE_IMAGE=... (any el-family image carrying
# these packages works, e.g. quay.io/centos/centos:stream9).
# systemd-devel: newer selinux-policy-devel macro sets inject
# "BuildRequires: pkgconfig(systemd)" (see macros.selinux-policy).
RUN dnf install -y \
        golang git make \
        rpm-build go-rpm-macros systemd-rpm-macros systemd-devel \
        selinux-policy-devel policycoreutils \
    && dnf clean all

# The repository is mounted at a fixed path and owned by the host user;
# mark it safe for git so `make rpm` (git archive) works when the container
# runs as root.
ENV IS_IN_CONTAINER=true \
    GIT_CONFIG_COUNT=1 \
    GIT_CONFIG_KEY_0=safe.directory \
    GIT_CONFIG_VALUE_0=/workspace/host-metering

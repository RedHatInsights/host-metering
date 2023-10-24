#!/usr/bin/make -f

PROJECT := host-metering
RPMNAME := host-metering
VERSION := $(shell grep "Version" version/version.go | awk -F '"' '{print $$2}')
NEXT_VERSION ?=
SHORT_COMMIT ?= $(shell git rev-parse --short=8 HEAD)
AUTORELEASE ?= "git$(shell date "+%Y%m%d%H%M")G$(SHORT_COMMIT)"

DISTDIR ?= $(CURDIR)/dist
RPMTOPDIR := $(DISTDIR)/rpmbuild

GO := go
TESTDIR := $(CURDIR)/test

DASHBOARD_URL = http://localhost:9090/graph?g0.expr=system_cpu_logical_count&g0.tab=0&g0.range_input=1m

MOCKS_DIR = $(TESTDIR)/../mocks

# Test
.PHONY: test
test: vendor
	@echo "Running the unit tests..."

	PATH=$(MOCKS_DIR):$(PATH) \
	$(GO) test -v \
	-coverprofile=coverage.out \
	-covermode=atomic \
	-coverpkg=./... \
	./...


	@echo "Calculating the coverage..."
	$(GO) tool cover -html=coverage.out -o coverage.html
	$(GO) tool cover -func=coverage.out -o coverage.txt

	@cat coverage.txt

# Build
.PHONY: build
build:
	@echo "Building the project..."
	$(GO) build -o $(DISTDIR)/$(PROJECT)

.PHONY: build-selinux
build-selinux:
	@echo "Building SELinux policy..."
	cd contrib/selinux && \
		make -f /usr/share/selinux/devel/Makefile $(PROJECT).pp || exit

# Functional testing (manual or automatic)

.PHONY: container-env-setup
container-env-setup:
ifndef IS_IN_CONTAINER
CONTAINER_TARGET = prometheus
PROMETHEUS_ADDRESS = http://localhost:9090/api/v1/write
else
CONTAINER_TARGET =
PROMETHEUS_ADDRESS = http://prometheus:9090/api/v1/write
endif

.PHONY: test-daemon
test-daemon: cert build container-env-setup $(CONTAINER_TARGET)
	@echo "Running the $(PROJECT) in deamon mode..."

	PATH=$(MOCKS_DIR):$(PATH) \
	HOST_METERING_WRITE_URL=$(PROMETHEUS_ADDRESS) \
	$(DISTDIR)/$(PROJECT) --config .devcontainer/host-metering.conf daemon

cert: mocks/consumer/cert.pem mocks/consumer/key.pem

mocks/consumer/cert.pem mocks/consumer/key.pem:
	@echo "Generating test certificates..."
	cd mocks && ./create-cert.sh

# Containers

.PHONY: podman-containers
podman-containers:
	podman-compose -f .devcontainer/docker-compose.yml build

.PHONY: prometheus
prometheus: podman-containers
	@echo "See the dashboard at: ${DASHBOARD_URL}"

	podman-compose -f .devcontainer/docker-compose.yml up -d prometheus

.PHONY: prometheus-stop
prometheus-stop:
	podman-compose -f .devcontainer/docker-compose.yml stop prometheus

.PHONY: podman-%
podman-%:
	podman-compose -f .devcontainer/docker-compose.yml run -u root host-metering bash -c "cd /workspace/host-metering && make $(subst podman-,,$@)"

.PHONY: clean-pod
clean-pod:
	podman-compose -f .devcontainer/docker-compose.yml down

# Release
.PHONY: version
version:
	@echo $(VERSION)

.PHONY: version-update
version-update:
	@test -n "$(NEXT_VERSION)" || (echo "NEXT_VERSION is not set"; exit 1)
	@echo "Updating the version to $(NEXT_VERSION)..."
	sed -i "s/Version = \".*\"/Version = \"$(NEXT_VERSION)\"/" version/version.go

.PHONY: distdir
distdir:
	@echo "Creating the destination directory..."
	mkdir -p $(DISTDIR)

# Ensure vendor was run at least once
vendor:
	$(MAKE) force-vendor

# refresh vendor cache
.PHONY: force-vendor
force-vendor:
	@echo "Downloading go dependencies..."
	$(GO) mod tidy && $(GO) mod vendor

.PHONY: tarball
tarball: distdir force-vendor
	@echo "Creating a tarball with the source code..."
	git archive \
	    --format="tar" \
	    --prefix=$(PROJECT)-$(VERSION)/ \
	    --output $(DISTDIR)/$(PROJECT)-$(VERSION).tar \
	    HEAD

	@echo "Adding go dependencies to the tarball..."
	tar --append \
	    --transform="s/^\./$(PROJECT)-$(VERSION)/" \
	    --file $(DISTDIR)/$(PROJECT)-$(VERSION).tar \
	    ./vendor

	@echo "Compressing the tarball..."
	gzip -f $(DISTDIR)/$(PROJECT)-$(VERSION).tar

# RPM build
.PHONY: rpm/spec
rpm/spec:
	sed "s/#VERSION#/${VERSION}/" contrib/rpm/host-metering.spec.in > contrib/rpm/$(RPMNAME).spec
	sed "s/#AUTORELEASE#/${AUTORELEASE}/" -i contrib/rpm/$(RPMNAME).spec

.PHONY: rpm/srpm
rpm/srpm: tarball rpm/spec
	mkdir -p $(RPMTOPDIR)/SOURCES
	cp $(DISTDIR)/$(PROJECT)-$(VERSION).tar.gz $(RPMTOPDIR)/SOURCES/
	rm -rf $(RPMTOPDIR)/SRPMS/*
	rpmbuild --define '_topdir $(RPMTOPDIR)' -bs contrib/rpm/$(RPMNAME).spec

.PHONY: rpm
rpm: rpm/srpm
	rpmbuild --define '_topdir $(RPMTOPDIR)' -bb contrib/rpm/$(RPMNAME).spec

.PHONY: rpm/mock
rpm/mock: rpm/srpm
	mkdir -p $(DISTDIR)/mock7
	mock -r epel-7-x86_64 \
	     --resultdir=$(DISTDIR)/mock7/ \
	     --rebuild $(RPMTOPDIR)/SRPMS/$(shell ls -1 $(RPMTOPDIR)/SRPMS)

.PHONY: rpm/mock-8
rpm/mock-8: rpm/srpm
	mkdir -p $(DISTDIR)/mock8
	mock -r centos-stream-8-x86_64 \
	     --resultdir=$(DISTDIR)/mock8/ \
	     --rebuild $(RPMTOPDIR)/SRPMS/$(shell ls -1 $(RPMTOPDIR)/SRPMS)

.PHONY: rpm/mock-9
rpm/mock-9: rpm/srpm
	mkdir -p $(DISTDIR)/mock9
	mock -r centos-stream-9-x86_64 \
	     --resultdir=$(DISTDIR)/mock9 \
	     --rebuild $(RPMTOPDIR)/SRPMS/$(shell ls -1 $(RPMTOPDIR)/SRPMS)

# Clean
.PHONY: clean
clean:
	@echo "Cleaning the project..."
	rm -rf $(DISTDIR)
	rm -rf $(CURDIR)/vendor
	rm -rf $(CURDIR)/coverage.out
	rm -rf $(CURDIR)/coverage.html
	rm -rf $(CURDIR)/coverage.txt
	rm -rf $(CURDIR)/$(PROJECT)
	rm -rf $(CURDIR)/contrib/selinux/tmp
	rm -rf $(CURDIR)/contrib/selinux/*.pp
	rm -rf $(MOCKS_DIR)/cpumetrics
	rm -rf $(MOCKS_DIR)/consumer

.PHONY: clean-node
clean-node:
	@echo "Cleaning the node..."
	rm -rf $(CURDIR)/node_modules

.PHONY: clean-all
clean-all: clean clean-pod clean-node

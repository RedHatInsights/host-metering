#!/usr/bin/make -f

PROJECT := host-metering
RPMNAME := host-metering
VERSION := $(shell grep "Version" version/version.go | awk -F '"' '{print $$2}')
SHORT_COMMIT ?= $(shell git rev-parse --short=8 HEAD)
AUTORELEASE ?= "git$(shell date "+%Y%m%d%H%M")G$(SHORT_COMMIT)"

DISTDIR ?= $(CURDIR)/dist
RPMTOPDIR := $(DISTDIR)/rpmbuild

GO := go
TESTDIR := $(CURDIR)/test

.PHONY: test
test:
	@echo "Running the unit tests..."

	PATH=$(TESTDIR)/bin:$(PATH) \
	$(GO) test -v \
	-coverprofile=coverage.out \
	-covermode=atomic \
	-coverpkg=./... \
	./...


	@echo "Calculating the coverage..."
	$(GO) tool cover -html=coverage.out -o coverage.html
	$(GO) tool cover -func=coverage.out -o coverage.txt

	@cat coverage.txt

# Release
.PHONY: version
version:
	@echo $(VERSION)

.PHONY: distdir
distdir:
	@echo "Creating the destination directory..."
	mkdir -p $(DISTDIR)

.PHONY: vendor
vendor:
	@echo "Downloading go dependencies..."
	$(GO) mod tidy && $(GO) mod vendor

.PHONY: tarball
tarball: distdir vendor
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

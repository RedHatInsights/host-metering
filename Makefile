#!/usr/bin/make -f

PROJECT := host-metering
VERSION := $(shell grep "Version" version/version.go | awk -F '"' '{print $$2}')

DISTDIR ?= $(CURDIR)/dist

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

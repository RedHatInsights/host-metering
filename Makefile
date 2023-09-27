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

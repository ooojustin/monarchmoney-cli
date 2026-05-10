BINARY_NAME=monarch
DIST_DIR=dist
TOOLS_DIR=$(CURDIR)/.tools
GOLANGCI_LINT_VERSION=$(shell cat .golangci-lint-version)
GOLANGCI_LINT=$(TOOLS_DIR)/bin/golangci-lint

.PHONY: all build test clean lint lint-base lint-fmt lint-vet lint-golangci fmt install-tools run-doctor snapshot-test

all: lint test build

build:
	go build -o $(DIST_DIR)/$(BINARY_NAME) ./cmd/monarch

test:
	go test -v ./...

fmt:
	gofmt -s -w $$(git ls-files '*.go')

lint: lint-base lint-golangci

lint-base: lint-fmt lint-vet

lint-fmt:
	@files="$$(gofmt -s -l $$(git ls-files '*.go'))"; \
	if [ -n "$$files" ]; then \
		echo "These files need gofmt -s:"; \
		echo "$$files"; \
		exit 1; \
	fi

lint-vet:
	go vet ./...

lint-golangci: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run ./...

install-tools: $(GOLANGCI_LINT)

$(GOLANGCI_LINT): .golangci-lint-version
	GOBIN=$(TOOLS_DIR)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

clean:
	rm -rf $(DIST_DIR)

run-doctor: build
	./$(DIST_DIR)/$(BINARY_NAME) doctor

snapshot-test:
	@echo "Snapshot testing not yet implemented"

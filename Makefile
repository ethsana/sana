GO ?= go
GOLANGCI_LINT ?= $$($(GO) env GOPATH)/bin/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.30.0
GOGOPROTOBUF ?= protoc-gen-gogofaster
GOGOPROTOBUF_VERSION ?= v1.3.1

COMMIT_HASH ?= "$(shell git describe --long --dirty --always --match "" || true)"
CLEAN_COMMIT ?= "$(shell git describe --long --always --match "" || true)"
COMMIT_TIME ?= "$(shell git show -s --format=%ct $(CLEAN_COMMIT) || true)"
LDFLAGS ?= -s -w -X github.com/ethsana/sana.commitHash="$(COMMIT_HASH)" -X github.com/ethsana/sana.commitTime="$(COMMIT_TIME)"

.PHONY: all
all: build lint vet test-race binary

.PHONY: binary
binary: dist FORCE
	$(GO) version
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/ant ./cmd/ant

dist:
	mkdir $@

.PHONY: lint
lint: linter
	$(GOLANGCI_LINT) run

.PHONY: linter
linter:
	test -f $(GOLANGCI_LINT) || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$($(GO) env GOPATH)/bin $(GOLANGCI_LINT_VERSION)

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: test-race
test-race:
	$(GO) test -race -failfast -v ./...

.PHONY: test-integration
test-integration:
	$(GO) test -tags=integration -v ./...

.PHONY: test
test:
	$(GO) test -v -failfast ./...

.PHONY: build
build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" ./...


.PHONY: protobuftools
protobuftools:
	which protoc || ( echo "install protoc for your system from https://github.com/protocolbuffers/protobuf/releases" && exit 1)
	which $(GOGOPROTOBUF) || ( cd /tmp && GO111MODULE=on $(GO) get -u github.com/gogo/protobuf/$(GOGOPROTOBUF)@$(GOGOPROTOBUF_VERSION) )

.PHONY: protobuf
protobuf: GOFLAGS=-mod=mod # use modules for protobuf file include option
protobuf: protobuftools
	$(GO) generate -run protoc ./...

.PHONY: clean
clean:
	$(GO) clean
	rm -rf dist/

FORCE:

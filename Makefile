SHELL := /bin/bash

_PWD := $(shell pwd -P)

BINARY_FILE ?= $(join $(_PWD),/build/_output/k8s-webhook-framework)

GO_SOURCES := $(find $(_PWD) -type f -name "*.go" -print)
GOOS ?= linux
GOARCH ?= amd64
GOENV ?= GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0
GOFLAGS ?= -gcflags="all=-trimpath=${GOPATH}" -asmflags="all=-trimpath=${GOPATH}"

#eg, -v
TESTOPTS ?=

.PHONY: test
test: $(GO_SOURCES)
	@go test $(TESTOPTS) $(shell go list -mod=readonly -e ./...)


.PHONY: clean
clean:
	rm -f $(BINARY_FILE)

.PHONY: serve
serve:
	@go run main.go

.PHONY: build
build: $(BINARY_FILE)
$(BINARY_FILE): $(GO_SOURCES)
	mkdir -p $(shell dirname $(BINARY_FILE))
	$(GOENV) go build $(GOFLAGS) -o $(BINARY_FILE)

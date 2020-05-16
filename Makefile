SHELL := /bin/bash

_PWD := $(shell pwd -P)

IMAGE_REGISTRY ?= quay.io
IMAGE_REPOSITORY ?= $(USER)
IMAGE_NAME ?= managed-webhooks

VERSION_MAJOR ?= 0
VERSION_MINOR ?= 1
COMMIT_NUMBER=$(shell git rev-list `git rev-list --parents HEAD | egrep "^[a-f0-9]{40}$$"`..HEAD --count)
CURRENT_COMMIT=$(shell git rev-parse --short=7 HEAD)
VERSION=$(VERSION_MAJOR).$(VERSION_MINOR).$(COMMIT_NUMBER)-$(CURRENT_COMMIT)

IMAGE := $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME)

BINARY_FILE ?= build/_output/webhooks

GO_SOURCES := $(find $(_PWD) -type f -name "*.go" -print)
EXTRA_DEPS := $(find $(_PWD)/build -type f -print)
GOOS ?= linux
GOARCH ?= amd64
GOENV=GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0
GOBUILDFLAGS=-gcflags="all=-trimpath=$(GOPATH)" -asmflags="all=-trimpath=$(GOPATH)"

#eg, -v
TESTOPTS ?=

.PHONY: test
test: vet $(GO_SOURCES)
	@go test $(TESTOPTS) $(shell go list -mod=readonly -e ./...)

.PHONY: clean
clean:
	rm -f $(BINARY_FILE)

.PHONY: serve
serve:
	@go run ./cmd/main.go -port 8888

.PHONY: vet
vet:
	gofmt -s -l $(shell go list -f '{{ .Dir }}' ./... ) | grep ".*\.go"; if [ "$$?" = "0" ]; then gofmt -s -d $(shell go list -f '{{ .Dir }}' ./... ); exit 1; fi
	go vet ./cmd/... ./pkg/...

.PHONY: build
build: $(BINARY_FILE) test
$(BINARY_FILE): $(GO_SOURCES)
	mkdir -p $(shell dirname $(BINARY_FILE))
	$(GOENV) go build $(GOBUILDFLAGS) -o $(BINARY_FILE) ./cmd

.PHONY: build-image
build-image: $(GO_SOURCES) $(EXTRA_DEPS)
	docker build -t $(IMAGE):$(VERSION) -f $(join $(_PWD),/build/Dockerfile) . && \
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest


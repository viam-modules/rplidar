BUILD_CHANNEL?=local
OS=$(shell uname)
VERSION=v1.12.0
GIT_REVISION = $(shell git rev-parse HEAD | tr -d '\n')
TAG_VERSION?=$(shell git tag --points-at | sort -Vr | head -n1)
CGO_LDFLAGS="-L 'gen/third_party/rplidar_sdk-release-${VERSION}/sdk/output/${OS}/Release/'"
GO_BUILD_LDFLAGS = -ldflags "-X 'main.Version=${TAG_VERSION}' -X 'main.GitRevision=${GIT_REVISION}'"
GOPATH = $(HOME)/go/bin
export PATH := ${GOPATH}:$(PATH) 
SHELL := /usr/bin/env bash 

default: setup build-module

setup: install-dependencies
ifneq (, $(shell which brew))
	git apply darwin_patch/swig.patch
endif

install-dependencies:
ifneq (, $(shell which brew))
	brew update
	brew install make swig pkg-config jpeg
else ifneq (, $(shell which apt-get))
	$(warning  "Installing rplidar external dependencies via APT.")
	sudo apt-get update
	sudo apt install -y make swig libjpeg-dev pkg-config
else
	$(error "Unsupported system. Only apt and brew currently supported.")
endif

goformat:
	go install golang.org/x/tools/cmd/goimports
	gofmt -s -w .
	goimports -w -local=go.viam.com/utils `go list -f '{{.Dir}}' ./... | grep -Ev "proto"`

lint: goformat
	go install github.com/edaniels/golinters/cmd/combined
	go install github.com/golangci/golangci-lint/cmd/golangci-lint
	go install github.com/polyfloyd/go-errorlint
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go vet -vettool=`go env GOPATH`/bin/combined
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs `go env GOPATH`/bin/go-errorlint -errorf
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go run github.com/golangci/golangci-lint/cmd/golangci-lint run -v --config=./etc/.golangci.yaml

sdk:
	cd gen/third_party/rplidar_sdk-release-${VERSION}/sdk && $(MAKE)

clean-sdk:
	cd gen/third_party/rplidar_sdk-release-${VERSION}/sdk && $(MAKE) clean_sdk

swig: sdk
	cd gen && swig -v -go -cgo -c++ -intgosize 64 gen.i

build-module: swig
	mkdir -p bin && CGO_LDFLAGS=${CGO_LDFLAGS} go build $(GO_BUILD_LDFLAGS) -o bin/rplidar-module module/main.go

install:
	sudo cp bin/rplidar-module /usr/local/bin/rplidar-module

test: swig
	CGO_LDFLAGS=${CGO_LDFLAGS} go test -v -coverprofile=coverage.txt -covermode=atomic ./... -race

clean: clean-sdk
	rm -rf bin gen/gen_wrap.cxx gen/gen.go

include *.make

OS=$(shell uname)
VERSION=v1.12.0
CGO_LDFLAGS="-Lgen/third_party/rplidar_sdk-release-${VERSION}/sdk/output/${OS}/Release/"
BUILD_CHANNEL?=local

all: install-swig swig
.PHONY: all

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

test:
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...

sdk:
	cd gen/third_party/rplidar_sdk-release-${VERSION}/sdk && $(MAKE)

clean-sdk:
	cd gen/third_party/rplidar_sdk-release-${VERSION}/sdk && $(MAKE) clean_sdk

install-swig:
ifeq ("Darwin", "$(shell uname -s)")
	brew install swig	
else
	sudo apt install swig -y
endif

swig: sdk
	cd gen && swig -v -go -cgo -c++ -intgosize 64 gen.i

build-module: swig
	mkdir -p bin && CGO_LDFLAGS=${CGO_LDFLAGS} go build -o bin/rplidar-module module/main.go

build-server: swig
	mkdir -p bin && CGO_LDFLAGS=${CGO_LDFLAGS} go build -o bin/rplidar_server cmd/server/main.go

clean: clean-sdk
	rm -rf bin gen/gen_wrap.cxx gen/gen.go

appimage: build-module
	cd etc/packaging/appimages && BUILD_CHANNEL=${BUILD_CHANNEL} appimage-builder --recipe rplidar-module-`uname -m`.yml
	cd etc/packaging/appimages && ./package_release.sh
	mkdir -p etc/packaging/appimages/deploy/
	mv etc/packaging/appimages/*.AppImage* etc/packaging/appimages/deploy/
	chmod a+rx etc/packaging/appimages/deploy/*.AppImage

include *.make

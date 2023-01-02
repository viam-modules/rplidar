OS=$(shell uname)
VERSION=v1.12.0
BUILD_CHANNEL?=local

all: sdk swig
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

build-sdk:
	cd gen/third_party/rplidar_sdk-release-${VERSION}/sdk && $(MAKE)

sdk:  build-sdk
	sudo cp gen/third_party/rplidar_sdk-release-${VERSION}/sdk/output/${OS}/Release/librplidar_sdk.a /usr/local/lib/
	sudo chmod 755 /usr/local/lib/librplidar_sdk.a

clean-sdk:
	cd gen/third_party/rplidar_sdk-release-${VERSION}/sdk && $(MAKE) clean_sdk

install-swig:
ifeq ("Darwin", "$(shell uname -s)")
	brew install swig	
else
	./etc/install_swig_linux.sh
endif

swig:
	cd gen && swig -v -go -cgo -c++ -intgosize 64 gen.i

build-module:
	mkdir -p bin && go build -o bin/rplidar-module module/main.go

build-server:
	mkdir -p bin && go build -o bin/rplidar-server cmd/server/main.go

clean: clean-sdk
	rm -rf bin
	rm gen/gen_wrap.cxx
	rm gen/gen.go

appimage: build-module
	cd etc/packaging/appimages && BUILD_CHANNEL=${BUILD_CHANNEL} appimage-builder --recipe rplidar-module-`uname -m`.yml
	cd etc/packaging/appimages && ./package_release.sh
	mkdir -p etc/packaging/appimages/deploy/
	mv etc/packaging/appimages/*.AppImage* etc/packaging/appimages/deploy/
	chmod 755 etc/packaging/appimages/deploy/*.AppImage

include *.make

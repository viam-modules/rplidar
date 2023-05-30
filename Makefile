OS=$(shell uname)
VERSION=v1.12.0
GIT_REVISION = $(shell git rev-parse HEAD | tr -d '\n')
TAG_VERSION?=$(shell git tag --points-at | sort -Vr | head -n1)
CGO_LDFLAGS="-L 'gen/third_party/rplidar_sdk-release-${VERSION}/sdk/output/${OS}/Release/'"
GO_BUILD_LDFLAGS = -ldflags "-X 'main.Version=${TAG_VERSION}' -X 'main.GitRevision=${GIT_REVISION}'"

.PHONY: default
default: build-module

.PHONY: goformat
goformat:
	go install golang.org/x/tools/cmd/goimports
	gofmt -s -w .
	goimports -w -local=go.viam.com/utils `go list -f '{{.Dir}}' ./... | grep -Ev "proto"`

.PHONY: lint
lint: goformat
	go install github.com/edaniels/golinters/cmd/combined
	go install github.com/golangci/golangci-lint/cmd/golangci-lint
	go install github.com/polyfloyd/go-errorlint
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go vet -vettool=`go env GOPATH`/bin/combined
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs `go env GOPATH`/bin/go-errorlint -errorf
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go run github.com/golangci/golangci-lint/cmd/golangci-lint run -v --config=./etc/.golangci.yaml

.PHONY: test
test:
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: sdk
sdk:
	cd gen/third_party/rplidar_sdk-release-${VERSION}/sdk && $(MAKE)

.PHONY: clean-sdk
clean-sdk:
	cd gen/third_party/rplidar_sdk-release-${VERSION}/sdk && $(MAKE) clean_sdk

.PHONY: swig
swig: sdk
	cd gen && swig -v -go -cgo -c++ -intgosize 64 gen.i

.PHONY: build-module
build-module: swig
	mkdir -p bin && CGO_LDFLAGS=${CGO_LDFLAGS} go build $(GO_BUILD_LDFLAGS) -o bin/rplidar-module module/main.go

.PHONY: clean
clean: clean-sdk
	rm -rf bin gen/gen_wrap.cxx gen/gen.go

.PHONY: appimage
appimage: build-module
	cd etc/packaging/appimages && appimage-builder --recipe rplidar-module-`uname -m`.yml
	mkdir -p etc/packaging/appimages/deploy/
	mv etc/packaging/appimages/*.AppImage* etc/packaging/appimages/deploy/
	chmod a+rx etc/packaging/appimages/deploy/*.AppImage

.PHONY: clean-appimage
clean-appimage:
	rm -rf etc/packaging/appimages/AppDir && rm -rf etc/packaging/appimages/appimage-build && rm -rf etc/packaging/appimages/deploy

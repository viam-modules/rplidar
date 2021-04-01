OS=$(shell uname)
VERSION=v1.12.0

all: sdk swig
.PHONY: all

goformat:
	gofmt -s -w .

lint: goformat
	go install github.com/edaniels/golinters/cmd/combined
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go vet -vettool=`go env GOPATH`/bin/combined
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go run github.com/golangci/golangci-lint/cmd/golangci-lint run -v

test:
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...

sdk:
	cd gen/third_party/rplidar_sdk-release-${VERSION}/sdk && $(MAKE)
	sudo cp gen/third_party/rplidar_sdk-release-${VERSION}/sdk/output/${OS}/Release/librplidar_sdk.a /usr/local/lib/
	sudo chmod 755 /usr/local/lib/librplidar_sdk.a

swig:
	cd gen && swig -v -go -cgo -c++ -intgosize 64 gen.i

clean:
	cd gen/third_party/rplidar_sdk-release-${VERSION}/sdk && $(MAKE) clean_sdk

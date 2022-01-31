
## Dependencies for golang

The below will only work for Viam, Inc. employees right now. The C++ code is independent however.

Make sure the following is in your shell configuration:
```
export GOPRIVATE=github.com/viamrobotics/*,go.viam.com/*
```

Also run 
```
git config --global url.ssh://git@github.com/.insteadOf https://github.com/
```

_TODO: Needs an update of dependencies to latest version and refactoring._

# Getting started
NOTE: Doesn't work on osx, rplidar will not be recognized as a usb device.

Currently only tested on an RPI:
1. `make`
2. Make sure you run the commands above
3. Create a folder to save data in form of pcd files in: `mkdir data`
4. `go run -tags=pi cmd/server/main.go `
5. Save data or view the output:
    * Either view the output in the browser (e.g. <YOUR_IP_ADDRESS>:8080), or
    * Run the client in a separate terminal: `go run -tags=pi cmd/client/main.go`
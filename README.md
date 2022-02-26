
# rplidar
The below will only work for Viam, Inc. employees right now. The C++ code is independent however.

## Getting started

1. `make`
2. Dependencies for golang
    * Make sure the following is in your shell configuration:
        * `export GOPRIVATE=github.com/viamrobotics/*,go.viam.com/*`
    * `git config --global url.ssh://git@github.com/.insteadOf https://github.com/`
3. There are two options: Run a server/client, or a script that saves PCD files into a directory. See instructions for RPI/Debian and OSX below:

**RPI (Debian)**
* Server/Client: `go run cmd/server/main.go`
    * Either view the output in the browser (e.g. <YOUR_IP_ADDRESS>:8081), or
    * Run the client in a separate terminal: `go run cmd/client/main.go`
* Script that saves PCD files: `go run cmd/savepcdfiles/main.go`

**OSX**

1. Find the device path name by following [these instructions](https://stackoverflow.com/questions/48291366/how-to-find-dev-name-of-usb-device-for-serial-reading-on-mac-os), further denoted as `YOUR_RPLIDAR_PATH`
    * NOTE: It will likely be this path: `/dev/tty.SLAB_USBtoUART`
2. Run the server/client, or a script that saves PCD files into a directory:
    * Server/Client: `go run cmd/server/main.go -device YOUR_RPLIDAR_PATH`
        * Either view the output in the browser (e.g. <YOUR_IP_ADDRESS>:8081), or
        * Run the client in a separate terminal: `go run cmd/client/main.go`
    * Script that saves PCD files: `go run cmd/savepcdfiles/main.go -device YOUR_RPLIDAR_PATH`

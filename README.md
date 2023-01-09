
# rplidar
This repo integrates slamtec rplidar LIDARS within Viam's [RDK](https://github.com/viamrobotics/rdk).

It has been tested on the following rplidars:
* [RPLIDAR A1](https://www.slamtec.com/en/Lidar/A1)
* [RPLIDAR A3](https://www.slamtec.com/en/Lidar/A3)


## Getting started

1. Install the rplidar module:
   * OSx: 
      ```bash
      brew tap viamrobotics/brews && brew install rplidar-module
      ```
   * Linux AArch64 (ARM64) (e.g., on an RPI):
      ```bash
      sudo curl -o /usr/local/bin/rplidar-module http://packages.viam.com/apps/rplidar/rplidar-module-latest-aarch64.AppImage
      sudo chmod a+rx /usr/local/bin/rplidar-module
      ```
   * Linux x86_64:
      ```bash
      sudo curl -o /usr/local/bin/rplidar-module http://packages.viam.com/apps/rplidar/rplidar-module-latest-x86_64.AppImage
      sudo chmod a+rx /usr/local/bin/rplidar-module
      ```
2. Run the [RDK](https://github.com/viamrobotics/rdk) web server using one of the example config files [modules/sample_osx.json](./module/sample_osx.json) or [modules/sample_linux.json](./module/sample_linux.json), depending on your operating system. 

## Development

Run `make install-swig`.

### Run rplidar as a modular component

1. Build the module: `make build-module`
2. Run the [RDK](https://github.com/viamrobotics/rdk) web server using one of the example config files [modules/sample_osx.json](./module/sample_osx.json) or [modules/sample_linux.json](./module/sample_linux.json), depending on your operating system.

### Run rplidar as a standalone server/client

**RPI (Debian)**

* Server: `go run cmd/server/main.go`
* Client: `go run cmd/client/main.go`
* Script that saves PCD files: `go run cmd/savepcdfiles/main.go -datafolder my_data`

**macOS**

1. Find the device path name by following [these instructions](https://stackoverflow.com/questions/48291366/how-to-find-dev-name-of-usb-device-for-serial-reading-on-mac-os)
    * NOTE: It will likely be this path: `/dev/tty.SLAB_USBtoUART`
2. Server: `go run cmd/server/main.go -device /dev/tty.SLAB_USBtoUART`
3. Client: `go run cmd/client/main.go -device /dev/tty.SLAB_USBtoUART`
4. Script that saves PCD files: `go run cmd/savepcdfiles/main.go -device /dev/tty.SLAB_USBtoUART -datafolder my_data`

### Linting

```bash
make lint
```

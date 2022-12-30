
# rplidar
This repo integrates slamtec rplidar LIDARS within Viam's [RDK](https://github.com/viamrobotics/rdk).

It has been tested on the following rplidars:
* [RPLIDAR A1](https://www.slamtec.com/en/Lidar/A1)
* [RPLIDAR A3](https://www.slamtec.com/en/Lidar/A3)


## Installation
1. Install swig:
   * On MacOS:
      * `brew install swig`
   * On Linux:
        ```bash
        # Install g++
        sudo apt-get install g++
        # Install pcre
        sudo apt-get install libpcre3 libpcre3-dev libpcre2-dev
        # Download swig (source: http://www.swig.org/download.html)
        wget http://prdownloads.sourceforge.net/swig/swig-4.1.1.tar.gz
        # Unzip file & cd into directory
        chmod 777 swig-4.1.1.tar.gz && tar -xzvf swig-4.1.1.tar.gz
        cd swig-4.1.1
        # Specify swig install directory, e.g.:
        ./configure --prefix=/home/testbot/swigtool
        # Compile and install
        sudo make
        sudo make install
        # Add SWIG_PATH environment variable and add it in PATH
        export SWIG_PATH=/home/testbot/swigtool/bin
        export PATH=$SWIG_PATH:$PATH
        # Check if it is installed successfully
        swig -version
        ```
2. `make`
3. Dependencies for golang:
   * `export GOPRIVATE=github.com/viamrobotics/*,go.viam.com/*`
   * `git config --global url.ssh://git@github.com/.insteadOf https://github.com/`

## Getting started

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

## Development
### Linting

```bash
make lint
```

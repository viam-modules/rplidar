
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

### Linting

```bash
make lint
```

### (Optional) Using Canon Images

If desired, Viam's canon tool can be used to create a docker container to build `arm64` or `amd64` binaries of the SLAM server. The canon tool can be installed by running the following command: 

```bash
go install github.com/viamrobotics/canon@latest
```

And then by running one of the following commands in the rplidar repository to create the container:

```bash
canon -arch arm64
```

```bash
canon -arch amd64
```

These containers are set to persist between sessions via the `persistent` parameter in the `.canon.yaml` file located in the root of rplidar. More details regarding the use of Viam's canon tool can be found [here](https://github.com/viamrobotics/canon).


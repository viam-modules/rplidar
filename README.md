
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

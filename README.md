
# Rplidar Camera
This repo integrates slamtec rplidar LIDARS to be configured as a [camera](https://docs.viam.com/components/camera/) with Viam's [RDK](https://github.com/viamrobotics/rdk).

It has been tested on the following rplidars:
* [RPLIDAR A1](https://www.slamtec.com/en/Lidar/A1)
* [RPLIDAR A3](https://www.slamtec.com/en/Lidar/A3)
* [RPLIDAR S1](http://bucket.download.slamtec.com/f19ea8efcc2bb55dbfd5839f1d307e34aa4a6ca0/LD601_SLAMTEC_rplidar_datasheet_S1_v1.4_en.pdf)

## Build and Run

To use this module, follow these instructions to [add a module from the Viam Registry](https://docs.viam.com/modular-resources/configure/#add-a-module-from-the-viam-registry) and select the `viam:lidar:rplidar` model from the [`rplidar` module](https://app.viam.com/module/viam/rplidar).

### Requirements 

The `rplidar` module is distributed as an AppImage.
AppImages require FUSE version 2 to run.
See [FUSE troubleshooting](https://docs.viam.com/appendix/troubleshooting/#appimages-require-fuse-to-run) for instructions on installing FUSE 2 on your system if it is not already installed.

Currently, the `rplidar` module supports the Linux platform only.

## Configure your Rplidar

> [!NOTE]  
> Before configuring your camera, you must [create a robot](https://docs.viam.com/manage/fleet/robots/#add-a-new-robot).

Navigate to the **Config** tab of your robot’s page in [the Viam app](https://app.viam.com/). Click on the **Components** subtab and click **Create component**. Select the `camera` type, then select the `lidar:rplidar` model. Enter a name for your camera and click **Create**.

> [!NOTE]  
> For more information, see [Configure a Robot](https://docs.viam.com/manage/configuration/).

To save your changes, click **Save config** at the bottom of the page.
Check the **Logs** tab of your robot in the Viam app to make sure your RPlidar has connected and no errors are being raised.

### Attributes

No attributes are available for a `lidar:rplidar` camera.

## Build and Run locally

If you don't want to load the model from the registry, for example because you are actively changing its functionality, you can install it locally. Follow these instructions to [configure a local module on your machine](https://docs.viam.com/registry/configure/#edit-the-configuration-of-a-local-module).

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
2. Run the [RDK](https://github.com/viamrobotics/rdk) web server using one of the example config files [module/sample_osx_m1.json](./module/sample_osx_m1.json), [module/sample_osx_x86.json](./module/sample_osx_x86.json) or [module/sample_linux.json](./module/sample_linux.json), depending on your operating system and processor. 

## Development

### Dependencies

Install the following list of dependencies:

* [Golang](https://go.dev/doc/install)
* `make`, `swig`, `jpeg`, `pkg-config`:
    * MacOS: `brew install make swig pkg-config jpeg`
    * Linux: `apt update && apt install -y make swig libjpeg-dev pkg-config`

### Build & run rplidar as a modular component

1. Build the module: `make build-module`
2. Run the [RDK](https://github.com/viamrobotics/rdk) web server using one of the example config files:
    * MacOS: [modules/sample_osx.json](./module/sample_osx.json)
    * Linux: [modules/sample_linux.json](./module/sample_linux.json)

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


// Package rplidar implements a general rplidar LIDAR as a camera.
package rplidar

import (
	"errors"
	"fmt"

	"go.viam.com/rplidar/gen"
)

type rplidarDevice struct {
	driver           gen.RPlidarDriver
	model            byte
	serialNumber     string
	firmwareVersion  string
	hardwareRevision int
}

func getRplidarDevice(devicePath string) (rplidarDevice, error) {
	var driver gen.RPlidarDriver
	devInfo := gen.NewRplidar_response_device_info_t()
	defer gen.DeleteRplidar_response_device_info_t(devInfo)

	var connectErr error
	for _, rate := range []uint{256000, 115200} {
		possibleDriver := gen.RPlidarDriverCreateDriver(uint(gen.DRIVER_TYPE_SERIALPORT))
		if result := possibleDriver.Connect(devicePath, rate); Result(result) != ResultOk {
			r := Result(result)
			if r == ResultOpTimeout {
				continue
			}
			connectErr = fmt.Errorf("failed to connect: %w, try checking your defined device_path", Result(result).Failed())
			continue
		}

		if result := possibleDriver.GetDeviceInfo(devInfo, defaultTimeout); Result(result) != ResultOk {
			r := Result(result)
			if r == ResultOpTimeout {
				continue
			}
			connectErr = fmt.Errorf("failed to get device info: %w", Result(result).Failed())
			continue
		}
		driver = possibleDriver
		break
	}
	if driver == nil {
		if connectErr == nil {
			return rplidarDevice{}, fmt.Errorf("timed out connecting to %q", devicePath)
		}
		return rplidarDevice{}, connectErr
	}

	serialNum := devInfo.GetSerialnum()
	var serialNumStr string
	for pos := 0; pos < 16; pos++ {
		serialNumStr += fmt.Sprintf("%02X", gen.ByteArray_getitem(serialNum, pos))
	}

	firmwareVer := fmt.Sprintf("%d.%02d",
		devInfo.GetFirmware_version()>>8,
		devInfo.GetFirmware_version()&0xFF)
	hardwareRev := int(devInfo.GetHardware_version())

	healthInfo := gen.NewRplidar_response_device_health_t()
	defer gen.DeleteRplidar_response_device_health_t(healthInfo)

	if result := driver.GetHealth(healthInfo, defaultTimeout); Result(result) != ResultOk {
		gen.RPlidarDriverDisposeDriver(driver)
		driver = nil
		return rplidarDevice{}, fmt.Errorf("failed to get health: %w", Result(result).Failed())
	}

	if int(healthInfo.GetStatus()) == gen.RPLIDAR_STATUS_ERROR {
		gen.RPlidarDriverDisposeDriver(driver)
		driver = nil
		return rplidarDevice{}, errors.New("bad health")
	}

	rplidarDevice := &rplidarDevice{
		driver:           driver,
		model:            devInfo.GetModel(),
		serialNumber:     serialNumStr,
		firmwareVersion:  firmwareVer,
		hardwareRevision: hardwareRev,
	}

	return *rplidarDevice, nil
}

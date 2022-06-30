package helper

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rplidar"
	"go.viam.com/utils"
	"go.viam.com/utils/usb"
)

func GetPort(port utils.NetPortFlag, defaultPort utils.NetPortFlag, logger golog.Logger) utils.NetPortFlag {
	if port == 0 {
		logger.Debugf("using default port %d ", defaultPort)
		return defaultPort
	} else {
		logger.Debugf("using user defined port %d ", port)
	}
	return port
}

func GetTimeDeltaMilliseconds(scanTimeDelta, defaultTimeDeltaMilliseconds int, logger golog.Logger) int {
	// Based on empirical data, we can see that the rplidar collects data at a rate of 15Hz,
	// which is ~ 66ms per scan. This issues a warning to the user, in case they're expecting
	// to receive data at a higher rate than what is technically possible.
	if scanTimeDelta == 0 {
		logger.Debugf("using default time delta %d ", defaultTimeDeltaMilliseconds)
		return defaultTimeDeltaMilliseconds
	} else {
		logger.Debugf("using user defined time delta %d ", scanTimeDelta)
	}

	var estimatedTimePerScan int = 66
	if scanTimeDelta < estimatedTimePerScan {
		logger.Warnf("the expected scan rate of deltaT=%v is too small, has to be at least %v", scanTimeDelta, estimatedTimePerScan)
	}
	return scanTimeDelta
}

func getDevicePath(devicePath string, logger golog.Logger) (string, error) {
	if devicePath == "" {
		usbDevices := usb.Search(
			usb.SearchFilter{},
			func(vendorID, productID int) bool {
				return vendorID == rplidar.USBInfo.Vendor && productID == rplidar.USBInfo.Product
			})

		if len(usbDevices) != 0 {
			logger.Debugf("detected %d lidar devices", len(usbDevices))
			for _, comp := range usbDevices {
				logger.Debug(comp)
			}
			return usbDevices[0].Path, nil
		} else {
			return "", errors.New("no usb devices found")
		}
	}
	return devicePath, nil
}

func getDataFolder(dataFolder string, defaultDataFolder string, logger golog.Logger) (string, error) {
	if dataFolder == "" {
		logger.Debugf("using default data folder '%s' ", defaultDataFolder)
		dataFolder = defaultDataFolder
	} else {
		logger.Debugf("using user defined data folder %s", dataFolder)
	}

	if err := os.MkdirAll(filepath.Join(".", dataFolder), os.ModePerm); err != nil {
		return "", errors.New("can not create a new directory named: " + dataFolder)
	}
	return dataFolder, nil
}

func CreateRplidarComponent(name, model, devicePath, dataFolder, defaultDataFolder string, cameraType resource.SubtypeName, logger golog.Logger) (config.Component, error) {
	devicePath, err := getDevicePath(devicePath, logger)
	if err != nil {
		return config.Component{}, err
	}
	dataFolder, err = getDataFolder(dataFolder, defaultDataFolder, logger)
	if err != nil {
		return config.Component{}, err
	}

	lidarDevice := config.Component{
		Namespace: "rdk",
		Name:      name,
		Type:      cameraType,
		Model:     model,
		Attributes: config.AttributeMap{
			"device_path": devicePath,
			"data_folder": dataFolder,
		},
	}
	return lidarDevice, nil
}

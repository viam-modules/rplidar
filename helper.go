package rplidar

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"

	"go.viam.com/utils/usb"
)

var usbInfo = &usb.Identifier{
	Vendor:  0x10c4,
	Product: 0xea60,
}

func getDevicePath(devicePath string, logger golog.Logger) (string, error) {
	if devicePath == "" {
		usbDevices := usb.Search(
			usb.SearchFilter{},
			func(vendorID, productID int) bool {
				return vendorID == usbInfo.Vendor && productID == usbInfo.Product
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

package rplidar

import (
	"errors"

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

		if len(usbDevices) == 0 {
			return "", errors.New("no usb devices found")
		}

		logger.Debugf("detected %d lidar devices", len(usbDevices))
		for _, comp := range usbDevices {
			logger.Debug(comp)
		}
		return usbDevices[0].Path, nil
	}
	return devicePath, nil
}

func CreateRplidarComponent(name, model, devicePath string, cameraType resource.SubtypeName, logger golog.Logger) (config.Component, error) {
	devicePath, err := getDevicePath(devicePath, logger)
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
		},
	}
	return lidarDevice, nil
}

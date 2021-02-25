package rplidar

import "github.com/viamrobotics/robotcore/lidar"

const ModelName = "rplidar"
const DeviceType = lidar.DeviceType("RPLidar")

type DeviceInfo struct {
	Model            string `json:"model"`
	SerialNumber     string `json:"serial_number"`
	FirmwareVersion  string `json:"firmware_version"`
	HardwareRevision int    `json:"hardware_revision"`
}

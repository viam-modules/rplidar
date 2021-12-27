package rplidar

import "go.viam.com/rdk/component/camera"

const (
	// ModelName is how the lidar will be registered into core.
	ModelName = "rplidar"

	// Type is the lidar specific type.
	Type = camera.SubtypeName
)

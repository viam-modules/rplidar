package rplidar

import "go.viam.com/core/lidar"

const (
	// ModelName is how the lidar will be registered into core.
	ModelName = "rplidar"

	// Type is the lidar specific type.
	Type = lidar.Type(ModelName)
)

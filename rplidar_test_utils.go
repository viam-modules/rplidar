//nolint:golint
package rplidar

import (
	"go.viam.com/rplidar/inject"

	"go.viam.com/rplidar/gen"
)

// GoodRplidarReturnsZeroPoints creates a Rplidar that returns only zero data
func GoodRplidarReturnsZeroPoints() *rplidar {

	// Create injected rplidar driver
	injectedRPlidarDriver := inject.RPlidarDriver{}

	injectedRPlidarDriver.GrabScanDataHqFunc = func(a ...interface{}) uint {
		return uint(gen.RESULT_OK)
	}

	injectedRPlidarDriver.AscendScanDataFunc = func(a ...interface{}) uint {
		return 0
	}

	injectedRplidarDevice := rplidarDevice{
		driver: &injectedRPlidarDriver,
	}

	// Create injected node
	injectedNode := inject.Nodes{}

	injectedNode.GetDist_mm_q2Func = func() uint {
		return 0
	}

	injectedNode.GetAngle_z_q14Func = func() uint16 {
		return 0
	}

	rp := &rplidar{
		device:  &injectedRplidarDevice,
		nodes:   &injectedNode,
		testing: true,
	}

	return rp
}

// BadRplidarFailsToGrabScanData returns an Rplidar that fails when grabbing scan data
func BadRplidarFailsToGrabScanData() *rplidar {
	// Create injected rplidar driver
	injectedRPlidarDriver := inject.RPlidarDriver{}

	injectedRPlidarDriver.GrabScanDataHqFunc = func(a ...interface{}) uint {
		return uint(gen.RESULT_OPERATION_FAIL)
	}
	injectedRPlidarDriver.AscendScanDataFunc = func(a ...interface{}) uint {
		return 0
	}

	injectedRplidarDevice := rplidarDevice{
		driver: &injectedRPlidarDriver,
	}

	// Create injected node
	injectedNode := inject.Nodes{}

	rp := &rplidar{
		device:  &injectedRplidarDevice,
		nodes:   &injectedNode,
		testing: true,
	}

	return rp
}

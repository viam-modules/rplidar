package rplidar

import (
	"sync"

	"go.viam.com/rplidar/inject"

	"go.viam.com/rplidar/gen"
)

func GoodRplidarReturnsZeroPoints() *Rplidar {

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

	// Create injected node for extracting angle and distance
	injectedNode := inject.Nodes{}

	injectedNode.GetDist_mm_q2Func = func() uint {
		return 0
	}

	injectedNode.GetAngle_z_q14Func = func() uint16 {
		return 0
	}

	rp := &Rplidar{
		device:      injectedRplidarDevice,
		nodes:       &injectedNode,
		deviceMutex: sync.Mutex{},
		testing:     true,
	}

	return rp
}

func BadRplidarFailsToGrabScanData() *Rplidar {
	// Create injected rplidar driver that fails when grabbing scan data
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

	// Create injected node for extracting angle and distance
	injectedNode := inject.Nodes{}

	rp := &Rplidar{
		device:      injectedRplidarDevice,
		nodes:       &injectedNode,
		deviceMutex: sync.Mutex{},
		testing:     true,
	}

	return rp
}

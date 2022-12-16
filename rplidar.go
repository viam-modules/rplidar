// Package rplidar implements a general rplidar LIDAR as a camera.
package rplidar

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rplidar/gen"

	goutils "go.viam.com/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

const (
	model          = "rplidar"
	defaultTimeout = uint(1000)
	// DefaultPort is the default port for the rplidar.
	DefaultPort = 4444
)

func init() {
	registry.RegisterComponent(camera.Subtype, model, registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			port := config.Attributes.Int("port", DefaultPort)
			devicePath := config.Attributes.String("device_path")
			if devicePath == "" {
				return nil, errors.New("need to specify a devicePath (ex. /dev/ttyUSB0)")
			}
			return NewRPLidar(logger, port, devicePath)
		}})
}

// NewRPLidar returns a new RPLidar device at the given path.
func NewRPLidar(logger golog.Logger, port int, devicePath string) (camera.Camera, error) {
	rplidarDevice, err := getRplidarDevice(devicePath)
	if err != nil {
		return nil, err
	}

	rp := &RPLidar{
		device:                  rplidarDevice,
		nodeSize:                8192,
		logger:                  logger,
		defaultNumScans:         1,
		warmupNumDiscardedScans: 5,
	}
	rp.Start()
	return rp, nil
}

// RPLidar controls an RPLidar device.
type RPLidar struct {
	generic.Unimplemented
	mu                      sync.Mutex
	device                  rplidarDevice
	nodes                   gen.Rplidar_response_measurement_node_hq_t
	nodeSize                int
	started                 bool
	scannedOnce             bool
	defaultNumScans         int
	warmupNumDiscardedScans int

	logger golog.Logger
}

// Start requests that the rplidar starts up and starts spinning.
func (rp *RPLidar) Start() {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	rp.started = true
	rp.logger.Debug("starting motor")
	rp.device.driver.StartMotor()
	rp.device.driver.StartScan(false, true)
	rp.nodes = gen.New_measurementNodeHqArray(rp.nodeSize)
}

// Stop request that the rplidar stops spinning.
func (rp *RPLidar) Stop() {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.nodes != nil {
		defer func() {
			gen.Delete_measurementNodeHqArray(rp.nodes)
			rp.nodes = nil
		}()
	}
	rp.logger.Debug("stopping motor")
	rp.device.driver.Stop()
	rp.device.driver.StopMotor()
	rp.started = false
}

// NextPointCloud performs a scan on the rplidar and performs some filtering to clean up the data.
func (rp *RPLidar) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	pc, err := rp.getPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	return pc, nil
}

func (rp *RPLidar) scan(ctx context.Context, numScans int) (pointcloud.PointCloud, error) {
	pc := pointcloud.New()
	nodeCount := int64(rp.nodeSize)

	var dropCount int
	for i := 0; i < numScans; i++ {
		nodeCount = int64(rp.nodeSize)
		result := rp.device.driver.GrabScanDataHq(rp.nodes, &nodeCount, defaultTimeout)
		if Result(result) != ResultOk {
			return nil, fmt.Errorf("bad scan: %w", Result(result).Failed())
		}
		rp.device.driver.AscendScanData(rp.nodes, nodeCount)

		for pos := 0; pos < int(nodeCount); pos++ {
			node := gen.MeasurementNodeHqArray_getitem(rp.nodes, pos)
			if node.GetDist_mm_q2() == 0 {
				dropCount++
				continue // TODO(erd): okay to skip?
			}

			nodeAngle := (float64(node.GetAngle_z_q14()) * 90 / (1 << 14))
			nodeDistance := float64(node.GetDist_mm_q2()) / 4

			err := pc.Set(pointFrom(utils.DegToRad(nodeAngle), utils.DegToRad(0), nodeDistance/1000, 255))
			if err != nil {
				return nil, err
			}
		}
	}
	if pc.Size() == 0 {
		return nil, nil
	}
	return pc, nil
}

func (rp *RPLidar) getPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if !rp.started {
		rp.Start()
	}

	// wait and then discard scans for warmup
	if !rp.scannedOnce {
		rp.scannedOnce = true
		goutils.SelectContextOrWait(ctx, time.Duration(rp.warmupNumDiscardedScans)*time.Second)
		if _, err := rp.scan(ctx, rp.warmupNumDiscardedScans); err != nil {
			return nil, err
		}
	}

	pc, err := rp.scan(ctx, rp.defaultNumScans)
	if err != nil {
		return nil, err
	}
	return pc, nil
}

// Properties is a part of the Camera interface but is not implemented for the rplidar.
func (rp *RPLidar) Properties(ctx context.Context) (camera.Properties, error) {
	var props camera.Properties
	return props, utils.NewUnimplementedInterfaceError("Properties", nil)
}

// Projector is a part of the Camera interface but is not implemented for the rplidar.
func (rp *RPLidar) Projector(ctx context.Context) (transform.Projector, error) {
	var proj transform.Projector
	return proj, utils.NewUnimplementedInterfaceError("Projector", nil)
}

// Stream is a part of the Camera interface but is not implemented for the rplidar.
func (rp *RPLidar) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	var stream gostream.VideoStream
	return stream, utils.NewUnimplementedInterfaceError("Stream", nil)
}

// Close stops the rplidar and disposes of the driver.
func (rp *RPLidar) Close(ctx context.Context) error {
	if rp.device.driver != nil {
		rp.Stop()
		gen.RPlidarDriverDisposeDriver(rp.device.driver)
		rp.device.driver = nil
	}
	return nil
}

func pointFrom(yaw, pitch, distance float64, reflectivity uint8) (r3.Vector, pointcloud.Data) {
	ea := spatialmath.NewEulerAngles()
	ea.Yaw = yaw
	ea.Pitch = pitch

	pose1 := spatialmath.NewPoseFromOrientation(r3.Vector{0, 0, 0}, ea)
	pose2 := spatialmath.NewPoseFromPoint(r3.Vector{distance, 0, 0})
	p := spatialmath.Compose(pose1, pose2).Point()

	pos := pointcloud.NewVector(p.X*1000, p.Y*1000, p.Z*1000)
	d := pointcloud.NewBasicData()
	d.SetIntensity(uint16(reflectivity) * 255)

	return pos, d
}

// Package rplidar implements a general rplidar LIDAR as a camera.
package rplidar

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
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
	// ModelName is how the lidar will be registered into rdk.
	ModelName      = "rplidar"
	defaultTimeout = uint(1000)
	DefaultPort    = 4444
)

func init() {
	registry.RegisterComponent(camera.Subtype, ModelName, registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			port := config.Attributes.Int("port", DefaultPort)
			devicePath := config.Attributes.String("device_path")
			if devicePath == "" {
				return nil, errors.New("need to specify a devicePath (ex. /dev/ttyUSB0)")
			}
			dataFolder := config.Attributes.String("data_folder")
			if dataFolder == "" {
				return nil, errors.New("need to set 'data_folder' to a valid storage location")
			}
			return NewRPLidar(logger, port, devicePath, dataFolder)
		}})
}

// NewRPLidar returns a new RPLidar device at the given path.
func NewRPLidar(logger golog.Logger, port int, devicePath string, dataFolder string) (camera.Camera, error) {
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
		dataFolder:              dataFolder,
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
	dataFolder              string

	logger golog.Logger
}

// Start requests that the rplidar starts up and starts spinning.
func (rp *RPLidar) Start() {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	rp.started = true
	rp.logger.Debugf("starting motor")
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
	rp.logger.Debugf("stopping motor")
	rp.device.driver.Stop()
	rp.device.driver.StopMotor()
	rp.started = false
}

// NextPointCloud performs a scan on the rplidar and performs some filtering to clean up the data.
// It also saves the pointcloud in form of a pcd file.
func (rp *RPLidar) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	pc, timeStamp, err := rp.getPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	if err = rp.savePCDFile(timeStamp, pc); err != nil {
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

			err := pc.Set(pointFrom(utils.DegToRad(nodeAngle), utils.DegToRad(0), float64(nodeDistance)/1000, 255))
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

func (rp *RPLidar) savePCDFile(timeStamp time.Time, pc pointcloud.PointCloud) error {
	f, err := os.Create(rp.dataFolder + "/rplidar_data_" + timeStamp.UTC().Format(time.RFC3339Nano) + ".pcd")
	if err != nil {
		return err
	}

	w := bufio.NewWriter(f)
	if err = pointcloud.ToPCD(pc, w, pointcloud.PCDBinary); err != nil {
		return err
	}
	if err = w.Flush(); err != nil {
		return err
	}
	return f.Close()
}

func (rp *RPLidar) getPointCloud(ctx context.Context) (pointcloud.PointCloud, time.Time, error) {
	if !rp.started {
		rp.Start()
	}

	// wait and then discard scans for warmup
	if !rp.scannedOnce {
		rp.scannedOnce = true
		goutils.SelectContextOrWait(ctx, time.Duration(rp.warmupNumDiscardedScans)*time.Second)
		if _, err := rp.scan(ctx, rp.warmupNumDiscardedScans); err != nil {
			return nil, time.Now(), err
		}
	}

	pc, err := rp.scan(ctx, rp.defaultNumScans)
	if err != nil {
		return nil, time.Now(), err
	}
	return pc, time.Now(), nil
}

func (rp *RPLidar) Properties(ctx context.Context) (camera.Properties, error) {
	var props camera.Properties
	return props, utils.NewUnimplementedInterfaceError("Properties", nil)
}

func (rp *RPLidar) Projector(ctx context.Context) (transform.Projector, error) {
	var proj transform.Projector
	return proj, utils.NewUnimplementedInterfaceError("Projector", nil)
}

func (rp *RPLidar) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	var stream gostream.VideoStream
	return stream, utils.NewUnimplementedInterfaceError("Stream", nil)
}

func (rp *RPLidar) GetFrame(ctx context.Context, mimeType string) ([]byte, string, int64, int64, error) {
	return nil, "", -1, -1, utils.NewUnimplementedInterfaceError("GetFrame", nil)
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

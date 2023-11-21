// Package rplidar implements a general rplidar LIDAR as a camera.
package rplidar

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rplidar/gen"

	goutils "go.viam.com/utils"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

const (
	// The max time in milliseconds it should take for the rplidar to get scan data.
	defaultTimeoutMs = uint(1000)
	// The number of full 360 scans to complete before returning a point cloud.
	defaultNumScans = 1
	// The number of scans to discard at startup to ensure valid data is returned to the user.
	defaultWarmupNumDiscardedScans = 5
	// The number of max data points returned in each scan
	defaultNodeSize = 8192
)

var (
	// Model is the model of the rplidar
	Model = resource.NewModel("viam", "lidar", "rplidar")
	// rplidarModelByteMap maps the byte model representation to a string representation
	rplidarModelByteMap = map[byte]string{24: "A1", 49: "A3", 97: "S1"}
)

// Rplidar controls an Rplidar device.
type Rplidar struct {
	resource.Named
	resource.AlwaysRebuild

	deviceMutex *sync.Mutex
	device      rplidarDevice
	nodes       gen.Rplidar_response_measurement_node_hq_t
	minRangeMM  float64

	cancelFunc       func()
	cacheMutex       *sync.RWMutex
	cachedPointCloud pointcloud.PointCloud

	cacheBackgroundWorkers sync.WaitGroup
	logger                 logging.Logger
}

// Config describes how to configure the RPlidar component.
type Config struct {
	DevicePath string  `json:"device_path"`
	MinRangeMM float64 `json:"min_range_mm"`
}

// Validate checks that the config attributes are valid for an RPlidar.
func (conf *Config) Validate(path string) ([]string, error) {

	if conf.MinRangeMM < 0 {
		return nil, errors.New("min_range must be positive")
	}

	return nil, nil
}

func init() {
	resource.RegisterComponent(camera.API, Model, resource.Registration[camera.Camera, *Config]{Constructor: newRplidar})
}

func newRplidar(ctx context.Context, _ resource.Dependencies, c resource.Config, logger logging.Logger) (camera.Camera, error) {
	svcConf, err := resource.NativeConfig[*Config](c)
	if err != nil {
		return nil, err
	}

	devicePath := svcConf.DevicePath
	if devicePath == "" {
		var err error
		if devicePath, err = searchForDevicePath(logger); err != nil {
			return nil, errors.Wrap(err, "need to specify a devicePath (ex. /dev/ttyUSB0)")
		}
	}
	logger.Info("attempting to connect to device at path " + devicePath)

	rplidarDevice, err := getRplidarDevice(devicePath)
	if err != nil {
		return nil, err
	}

	logger.Info("found and connected to an " + rplidarModelByteMap[rplidarDevice.model] + " rplidar")

	rp := &Rplidar{
		Named:      c.ResourceName().AsNamed(),
		device:     rplidarDevice,
		minRangeMM: svcConf.MinRangeMM,

		deviceMutex:            &sync.Mutex{},
		cacheMutex:             &sync.RWMutex{},
		cacheBackgroundWorkers: sync.WaitGroup{},

		logger: logger,
	}

	// Setup RPLiDAR
	if err := rp.setupRPLidar(ctx); err != nil {
		return nil, errors.Wrap(err, "there was a problem setting up the rplidar")
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)
	rp.cancelFunc = cancelFunc

	// Start background caching process
	rp.cacheBackgroundWorkers.Add(1)
	go func() {
		defer rp.cacheBackgroundWorkers.Done()
		rp.cachePointCloudLoop(cancelCtx)
	}()

	return rp, nil
}

// setupRPLiDAR starts the motor, in necessary, and warms up the device, discard several scans to
// ensure data returned to the user is valid.
func (rp *Rplidar) setupRPLidar(ctx context.Context) error {
	// Start the motor
	// Note: S1 rplidars do not need to start the motor before scanning can begin
	if rplidarModelByteMap[rp.device.model] != "S1" {
		rp.logger.Debug("starting motor")
		rp.device.driver.StartMotor()
	}

	// Setup rplidar scan and scan once as per warmup procedure
	rp.device.driver.StartScan(false, true)
	rp.nodes = gen.New_measurementNodeHqArray(defaultNodeSize)

	goutils.SelectContextOrWait(ctx, time.Duration(defaultWarmupNumDiscardedScans)*time.Second)
	if _, err := rp.scan(ctx, defaultWarmupNumDiscardedScans); err != nil {
		return err
	}

	return nil
}

// cachePointCloudLoop is a background process that repeatedly gets point cloud data from the rplidar
// and caches it for later access. This will reduce delays in returning data and prevent overcrowding
// of the rplidar's serial line.
func (rp *Rplidar) cachePointCloudLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			pc, err := rp.scan(ctx, defaultNumScans)
			if err != nil {
				rp.logger.Debugf("issue getting pointcloud to cache: %v", err)
			}

			rp.cacheMutex.Lock()
			rp.cachedPointCloud = pc
			rp.cacheMutex.Lock()
		}
	}
}

// scan uses the serial connection to the rplidar to get data and creates a pointcloud from the
// returned distances and angles
func (rp *Rplidar) scan(ctx context.Context, numScans int) (pointcloud.PointCloud, error) {
	rp.deviceMutex.Lock()
	defer rp.deviceMutex.Unlock()

	pc := pointcloud.New()

	var dropCount int
	nodeCount := int64(defaultNodeSize)
	for i := 0; i < numScans; i++ {
		result := rp.device.driver.GrabScanDataHq(rp.nodes, &nodeCount, defaultTimeoutMs)
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

			// Filter out points below minRange
			if nodeDistance < rp.minRangeMM {
				continue
			}

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

// NextPointCloud returns the current cached point cloud. If a user requests point cloud data faster than what
// the rplidar can manage, the same point cloud as before will be returned.
func (rp *Rplidar) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	rp.cacheMutex.RLock()
	defer rp.cacheMutex.RUnlock()

	if rp.cachedPointCloud == nil {
		return nil, errors.New("pointcloud has not been saved yet")
	}
	return rp.cachedPointCloud, nil
}

// Images is a part of the camera interface but is not implemented for the rplidar.
func (rp *Rplidar) Images(ctx context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	return nil, resource.ResponseMetadata{}, errors.New("images unimplemented")
}

// Properties returns information regarding the output of a camera, in this case that it returns PCDs.
func (rp *Rplidar) Properties(ctx context.Context) (camera.Properties, error) {
	props := camera.Properties{
		SupportsPCD: true,
	}
	return props, nil
}

// Projector is a part of the Camera interface but is not implemented for the rplidar.
func (rp *Rplidar) Projector(ctx context.Context) (transform.Projector, error) {
	var proj transform.Projector
	return proj, errors.New("projector unimplemented")
}

// Stream is a part of the Camera interface but is not implemented for the rplidar.
func (rp *Rplidar) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	var stream gostream.VideoStream
	return stream, errors.New("stream unimplemented")
}

// Close stops the rplidar and disposes of the driver.
func (rp *Rplidar) Close(ctx context.Context) error {

	// Close background process
	rp.cancelFunc()
	rp.cacheBackgroundWorkers.Done()

	rp.cacheMutex.Lock()
	defer rp.cacheMutex.Unlock()

	// Close driver related resources
	rp.deviceMutex.Lock()
	defer rp.deviceMutex.Unlock()

	if rp.device.driver != nil {
		if rp.nodes != nil {
			defer func() {
				gen.Delete_measurementNodeHqArray(rp.nodes)
				rp.nodes = nil
			}()
		}
		rp.device.driver.Stop()
		// Stop the motor
		// Note: S1 rplidars do not require the motor to be stopped during closeout
		if rplidarModelByteMap[rp.device.model] != "S1" {
			rp.logger.Debug("stopping motor")
			rp.device.driver.StopMotor()
		}

		gen.RPlidarDriverDisposeDriver(rp.device.driver)
		rp.device.driver = nil
	}

	return nil
}

func pointFrom(yaw, pitch, distance float64, reflectivity uint8) (r3.Vector, pointcloud.Data) {
	ea := spatialmath.NewEulerAngles()
	ea.Yaw = yaw
	ea.Pitch = pitch

	pose1 := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, ea)
	pose2 := spatialmath.NewPoseFromPoint(r3.Vector{X: distance, Y: 0, Z: 0})
	p := spatialmath.Compose(pose1, pose2).Point()

	// Rotate the point 180 degrees on the y axis. Since lidar data is always 2D, we don't worry
	// about the Z value.
	p.X = -p.X

	pos := pointcloud.NewVector(p.X*1000, p.Y*1000, p.Z*1000)
	d := pointcloud.NewBasicData()
	d.SetIntensity(uint16(reflectivity) * 255)

	return pos, d
}

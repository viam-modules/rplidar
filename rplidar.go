// Package rplidar implements a general rplidar LIDAR as a camera.
package rplidar

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rplidar/gen"

	"go.viam.com/utils/usb"

	goutils "go.viam.com/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
	"golang.org/x/exp/slices"
)

const (
	defaultTimeout = uint(1000)
)

var (
	// Model is the model of the rplidar
	Model = resource.NewModel("viam", "lidar", "rplidar")
	// availableRplidarModels is a list of supported rplidar models
	availableRplidarModels = []string{"A1", "A2", "A3", "S1"}
)

// Rplidar controls an Rplidar device.
type Rplidar struct {
	resource.Named
	resource.AlwaysRebuild
	mu                      sync.Mutex
	close                   bool
	model                   string
	device                  rplidarDevice
	nodes                   gen.Rplidar_response_measurement_node_hq_t
	nodeSize                int
	started                 bool
	scannedOnce             bool
	defaultNumScans         int
	warmupNumDiscardedScans int

	logger golog.Logger
}

// Config describes how to configure the RPlidar component.
type Config struct {
	DevicePath   string `json:"device_path"`
	RplidarModel string `json:"rplidar_model"`
}

// Validate checks that the config attributes are valid for an RPlidar.
func (conf *Config) Validate(path string) ([]string, error) {
	return nil, nil
}

func init() {
	resource.RegisterComponent(camera.API, Model, resource.Registration[camera.Camera, *Config]{Constructor: newRplidar})
}

func newRplidar(ctx context.Context, _ resource.Dependencies, c resource.Config, logger golog.Logger) (camera.Camera, error) {
	svcConf, err := resource.NativeConfig[*Config](c)
	if err != nil {
		return nil, err
	}

	rplidarModel := svcConf.RplidarModel
	if rplidarModel == "" {
		logger.Info("No rplidar model given, setting to default of A1")
		rplidarModel = "A1"
	}
	if !slices.Contains(availableRplidarModels, rplidarModel) {
		return nil, errors.Errorf("invalid rplidar model given, please choose one of the following %v", availableRplidarModels)
	}

	devicePath := svcConf.DevicePath
	if devicePath == "" {
		var err error
		if devicePath, err = searchForDevicePath(logger); err != nil {
			return nil, errors.Wrap(err, "need to specify a devicePath (ex. /dev/ttyUSB0)")
		}
	}
	logger.Info("connected to device at path " + devicePath)

	rplidarDevice, err := getRplidarDevice(devicePath)
	if err != nil {
		return nil, err
	}

	rp := &Rplidar{
		Named:                   c.ResourceName().AsNamed(),
		model:                   rplidarModel,
		device:                  rplidarDevice,
		nodeSize:                8192,
		logger:                  logger,
		defaultNumScans:         1,
		warmupNumDiscardedScans: 5,
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.start()

	return rp, nil
}

// NextPointCloud performs a scan on the rplidar and performs some filtering to clean up the data.
func (rp *Rplidar) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.close {
		return nil, errors.New("resource (rplidar) is closed")
	}

	pc, err := rp.getPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	return pc, nil
}

// start requests that the rplidar starts up and starts spinning.
func (rp *Rplidar) start() {
	rp.started = true
	if rp.model != "S1" {
		rp.logger.Debug("starting motor")
		rp.device.driver.StartMotor()
	}
	rp.device.driver.StartScan(false, true)
	rp.nodes = gen.New_measurementNodeHqArray(rp.nodeSize)
}

// stop request that the rplidar stops spinning.
func (rp *Rplidar) stop() {
	if rp.nodes != nil {
		defer func() {
			gen.Delete_measurementNodeHqArray(rp.nodes)
			rp.nodes = nil
		}()
	}
	rp.device.driver.Stop()
	if rp.model != "S1" {
		rp.logger.Debug("stopping motor")
		rp.device.driver.StopMotor()
	}
	rp.started = false
}

func (rp *Rplidar) scan(ctx context.Context, numScans int) (pointcloud.PointCloud, error) {
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

func (rp *Rplidar) getPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if !rp.started {
		rp.start()
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
func (rp *Rplidar) Properties(ctx context.Context) (camera.Properties, error) {
	var props camera.Properties
	return props, errors.New("properties unimplemented")
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
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.close = true

	if rp.device.driver != nil {
		rp.stop()
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

func searchForDevicePath(logger golog.Logger) (string, error) {
	var usbInfo = &usb.Identifier{
		Vendor:  0x10c4,
		Product: 0xea60,
	}

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

// Package rplidar implements a general rplidar LIDAR as a camera.
package rplidar

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"sync"
	"time"

	"go.viam.com/rplidar/gen"
	"go.viam.com/utils"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/usb"
	rdkUtils "go.viam.com/rdk/utils"
)

const (
	// ModelName is how the lidar will be registered into rdk.
	ModelName      = "rplidar"
	defaultTimeout = uint(1000)
)

var USBInfo = &usb.Identifier{
	Vendor:  0x10c4,
	Product: 0xea60,
}

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		ModelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			port := config.Attributes.Int("port", 8081)
			devicePath := config.Attributes.String("device_path")
			if devicePath == "" {
				return nil, errors.New("need to specify a devicePath (ex. /dev/ttyUSB0")
			}
			dataFolder := config.Attributes.String("data_folder")
			if dataFolder == "" {
				return nil, errors.New("need to specify a folder for the lidar data")
			}
			return NewRPLidar(logger, port, devicePath, dataFolder)
		}})
}

type (
	// Result describes the status of an RPLidar operation.
	Result uint32

	// ResultError is a result that encodes an error.
	ResultError struct {
		Result
	}
)

// The set of possible results.
var (
	ResultOk                 = Result(gen.RESULT_OK)
	ResultAlreadyDone        = Result(gen.RESULT_ALREADY_DONE)
	ResultInvalidData        = Result(gen.RESULT_INVALID_DATA)
	ResultOpFail             = Result(gen.RESULT_OPERATION_FAIL)
	ResultOpTimeout          = Result(gen.RESULT_OPERATION_TIMEOUT)
	ResultOpStop             = Result(gen.RESULT_OPERATION_STOP)
	ResultOpNotSupported     = Result(gen.RESULT_OPERATION_NOT_SUPPORT)
	ResultFormatNotSupported = Result(gen.RESULT_FORMAT_NOT_SUPPORT)
	ResultInsufficientMemory = Result(gen.RESULT_INSUFFICIENT_MEMORY)
)

// Failed returns an error if the result is that of a failure.
func (r Result) Failed() error {
	if uint64(r)&gen.RESULT_FAIL_BIT == 0 {
		return nil
	}
	return ResultError{r}
}

// String returns a human readable version of a result.
func (r Result) String() string {
	switch r {
	case ResultOk:
		return "Ok"
	case ResultAlreadyDone:
		return "AlreadyDone"
	case ResultInvalidData:
		return "InvalidData"
	case ResultOpFail:
		return "OpFail"
	case ResultOpTimeout:
		return "OpTimeout"
	case ResultOpStop:
		return "OpStop"
	case ResultOpNotSupported:
		return "OpNotSupported"
	case ResultFormatNotSupported:
		return "FormatNotSupported"
	case ResultInsufficientMemory:
		return "InsufficientMemory"
	default:
		return "Unknown"
	}
}

// Error returns the error as a human readable string.
func (r ResultError) Error() string {
	return r.String()
}

// NewRPLidar returns a new RPLidar device at the given path.
func NewRPLidar(logger golog.Logger, port int, devicePath string, dataFolder string) (camera.Camera, error) {
	var driver gen.RPlidarDriver
	devInfo := gen.NewRplidar_response_device_info_t()
	defer gen.DeleteRplidar_response_device_info_t(devInfo)

	var connectErr error
	for _, rate := range []uint{256000, 115200} {
		possibleDriver := gen.RPlidarDriverCreateDriver(uint(gen.DRIVER_TYPE_SERIALPORT))
		if result := possibleDriver.Connect(devicePath, rate); Result(result) != ResultOk {
			r := Result(result)
			if r == ResultOpTimeout {
				continue
			}
			connectErr = fmt.Errorf("failed to connect: %w", Result(result).Failed())
			continue
		}

		if result := possibleDriver.GetDeviceInfo(devInfo, defaultTimeout); Result(result) != ResultOk {
			r := Result(result)
			if r == ResultOpTimeout {
				continue
			}
			connectErr = fmt.Errorf("failed to get device info: %w", Result(result).Failed())
			continue
		}
		driver = possibleDriver
		break
	}
	if driver == nil {
		if connectErr == nil {
			return nil, fmt.Errorf("timed out connecting to %q", devicePath)
		}
		return nil, connectErr
	}

	serialNum := devInfo.GetSerialnum()
	var serialNumStr string
	for pos := 0; pos < 16; pos++ {
		serialNumStr += fmt.Sprintf("%02X", gen.ByteArray_getitem(serialNum, pos))
	}

	firmwareVer := fmt.Sprintf("%d.%02d",
		devInfo.GetFirmware_version()>>8,
		devInfo.GetFirmware_version()&0xFF)
	hardwareRev := int(devInfo.GetHardware_version())

	healthInfo := gen.NewRplidar_response_device_health_t()
	defer gen.DeleteRplidar_response_device_health_t(healthInfo)

	if result := driver.GetHealth(healthInfo, defaultTimeout); Result(result) != ResultOk {
		gen.RPlidarDriverDisposeDriver(driver)
		driver = nil
		return nil, fmt.Errorf("failed to get health: %w", Result(result).Failed())
	}

	if int(healthInfo.GetStatus()) == gen.RPLIDAR_STATUS_ERROR {
		gen.RPlidarDriverDisposeDriver(driver)
		driver = nil
		return nil, errors.New("bad health")
	}

	d := &Device{
		driver:                  driver,
		nodeSize:                8192,
		logger:                  logger,
		model:                   devInfo.GetModel(),
		serialNumber:            serialNumStr,
		firmwareVersion:         firmwareVer,
		hardwareRevision:        hardwareRev,
		defaultNumScans:         1,
		warmupNumDiscardedScans: 5,
		dataFolder:              dataFolder,
	}
	d.Start()
	return d, nil
}

// Device controls an RPLidar device.
type Device struct {
	mu                      sync.Mutex
	driver                  gen.RPlidarDriver
	nodes                   gen.Rplidar_response_measurement_node_hq_t
	nodeSize                int
	started                 bool
	scannedOnce             bool
	bounds                  *r3.Vector
	defaultNumScans         int
	warmupNumDiscardedScans int
	dataFolder              string

	logger golog.Logger

	// info
	model            byte
	serialNumber     string
	firmwareVersion  string
	hardwareRevision int
}

// Info returns metadata about the device.
func (d *Device) Info() (map[string]interface{}, error) {
	return map[string]interface{}{
		"model":             d.Model(),
		"serial_number":     d.serialNumber,
		"firmware_version":  d.firmwareVersion,
		"hardware_revision": d.hardwareRevision,
	}, nil
}

// SerialNumber returns the serial number of the device.
func (d *Device) SerialNumber() string {
	return d.serialNumber
}

// FirmwareVersion returns the firmware version of the device.
func (d *Device) FirmwareVersion() string {
	return d.firmwareVersion
}

// HardwareRevision returns the hardware version of the device.
func (d *Device) HardwareRevision() int {
	return d.hardwareRevision
}

const (
	modelA1 = 24
	modelA3 = 49
)

// Model returns the model number of the device, if it is known.
func (d *Device) Model() string {
	switch d.model {
	case modelA1:
		return "A1"
	case modelA3:
		return "A3"
	default:
		return "unknown"
	}
}

// Range returns the meter range of the device.
func (d *Device) Range() (float64, error) {
	switch d.model {
	case modelA1:
		return 12, nil
	case modelA3:
		return 25, nil
	default:
		return 0, fmt.Errorf("range unknown for model %d", d.model)
	}
}

func (d *Device) filterParams() (minAngleDiff float64, maxDistDiff float64) {
	switch d.model {
	case modelA1:
		return .9, .05
	case modelA3:
		return .3375, .05
	default:
		return -math.MaxFloat64, math.MaxFloat64
	}
}

// Bounds returns the square meter bounds of the device.
func (d *Device) Bounds() (r3.Vector, error) {
	if d.bounds != nil {
		return *d.bounds, nil
	}
	r, err := d.Range()
	if err != nil {
		return r3.Vector{}, err
	}
	width := r * 2
	height := width
	bounds := r3.Vector{width, height, 1}
	d.bounds = &bounds
	return bounds, nil
}

// Start requests that the device starts up and starts spinning.
func (d *Device) Start() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.started = true
	d.logger.Debugf("starting motor")
	d.driver.StartMotor()
	d.driver.StartScan(false, true)
	d.nodes = gen.New_measurementNodeHqArray(d.nodeSize)
}

// Stop request that the device stops spinning.
func (d *Device) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.nodes != nil {
		defer func() {
			gen.Delete_measurementNodeHqArray(d.nodes)
			d.nodes = nil
		}()
	}
	d.logger.Debugf("stopping motor")
	d.driver.Stop()
	d.driver.StopMotor()
	d.started = false
}

// NextPointCloud performs a scan on the device and performs some filtering to clean up the data.
// It also saves the pointcloud in form of a pcd file.
func (d *Device) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	pc, timeStamp, err := d.getPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	if err = d.savePCDFile(timeStamp, pc); err != nil {
		return nil, err
	}
	return pc, nil
}

func (d *Device) scan(ctx context.Context, numScans int) (pointcloud.PointCloud, error) {
	pc := pointcloud.New()
	nodeCount := int64(d.nodeSize)

	var dropCount int
	for i := 0; i < numScans; i++ {
		nodeCount = int64(d.nodeSize)
		result := d.driver.GrabScanDataHq(d.nodes, &nodeCount, defaultTimeout)
		if Result(result) != ResultOk {
			return nil, fmt.Errorf("bad scan: %w", Result(result).Failed())
		}
		d.driver.AscendScanData(d.nodes, nodeCount)

		for pos := 0; pos < int(nodeCount); pos++ {
			node := gen.MeasurementNodeHqArray_getitem(d.nodes, pos)
			if node.GetDist_mm_q2() == 0 {
				dropCount++
				continue // TODO(erd): okay to skip?
			}

			nodeAngle := (float64(node.GetAngle_z_q14()) * 90 / (1 << 14))
			nodeDistance := float64(node.GetDist_mm_q2()) / 4

			err := pc.Set(pointFrom(rdkUtils.DegToRad(nodeAngle), rdkUtils.DegToRad(0), float64(nodeDistance)/1000, 255))
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

func (d *Device) savePCDFile(timeStamp time.Time, pc pointcloud.PointCloud) error {
	f, err := os.Create(d.dataFolder + "/rplidar_data_" + timeStamp.UTC().Format("2006-01-02T15_04_05.0000") + ".pcd")
	if err != nil {
		return err
	}

	w := bufio.NewWriter(f)
	if err = pc.ToPCD(w); err != nil {
		return err
	}
	if err = w.Flush(); err != nil {
		return err
	}
	return f.Close()
}

func (d *Device) getPointCloud(ctx context.Context) (pointcloud.PointCloud, time.Time, error) {
	if !d.started {
		d.Start()
	}

	// wait and then discard scans for warmup
	if !d.scannedOnce {
		d.scannedOnce = true
		utils.SelectContextOrWait(ctx, time.Duration(d.warmupNumDiscardedScans)*time.Second)
		if _, err := d.scan(ctx, d.warmupNumDiscardedScans); err != nil {
			return nil, time.Now(), err
		}
	}

	pc, err := d.scan(ctx, d.defaultNumScans)
	if err != nil {
		return nil, time.Now(), err
	}
	return pc, time.Now(), nil
}

func pointFrom(yaw, pitch, distance float64, reflectivity uint8) pointcloud.Point {
	ea := spatialmath.NewEulerAngles()
	ea.Yaw = yaw
	ea.Pitch = pitch

	pose1 := spatialmath.NewPoseFromOrientation(r3.Vector{0, 0, 0}, ea)
	pose2 := spatialmath.NewPoseFromPoint(r3.Vector{distance, 0, 0})
	p := spatialmath.Compose(pose1, pose2).Point()

	pc := pointcloud.NewBasicPoint(p.X*1000, p.Y*1000, p.Z*1000).SetIntensity(uint16(reflectivity) * 255)
	pc = pc.SetColor(color.NRGBA{255, 0, 0, 255})

	return pc
}

// AngularResolution returns the highest angular resolution the device offers.
func (d *Device) AngularResolution() (float64, error) {
	switch d.model {
	case modelA1:
		return .9, nil
	case modelA3:
		return .3375, nil
	default:
		return 1, nil
	}
}

// Next grabs the next image.
func (d *Device) Next(ctx context.Context) (image.Image, func(), error) {
	pc, _, err := d.getPointCloud(ctx)
	if err != nil {
		return nil, nil, err
	}

	minX := 0.0
	minY := 0.0

	maxX := 0.0
	maxY := 0.0

	pc.Iterate(func(p pointcloud.Point) bool {
		pos := p.Position()
		minX = math.Min(minX, pos.X)
		maxX = math.Max(maxX, pos.X)
		minY = math.Min(minY, pos.Y)
		maxY = math.Max(maxY, pos.Y)
		return true
	})

	width := 800
	height := 800

	scale := func(x, y float64) (int, int) {
		return int(float64(width) * ((x - minX) / (maxX - minX))),
			int(float64(height) * ((y - minY) / (maxY - minY)))
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	set := func(xpc, ypc float64, clr color.NRGBA) {
		x, y := scale(xpc, ypc)
		img.SetNRGBA(x, y, clr)
	}

	pc.Iterate(func(p pointcloud.Point) bool {
		set(p.Position().X, p.Position().Y, color.NRGBA{255, 0, 0, 255})
		return true
	})

	centerSize := .1
	for x := -1 * centerSize; x < centerSize; x += .01 {
		for y := -1 * centerSize; y < centerSize; y += .01 {
			set(x, y, color.NRGBA{0, 255, 0, 255})
		}
	}

	return img, nil, nil
}

// Close stops the device and disposes of the driver.
func (d *Device) Close(ctx context.Context) error {
	if d.driver != nil {
		d.Stop()
		gen.RPlidarDriverDisposeDriver(d.driver)
		d.driver = nil
	}
	return nil
}

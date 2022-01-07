package rplidar

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"sync"
	"time"

	"go.viam.com/rplidar/gen"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
	gutils "go.viam.com/utils"
)

const (
	// ModelName is how the lidar will be registered into core.
	ModelName = "rplidar"

	// Type is the lidar specific type.
	Type = camera.SubtypeName
)

func init() {
	registry.RegisterComponent(camera.Subtype, "rplidar", registry.Component{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
		devicePath := config.Attributes.String("device_path")
		if devicePath == "" {
			return nil, errors.New("need to specify a devicePath (ex. /dev/ttyUSB0")
		}
		return NewDevice(devicePath)
	}})
	// camera.RegisterType(rplidar.Type, camera.TypeRegistration{
	// 	USBInfo: &usb.Identifier{
	// 		Vendor:  0x10c4,
	// 		Product: 0xea60,
	// 	},
	// })
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

const defaultTimeout = uint(1000)

// NewDevice returns a new RPLidar device at the given path.
func NewDevice(devicePath string) (camera.Camera, error) {
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
		return nil, fmt.Errorf("failed to get health: %w", Result(result).Failed())
	}

	if int(healthInfo.GetStatus()) == gen.RPLIDAR_STATUS_ERROR {
		return nil, errors.New("bad health")
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	d := &Device{
		driver:           driver,
		nodeSize:         8192,
		model:            devInfo.GetModel(),
		serialNumber:     serialNumStr,
		firmwareVersion:  firmwareVer,
		hardwareRevision: hardwareRev,
	}

	d.cancelFunc = cancelFunc
	d.activeBackgroundWorkers.Add(1)

	err := d.Start(cancelCtx)
	if err != nil {
		return nil, fmt.Errorf("error on startup of rplidar %v", err)
	}

	// gutils.PanicCapturingGo(func() {
	// 	if err == nil {
	// 		d.run(cancelCtx)
	// 	}
	// })

	return d, nil
}

// Device controls an RPLidar device.
type Device struct {
	mu          sync.Mutex
	driver      gen.RPlidarDriver
	nodes       gen.Rplidar_response_measurement_node_hq_t
	nodeSize    int
	started     bool
	scannedOnce bool
	bounds      *r3.Vector

	// info
	model            byte
	serialNumber     string
	firmwareVersion  string
	hardwareRevision int

	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

type ScanOptions struct {
	// Count determines how many scans to perform.
	Count int
}

// Info returns metadata about the device.
func (d *Device) Info(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"model":             d.Model(),
		"serial_number":     d.serialNumber,
		"firmware_version":  d.firmwareVersion,
		"hardware_revision": d.hardwareRevision,
	}, nil
}

func (d *Device) run(ctx context.Context) error {
	defer d.activeBackgroundWorkers.Done()
	defer d.Stop(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := ctx.Err()
		if err != nil {
			// cancelled
			return err
		}

		if !gutils.SelectContextOrWait(ctx, time.Second) {
			return nil
		}

	}
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
func (d *Device) Range(ctx context.Context) (float64, error) {
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
func (d *Device) Bounds(ctx context.Context) (r3.Vector, error) {
	if d.bounds != nil {
		return *d.bounds, nil
	}
	r, err := d.Range(ctx)
	if err != nil {
		return r3.Vector{}, err
	}
	width := r * 2
	height := width
	bounds := r3.Vector{width, height, 1}
	d.bounds = &bounds
	return bounds, nil
}

// Start requests that the device start up (start spinning).
func (d *Device) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.start()
	return nil
}

func (d *Device) start() {
	golog.Global.Debugf("starting motor")

	d.started = true
	d.driver.StartMotor()
	d.driver.StartScan(false, true)
	d.nodes = gen.New_measurementNodeHqArray(d.nodeSize)
}

// Stop request that the device stop (stop spinning).
func (d *Device) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.nodes != nil {
		defer func() {
			gen.Delete_measurementNodeHqArray(d.nodes)
			d.nodes = nil
		}()
	}
	golog.Global.Debugf("stopping motor")

	d.driver.Stop()
	d.driver.StopMotor()
	return nil
}

// Close just stops the device.
// func (d *Device) Close(ctx context.Context) error {
// 	return d.Stop(ctx)
// }

const defaultNumScans = 3

// Scan performs a scan on the device and performs some filtering to clean up the data.
func (d *Device) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.scan(ctx)
}

func (d *Device) scan(ctx context.Context) (pointcloud.PointCloud, error) {
	if !d.started {
		d.start()
		d.started = true
	}
	if !d.scannedOnce {
		d.scannedOnce = true
		// discard scans for warmup
		//nolint
		//sopt.Count = 10
		d.scan(ctx)
		time.Sleep(time.Second)
	}

	numScans := defaultNumScans // 3
	//if sopt.Count != 0 {
	//	numScans = sopt.Count
	//}

	nodeCount := int64(d.nodeSize)
	//measurements := make(lidar.Measurements, 0, nodeCount*int64(numScans))

	pc := pointcloud.New()

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

			err := pc.Set(pointFrom(utils.DegToRad(nodeAngle), utils.DegToRad(0), float64(nodeDistance)/1000, 255))
			if err != nil {
				return nil, err
			}
		}
	}
	if pc.Size() == 0 {
		return nil, nil
	}

	return pc, pc.WriteToFile("bar.las")

	// if options.NoFilter {
	// 	return measurements, nil
	// }
	// sort.Stable(measurements)
	// filteredMeasurements := make(lidar.Measurements, 0, len(measurements))

	// minAngleDiff, maxDistDiff := d.filterParams()
	// prev := measurements[0]
	// detectedRay := false
	// for mIdx := 1; mIdx < len(measurements); mIdx++ {
	// 	curr := measurements[mIdx]
	// 	currAngle := curr.AngleDeg()
	// 	prevAngle := prev.AngleDeg()
	// 	currDist := curr.Distance()
	// 	prevDist := prev.Distance()
	// 	if math.Abs(currAngle-prevAngle) < minAngleDiff {
	// 		if math.Abs(currDist-prevDist) > maxDistDiff {
	// 			detectedRay = true
	// 			continue
	// 		}
	// 	}
	// 	prev = curr
	// 	if !detectedRay {
	// 		filteredMeasurements = append(filteredMeasurements, curr)
	// 	}
	// 	detectedRay = false
	// }
	// return filteredMeasurements, nil
}

func pointFrom(yaw, pitch, distance float64, reflectivity uint8) pointcloud.Point {
	ea := spatialmath.NewEulerAngles()
	ea.Yaw = yaw
	ea.Pitch = pitch

	pose1 := spatialmath.NewPoseFromOrientation(r3.Vector{0, 0, 0}, ea)
	pose2 := spatialmath.NewPoseFromPoint(r3.Vector{distance, 0, 0})
	p := spatialmath.Compose(pose1, pose2).Point()

	//fmt.Printf("Reflectivity = %v | Type = %T | GoRep = %#v \n", reflectivity, reflectivity, reflectivity)

	pc := pointcloud.NewBasicPoint(p.X*1000, p.Y*1000, p.Z*1000).SetIntensity(uint16(reflectivity) * 255)
	pc = pc.SetColor(color.NRGBA{255, 0, 0, 255})

	//fmt.Printf(" PC: X = %v | Y = %v | Z = %v \n", p.X, p.Y, p.Z)

	return pc
}

// AngularResolution returns the highest angular resolution the device offers.
func (d *Device) AngularResolution(ctx context.Context) (float64, error) {
	switch d.model {
	case modelA1:
		return .9, nil
	case modelA3:
		return .3375, nil
	default:
		return 1, nil
	}
}

func (d *Device) Next(ctx context.Context) (image.Image, func(), error) {
	pc, err := d.NextPointCloud(ctx)
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

func (d *Device) Close(ctx context.Context) error {
	d.cancelFunc()
	d.activeBackgroundWorkers.Wait()
	return nil
}

package rplidarserial

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/viamrobotics/rplidar"
	rplidargen "github.com/viamrobotics/rplidar/gen"

	"github.com/viamrobotics/robotcore/lidar"
	"github.com/viamrobotics/robotcore/usb"
	"github.com/viamrobotics/robotcore/utils"
)

func init() {
	lidar.RegisterDeviceType(rplidar.DeviceType, lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription) (lidar.Device, error) {
			return NewDevice(desc.Path)
		},
		USBInfo: &usb.Identifier{
			Vendor:  0x10c4,
			Product: 0xea60,
		},
	})
}

type Result uint32
type ResultError struct {
	Result
}

var (
	ResultOk                 = Result(rplidargen.RESULT_OK)
	ResultAlreadyDone        = Result(rplidargen.RESULT_ALREADY_DONE)
	ResultInvalidData        = Result(rplidargen.RESULT_INVALID_DATA)
	ResultOpFail             = Result(rplidargen.RESULT_OPERATION_FAIL)
	ResultOpTimeout          = Result(rplidargen.RESULT_OPERATION_TIMEOUT)
	ResultOpStop             = Result(rplidargen.RESULT_OPERATION_STOP)
	ResultOpNotSupported     = Result(rplidargen.RESULT_OPERATION_NOT_SUPPORT)
	ResultFormatNotSupported = Result(rplidargen.RESULT_FORMAT_NOT_SUPPORT)
	ResultInsufficientMemory = Result(rplidargen.RESULT_INSUFFICIENT_MEMORY)
)

func (r Result) Failed() error {
	if uint64(r)&rplidargen.RESULT_FAIL_BIT == 0 {
		return nil
	}
	return ResultError{r}
}

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

func (r ResultError) Error() string {
	return r.String()
}

const defaultTimeout = uint(1000)

func NewDevice(devicePath string) (*Device, error) {
	var driver rplidargen.RPlidarDriver
	devInfo := rplidargen.NewRplidar_response_device_info_t()
	defer rplidargen.DeleteRplidar_response_device_info_t(devInfo)

	var connectErr error
	for _, rate := range []uint{256000, 115200} {
		possibleDriver := rplidargen.RPlidarDriverCreateDriver(uint(rplidargen.DRIVER_TYPE_SERIALPORT))
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
		serialNumStr += fmt.Sprintf("%02X", rplidargen.ByteArray_getitem(serialNum, pos))
	}

	firmwareVer := fmt.Sprintf("%d.%02d",
		devInfo.GetFirmware_version()>>8,
		devInfo.GetFirmware_version()&0xFF)
	hardwareRev := int(devInfo.GetHardware_version())

	healthInfo := rplidargen.NewRplidar_response_device_health_t()
	defer rplidargen.DeleteRplidar_response_device_health_t(healthInfo)

	if result := driver.GetHealth(healthInfo, defaultTimeout); Result(result) != ResultOk {
		return nil, fmt.Errorf("failed to get health: %w", Result(result).Failed())
	}

	if int(healthInfo.GetStatus()) == rplidargen.RPLIDAR_STATUS_ERROR {
		return nil, errors.New("bad health")
	}

	return &Device{
		driver:           driver,
		nodeSize:         8192,
		model:            devInfo.GetModel(),
		serialNumber:     serialNumStr,
		firmwareVersion:  firmwareVer,
		hardwareRevision: hardwareRev,
	}, nil
}

type Device struct {
	mu          sync.Mutex
	driver      rplidargen.RPlidarDriver
	nodes       rplidargen.Rplidar_response_measurement_node_hq_t
	nodeSize    int
	started     bool
	scannedOnce bool
	bounds      *image.Point

	// info
	model            byte
	serialNumber     string
	firmwareVersion  string
	hardwareRevision int
}

func (d *Device) Info() *rplidar.DeviceInfo {
	return &rplidar.DeviceInfo{
		Model:            d.Model(),
		SerialNumber:     d.serialNumber,
		FirmwareVersion:  d.firmwareVersion,
		HardwareRevision: d.hardwareRevision,
	}
}

func (d *Device) SerialNumber() string {
	return d.serialNumber
}

func (d *Device) FirmwareVersion() string {
	return d.firmwareVersion
}

func (d *Device) HardwareRevision() int {
	return d.hardwareRevision
}

const (
	modelA1 = 24
	modelA3 = 49
)

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

func (d *Device) Range(ctx context.Context) (int, error) {
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

func (d *Device) Bounds(ctx context.Context) (image.Point, error) {
	if d.bounds != nil {
		return *d.bounds, nil
	}
	r, err := d.Range(ctx)
	if err != nil {
		return image.Point{}, err
	}
	width := r * 2
	height := width
	bounds := image.Point{width, height}
	d.bounds = &bounds
	return bounds, nil
}

func (d *Device) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.start()
	return nil
}

func (d *Device) start() {
	d.started = true
	d.driver.StartMotor()
	d.driver.StartScan(false, true)
	d.nodes = rplidargen.New_measurementNodeHqArray(d.nodeSize)
}

func (d *Device) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.nodes != nil {
		defer func() {
			rplidargen.Delete_measurementNodeHqArray(d.nodes)
			d.nodes = nil
		}()
	}
	d.driver.Stop()
	d.driver.StopMotor()
	return nil
}

func (d *Device) Close(ctx context.Context) error {
	return d.Stop(ctx)
}

const defaultNumScans = 3

func (d *Device) Scan(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.scan(options)
}

func (d *Device) scan(options lidar.ScanOptions) (lidar.Measurements, error) {
	if !d.started {
		d.start()
		d.started = true
	}
	if !d.scannedOnce {
		d.scannedOnce = true
		// discard scans for warmup
		//nolint
		d.scan(lidar.ScanOptions{Count: 10})
		time.Sleep(time.Second)
	}

	numScans := defaultNumScans
	if options.Count != 0 {
		numScans = options.Count
	}
	// numScans = 1

	nodeCount := int64(d.nodeSize)
	measurements := make(lidar.Measurements, 0, nodeCount*int64(numScans))

	var dropCount int
	for i := 0; i < numScans; i++ {
		nodeCount = int64(d.nodeSize)
		result := d.driver.GrabScanDataHq(d.nodes, &nodeCount, defaultTimeout)
		if Result(result) != ResultOk {
			return nil, fmt.Errorf("bad scan: %w", Result(result).Failed())
		}
		d.driver.AscendScanData(d.nodes, nodeCount)

		for pos := 0; pos < int(nodeCount); pos++ {
			node := rplidargen.MeasurementNodeHqArray_getitem(d.nodes, pos)
			if node.GetDist_mm_q2() == 0 {
				dropCount++
				continue // TODO(erd): okay to skip?
			}

			nodeAngle := (float64(node.GetAngle_z_q14()) * 90 / (1 << 14))
			nodeDistance := float64(node.GetDist_mm_q2()) / 4
			measurements = append(measurements, lidar.NewMeasurement(nodeAngle, nodeDistance/1000))
		}
	}
	if len(measurements) == 0 {
		return nil, nil
	}
	if options.NoFilter {
		return measurements, nil
	}
	sort.Stable(measurements)
	filteredMeasurements := make(lidar.Measurements, 0, len(measurements))

	minAngleDiff, maxDistDiff := d.filterParams()
	prev := measurements[0]
	detectedRay := false
	for mIdx := 1; mIdx < len(measurements); mIdx++ {
		curr := measurements[mIdx]
		currAngle := utils.RadToDeg(curr.Angle())
		prevAngle := utils.RadToDeg(prev.Angle())
		currDist := curr.Distance()
		prevDist := prev.Distance()
		if math.Abs(currAngle-prevAngle) < minAngleDiff {
			if math.Abs(currDist-prevDist) > maxDistDiff {
				detectedRay = true
				continue
			}
		}
		prev = curr
		if !detectedRay {
			filteredMeasurements = append(filteredMeasurements, curr)
		}
		detectedRay = false
	}
	return filteredMeasurements, nil
}

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

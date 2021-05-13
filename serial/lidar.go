package serial

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.viam.com/rplidar"
	"go.viam.com/rplidar/gen"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
	"go.viam.com/core/config"
	"go.viam.com/core/lidar"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/usb"
)

func init() {
	registry.RegisterLidar(rplidar.ModelName, func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		return NewDevice(config.Host)
	})
	lidar.RegisterType(rplidar.Type, lidar.TypeRegistration{
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

func (r Result) Failed() error {
	if uint64(r)&gen.RESULT_FAIL_BIT == 0 {
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
	driver      gen.RPlidarDriver
	nodes       gen.Rplidar_response_measurement_node_hq_t
	nodeSize    int
	started     bool
	scannedOnce bool
	bounds      *r2.Point

	// info
	model            byte
	serialNumber     string
	firmwareVersion  string
	hardwareRevision int
}

func (d *Device) Info(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"model":             d.Model(),
		"serial_number":     d.serialNumber,
		"firmware_version":  d.firmwareVersion,
		"hardware_revision": d.hardwareRevision,
	}, nil
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

func (d *Device) Bounds(ctx context.Context) (r2.Point, error) {
	if d.bounds != nil {
		return *d.bounds, nil
	}
	r, err := d.Range(ctx)
	if err != nil {
		return r2.Point{}, err
	}
	width := r * 2
	height := width
	bounds := r2.Point{width, height}
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
	d.nodes = gen.New_measurementNodeHqArray(d.nodeSize)
}

func (d *Device) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.nodes != nil {
		defer func() {
			gen.Delete_measurementNodeHqArray(d.nodes)
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
			node := gen.MeasurementNodeHqArray_getitem(d.nodes, pos)
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
		currAngle := curr.AngleDeg()
		prevAngle := prev.AngleDeg()
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

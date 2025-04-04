// Package rplidar implements a general rplidar LIDAR as a camera.
package rplidar

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"go.viam.com/rplidar/gen"

	ps "github.com/mitchellh/go-ps"

	goutils "go.viam.com/utils"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
	rputils "go.viam.com/rplidar/utils"
)

// RPLiDARModel represents the model of rplidar being used
//
//nolint:golint
type RPLiDARModel int64

const (
	// The max time in milliseconds it should take for the RPlidar to get scan data.
	defaultDeviceTimeoutMs = uint(1000)
	// The number of full 360 scans to complete before returning a point cloud.
	defaultNumScans = 1
	// The number of scans to discard at startup to ensure valid data is returned to the user.
	defaultWarmupNumDiscardedScans = 5
	// The number of max nodes or data points returned in each scan.
	defaultNodeSize = 8192
	// The amount of time to wait after the motor start before scanning can begin.
	defaultWarmUpTimeout = time.Second

	rplidarModuleLockDir      = "/tmp/"
	rplidarModuleLockFileName = "rplidar_pid%v_dv%v.lock"
	devicePathPrefixOffset    = len(`\dev\`)

	// A1 rplidar model
	A1 RPLiDARModel = iota
	// A3 rplidar model
	A3
	// S1 rplidar model
	S1
)

var (
	// Model is the model of the RPLiDAR
	Model = resource.NewModel("viam", "lidar", "rplidar")
	// rplidarModelByteMap maps the byte model representation to a string representation
	rplidarModelByteMap = map[byte]RPLiDARModel{24: A1, 49: A3, 97: S1}
	// The max capture frequency for rplidar models, based on their datasheets
	maxScanningFrequencyByModel = map[RPLiDARModel]float64{A1: 10, A3: 15, S1: 15}
)

// modelToString converted the RPLiDARModel to a string
func modelToString(model RPLiDARModel) string {
	switch model {
	case A1:
		return "A1"
	case A3:
		return "A3"
	case S1:
		return "S1"
	default:
	}
	return "unsupported model"
}

// dataCache stores pointcloud data returned from the RPLiDAR for later access. This data is under mutex protection.
type dataCache struct {
	mutex      sync.RWMutex
	pointCloud pointcloud.PointCloud
}

// rplidar contains the connection, filters and data cached used to interface with an RPLiDAR device.
type rplidar struct {
	resource.Named
	resource.AlwaysRebuild

	lockFilePath string
	device       *rplidarDevice
	nodes        gen.Rplidar_response_measurement_node_hq_t
	minRangeMM   float64

	cancelFunc             func()
	cacheBackgroundWorkers sync.WaitGroup
	cache                  *dataCache

	logger logging.Logger
}

// Config describes how to configure the RPLiDAR component.
type Config struct {
	SerialPath string  `json:"serial_path"`
	MinRangeMM float64 `json:"min_range_mm"`
}

// Validate checks that the config attributes are valid for an RPLiDAR.
func (conf *Config) Validate(_ string) ([]string, error) {

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

	devicePath := svcConf.SerialPath
	if devicePath == "" {
		var err error
		if devicePath, err = searchForDevicePath(logger); err != nil {
			return nil, errors.Wrap(err, "need to specify a devicePath (ex. /dev/ttyUSB0)")
		}
	}

	// Check lock file for conflicting processes
	lockFilePath, err := checkLockFiles(devicePath)
	if err != nil {
		return nil, err
	}

	// Attempt to connect to rplidar
	logger.Info("attempting to connect to device at serial_path: " + devicePath)

	rplidarDevice, err := getRplidarDevice(devicePath)
	if err != nil {
		return nil, err
	}

	rplidarModel := rplidarModelByteMap[rplidarDevice.model]
	logger.Info("found and connected to an " + modelToString(rplidarModel) + " rplidar")

	// Check configured capture frequency
	captureFreqHz, err := getCaptureFrequencyHzFromConfig(c)
	if err != nil {
		return nil, err
	}

	if captureFreqHz > maxScanningFrequencyByModel[rplidarModel] {
		return nil, errors.Errorf("configured capture frequency (%v) is greater than max frequency (%v) for rplidar %v",
			captureFreqHz,
			maxScanningFrequencyByModel[rplidarModel],
			rplidarModel)
	}

	rp := &rplidar{
		Named:        c.ResourceName().AsNamed(),
		device:       rplidarDevice,
		lockFilePath: lockFilePath,
		minRangeMM:   svcConf.MinRangeMM,

		cache:                  &dataCache{},
		cacheBackgroundWorkers: sync.WaitGroup{},

		logger: logger,
	}

	// Setup RPLiDAR
	if err := rp.setupRPLidar(ctx); err != nil {
		return nil, errors.Wrap(err, "there was a problem setting up the rplidar")
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	rp.cancelFunc = cancelFunc

	// Start background caching of pointcloud data
	rp.cacheBackgroundWorkers.Add(1)
	go func() {
		defer rp.cacheBackgroundWorkers.Done()
		rp.cachePointCloudLoop(cancelCtx)
	}()

	return rp, nil
}

// setupRPLiDAR starts the motor, if necessary, warms up the device, and ensures data returned to the
// user is valid.
func (rp *rplidar) setupRPLidar(ctx context.Context) error {
	// Note: S1 RPLiDARs do not need to start the motor before scanning can begin
	if rplidarModelByteMap[rp.device.model] != S1 {
		rp.logger.Debug("starting motor")
		rp.device.driver.StartMotor()
	}

	// Perform warmup scans
	rp.device.driver.StartScan(false, true)
	rp.nodes = gen.New_measurementNodeHqArray(defaultNodeSize)

	goutils.SelectContextOrWait(ctx, defaultWarmUpTimeout)
	if _, err := rp.scan(ctx, defaultWarmupNumDiscardedScans); err != nil {
		return err
	}

	return nil
}

// cachePointCloudLoop is a background process that repeatedly gets point cloud data from the RPLiDAR
// and caches it for later access.
func (rp *rplidar) cachePointCloudLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			pc, err := rp.scan(ctx, defaultNumScans)
			if err != nil {
				rp.logger.Debugf("issue getting pointcloud to cache: %v", err)
			}

			rp.cache.mutex.Lock()
			rp.cache.pointCloud = pc
			rp.cache.mutex.Unlock()
		}
	}
}

// scan uses the serial connection to the RPLiDAR to get data and create a pointcloud from it
func (rp *rplidar) scan(_ context.Context, numScans int) (pointcloud.PointCloud, error) {
	rp.device.mutex.Lock()
	defer rp.device.mutex.Unlock()

	pc := pointcloud.New()

	var dropCount int
	nodeCount := int64(defaultNodeSize)
	for i := 0; i < numScans; i++ {
		result := rp.device.driver.GrabScanDataHq(rp.nodes, &nodeCount, defaultDeviceTimeoutMs)
		if Result(result) != ResultOk {
			return nil, fmt.Errorf("bad scan: %w", Result(result).Failed())
		}
		rp.device.driver.AscendScanData(rp.nodes, nodeCount)

		for pos := 0; pos < int(nodeCount); pos++ {

			node := gen.MeasurementNodeHqArray_getitem(rp.nodes, rputils.CastInt(pos))

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

// NextPointCloud returns the current cached point cloud. If no pointcloud has been added to the cache at the
// point this call is made, it will return an error
func (rp *rplidar) NextPointCloud(_ context.Context) (pointcloud.PointCloud, error) {
	rp.cache.mutex.RLock()
	defer rp.cache.mutex.RUnlock()

	if rp.cache.pointCloud == nil {
		return nil, errors.New("pointcloud has not been saved yet")
	}
	return rp.cache.pointCloud, nil
}

// Images is a part of the camera interface but is not implemented for the RPLiDAR.
func (rp *rplidar) Images(_ context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	return nil, resource.ResponseMetadata{}, errors.New("images unimplemented")
}

// Properties returns information regarding the output of the RPLiDAR, in this case that it returns PCDs.
func (rp *rplidar) Properties(_ context.Context) (camera.Properties, error) {
	props := camera.Properties{
		SupportsPCD: true,
	}
	return props, nil
}

// Image is a part of the Camera interface but is not implemented for the RPLiDAR.
func (rp *rplidar) Image(context.Context, string, map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
	return nil, camera.ImageMetadata{}, errors.New("image unimplemented")
}

// Close stops the RPLiDAR and disposes of the driver.
func (rp *rplidar) Close(_ context.Context) error {

	// Close background process
	rp.cancelFunc()
	rp.cacheBackgroundWorkers.Wait()
	rp.cache.mutex.Lock()
	defer rp.cache.mutex.Unlock()

	// Close driver related resources
	rp.device.mutex.Lock()
	defer rp.device.mutex.Unlock()

	if rp.device.driver != nil {
		if rp.nodes != nil {
			defer func() {
				gen.Delete_measurementNodeHqArray(rp.nodes)
				rp.nodes = nil
			}()
		}
		rp.device.driver.Stop()
		// Stop the motor
		// Note: S1 RPLiDAR do not require the motor to be stopped during closeout
		if rplidarModelByteMap[rp.device.model] != S1 {
			rp.logger.Debug("stopping motor")
			rp.device.driver.StopMotor()
		}

		gen.RPlidarDriverDisposeDriver(rp.device.driver)
		rp.device.driver = nil
	}

	if _, err := os.Stat(rp.lockFilePath); err == nil {
		if err := os.Remove(rp.lockFilePath); err != nil {
			return err
		}
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

// checkLockFiles compares the current process and device_path to rplidar.lock files to see if any ongoing
// sessions for that device path still exist
func checkLockFiles(devicePath string) (string, error) {

	// Get rplidar related processes
	rplidarProcesses, err := getRplidarProcesses()
	if err != nil {
		return "", errors.Wrapf(err, "error getting rplidar-module related processes")
	}
	currentProcess := rplidarProcesses[len(rplidarProcesses)-1]
	oldProcesses := rplidarProcesses[:len(rplidarProcesses)-1]

	// Get rplidar related lock files
	files, err := os.ReadDir(rplidarModuleLockDir)
	if err != nil {
		return "", errors.Wrapf(err, "error reading lock file directory")
	}
	var rplidarLockFiles []string
	for _, file := range files {
		if strings.Contains(file.Name(), "rplidar") {
			rplidarLockFiles = append(rplidarLockFiles, file.Name())
		}
	}

	// Look through lock files for those relating to active processes + given device path; if a lock file refers to
	// a no longer active process, delete it
	for _, lockFileName := range rplidarLockFiles {
		var matchFound bool
		for _, oldProc := range oldProcesses {
			if strings.Contains(lockFileName, fmt.Sprintf("pid%v", oldProc)) {
				matchFound = true
				if strings.Contains(lockFileName, fmt.Sprintf("dv%v", devicePath[devicePathPrefixOffset:])) {
					return "", errors.Errorf("another rplidar-module process using the same serial_path has been found, "+
						"possibly from an incomplete closure of a previous session. To use this serial path again, kill "+
						"the old process by running 'sudo kill -9 <PID>' (PID(s): %v)", oldProc)
				}

			}
		}
		// Remove lock files for processes that are not currently ongoing
		if !matchFound {
			if err := os.Remove(rplidarModuleLockDir + lockFileName); err != nil {
				return "", errors.Wrapf(err, fmt.Sprintf("could not remove lock file %v", lockFileName))
			}
		}
	}

	// Create lock file for current session
	newLockFile := rplidarModuleLockDir + fmt.Sprintf(rplidarModuleLockFileName, currentProcess, devicePath[devicePathPrefixOffset:])
	f, err := os.Create(newLockFile)
	if err != nil {
		return "", errors.Wrapf(err, "could not create lock file")
	}

	if err = f.Close(); err != nil {
		return "", errors.Wrapf(err, "could not close lock file")
	}

	return newLockFile, nil
}

// getRplidarProcesses returns the PIDs of the rplidar-module processes
func getRplidarProcesses() ([]int, error) {
	allProcesses, err := ps.Processes()
	if err != nil {
		return nil, err
	}

	var rplidarProcesses []int
	for _, p := range allProcesses {
		if p.Executable() == "rplidar-module" {
			rplidarProcesses = append(rplidarProcesses, p.Pid())
		}
	}

	sort.Ints(rplidarProcesses)

	return rplidarProcesses, nil
}

// getCaptureFrequencyHzFromConfig extract the capture_frequency_hz from the rplidar resource config
func getCaptureFrequencyHzFromConfig(c resource.Config) (float64, error) {
	var captureFreqHz float64
	var captureMethodFound bool

	for _, assocResourceCfg := range c.AssociatedResourceConfigs {
		if captureMethodsMapInterface := assocResourceCfg.Attributes["capture_methods"]; captureMethodsMapInterface != nil {
			captureMethodFound = true
			for _, captureMethodsInterface := range captureMethodsMapInterface.([]interface{}) {
				captureMethods := captureMethodsInterface.(map[string]interface{})
				if captureMethods["method"].(string) == "NextPointCloud" {
					captureFreqHz = captureMethods["capture_frequency_hz"].(float64)
				}
			}
		}
	}

	if captureMethodFound && captureFreqHz <= 0. {
		return 0.0, errors.New("zero or negative capture frequency")
	}

	return captureFreqHz, nil
}

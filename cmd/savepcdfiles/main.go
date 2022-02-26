package main

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/multierr"
	"go.viam.com/rplidar"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/metadata/service"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/usb"
	"go.viam.com/utils"

	robotimpl "go.viam.com/rdk/robot/impl"
)

var (
	defaultTimeDeltaMilliseconds = 100
	defaultPort                  = 8081
	defaultDataFolder            = "data"
	logger                       = rlog.Logger.Named("save_pcd_files")
	name                         = "rplidar"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

// Arguments for the command.
type Arguments struct {
	Port                  utils.NetPortFlag `flag:"0"`
	TimeDeltaMilliseconds int               `flag:"delta,usage=delta ms"`
	DevicePath            string            `flag:"device,usage=device path"`
	DataFolder            string            `flag:"datafolder,usage=datafolder path"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments

	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	if argsParsed.TimeDeltaMilliseconds == 0 {
		logger.Debugf("using default time delta %d ", defaultTimeDeltaMilliseconds)
		argsParsed.TimeDeltaMilliseconds = defaultTimeDeltaMilliseconds
	} else {
		logger.Debugf("using user defined time delta %d ", argsParsed.TimeDeltaMilliseconds)
	}

	if argsParsed.Port == 0 {
		logger.Debugf("using default port %d ", defaultPort)
		argsParsed.Port = utils.NetPortFlag(defaultPort)
	} else {
		logger.Debugf("using user defined port %d ", argsParsed.Port)
	}

	devicePath := argsParsed.DevicePath
	if devicePath == "" {
		usbDevices := usb.Search(
			usb.SearchFilter{},
			func(vendorID, productID int) bool {
				return vendorID == rplidar.USBInfo.Vendor && productID == rplidar.USBInfo.Product
			})

		if len(usbDevices) != 0 {
			logger.Debugf("detected %d lidar devices", len(usbDevices))
			for _, comp := range usbDevices {
				logger.Debug(comp)
			}
			devicePath = usbDevices[0].Path
		} else {
			return errors.New("no usb devices found")
		}
	}

	if argsParsed.DataFolder == "" {
		logger.Debugf("using default data folder '%s' ", defaultDataFolder)
		argsParsed.DataFolder = defaultDataFolder
	} else {
		logger.Debugf("using user defined data folder %s", argsParsed.DataFolder)
	}
	if err := os.MkdirAll(filepath.Join(".", argsParsed.DataFolder), os.ModePerm); err != nil {
		return errors.New("can not create a new directory named: " + argsParsed.DataFolder)
	}

	// Create rplidar component
	lidarComponent := config.Component{
		Name:  name,
		Type:  config.ComponentTypeCamera,
		Model: rplidar.ModelName,
		Attributes: config.AttributeMap{
			"device_path": devicePath,
			"data_folder": argsParsed.DataFolder,
		},
	}

	metadataSvc, err := service.New()
	if err != nil {
		return err
	}
	ctx = service.ContextWithService(ctx, metadataSvc)

	cfg := &config.Config{Components: []config.Component{lidarComponent}}
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}

	rplidar, ok := myRobot.CameraByName(name)
	if !ok {
		return errors.New("no rplidar found with name: " + name)
	}

	// Based on empirical data, we can see that the rplidar collects data at a rate of 15Hz,
	// which is ~ 66ms per scan. This issues a warning to the user, in case they're expecting
	// to receive data at a higher rate than what is technically possible.
	scanTimeDelta := argsParsed.TimeDeltaMilliseconds
	estimatedTimePerScan := 66
	if scanTimeDelta < int(estimatedTimePerScan) {
		logger.Warnf("the expected scan rate of deltaT=%v is too small, has to be at least %v", scanTimeDelta, estimatedTimePerScan)
	}

	return savePCDFiles(ctx, myRobot, rplidar, argsParsed.TimeDeltaMilliseconds, logger)
}

func savePCDFiles(ctx context.Context, myRobot robot.LocalRobot, rplidar camera.Camera, timeDeltaMilliseconds int, logger golog.Logger) (err error) {
	for {
		if !utils.SelectContextOrWait(ctx, time.Duration(math.Max(1, float64(timeDeltaMilliseconds)))*time.Millisecond) {
			return multierr.Combine(ctx.Err(), myRobot.Close(ctx))
		}

		pc, err := rplidar.NextPointCloud(ctx)
		if err != nil {
			if err.Error() == "bad scan: OpTimeout" {
				logger.Warnf("Skipping this scan due to error: %v", err)
				continue
			} else {
				return multierr.Combine(err, myRobot.Close(ctx))
			}
		}
		logger.Infow("scanned", "pointcloud_size", pc.Size())
	}
}

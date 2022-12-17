package main

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"

	"math"
	"time"

	"go.uber.org/multierr"
	"go.viam.com/rplidar"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"

	"go.viam.com/utils"

	robotimpl "go.viam.com/rdk/robot/impl"
)

var (
	defaultTimeDeltaMs = 100
	defaultDataFolder  = "data"
	logger             = golog.NewLogger("save_pcd_files")
	name               = "rplidar"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

// Arguments for the command.
type Arguments struct {
	TimeDeltaMs int    `flag:"delta,usage=delta ms"`
	DevicePath  string `flag:"device,usage=device path"`
	DataFolder  string `flag:"datafolder,usage=datafolder path"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments

	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	scanTimeDelta := getTimeDeltaMs(argsParsed.TimeDeltaMs, defaultTimeDeltaMs, logger)

	lidarDevice, err := rplidar.CreateRplidarComponent(name,
		argsParsed.DevicePath,
		camera.SubtypeName,
		logger)
	if err != nil {
		return err
	}

	cfg := &config.Config{Components: []config.Component{lidarDevice}}
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}

	res, err := myRobot.ResourceByName(camera.Named(name))
	if err != nil {
		return errors.New("no rplidar found with name: " + name)
	}

	rplidar := res.(camera.Camera)

	dataFolder, err := getDataFolder(argsParsed.DataFolder, logger)
	if err != nil {
		return err
	}

	return savePCDFiles(ctx, myRobot, rplidar, dataFolder, scanTimeDelta, logger)
}

func savePCDFiles(ctx context.Context, contextCloser utils.ContextCloser, rplidar camera.PointCloudSource, dataFolder string, scanTimeDelta int, logger golog.Logger) error {
	for {
		if !utils.SelectContextOrWait(ctx, time.Duration(math.Max(1, float64(scanTimeDelta)))*time.Millisecond) {
			return multierr.Combine(ctx.Err(), contextCloser.Close(ctx))
		}

		pc, err := rplidar.NextPointCloud(ctx)
		if err != nil {
			if err.Error() == "bad scan: OpTimeout" {
				logger.Warnf("Skipping this scan due to error: %v", err)
				continue
			} else {
				return multierr.Combine(err, contextCloser.Close(ctx))
			}
		}
		if err = savePCDFile(dataFolder, time.Now(), pc); err != nil {
			return err
		}

		logger.Infow("scanned", "pointcloud_size", pc.Size())
	}
}

func savePCDFile(dataFolder string, timeStamp time.Time, pc pointcloud.PointCloud) error {
	f, err := os.Create(dataFolder + "/rplidar_data_" + timeStamp.UTC().Format(time.RFC3339Nano) + ".pcd")
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

func getTimeDeltaMs(scanTimeDelta, defaultTimeDeltaMs int, logger golog.Logger) int {
	// Based on empirical data, we can see that the rplidar collects data at a rate of 15Hz,
	// which is ~ 66ms per scan. This issues a warning to the user, in case they're expecting
	// to receive data at a higher rate than what is technically possible.
	if scanTimeDelta == 0 {
		logger.Debugf("using default time delta %d ", defaultTimeDeltaMs)
		return defaultTimeDeltaMs
	}
	logger.Debugf("using user defined time delta %d ", scanTimeDelta)

	var estimatedTimePerScan int = 66
	if scanTimeDelta < estimatedTimePerScan {
		logger.Warnf("the expected scan rate of deltaT=%v is too small, has to be at least %v", scanTimeDelta, estimatedTimePerScan)
	}
	return scanTimeDelta
}

func getDataFolder(dataFolder string, logger golog.Logger) (string, error) {
	if dataFolder == "" {
		logger.Debugf("using default data folder '%s' ", defaultDataFolder)
		dataFolder = defaultDataFolder
	} else {
		logger.Debugf("using user defined data folder %s", dataFolder)
	}

	if err := os.MkdirAll(filepath.Join(".", dataFolder), os.ModePerm); err != nil {
		return "", errors.New("can not create a new directory named: " + dataFolder)
	}
	return dataFolder, nil
}

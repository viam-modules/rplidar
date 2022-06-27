package main

import (
	"context"
	"errors"

	"math"
	"time"

	"go.uber.org/multierr"
	"go.viam.com/rplidar"
	"go.viam.com/rplidar/helper"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/utils"

	robotimpl "go.viam.com/rdk/robot/impl"
)

var (
	defaultTimeDeltaMilliseconds = 100
	defaultDataFolder            = "data"
	logger                       = rlog.Logger.Named("save_pcd_files")
	name                         = "rplidar"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

// Arguments for the command.
type Arguments struct {
	TimeDeltaMilliseconds int    `flag:"delta,usage=delta ms"`
	DevicePath            string `flag:"device,usage=device path"`
	DataFolder            string `flag:"datafolder,usage=datafolder path"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments

	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	scanTimeDelta := helper.GetTimeDeltaMilliseconds(argsParsed.TimeDeltaMilliseconds, defaultTimeDeltaMilliseconds, logger)

	lidarDevice, err := helper.CreateRplidarComponent(name,
		rplidar.ModelName,
		argsParsed.DevicePath,
		argsParsed.DataFolder,
		defaultDataFolder,
		camera.SubtypeName,
		logger)
	if err != nil {
		return err
	}

	ctx = context.Background() // , err = helper.GetServiceContext(ctx)
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

	return savePCDFiles(ctx, myRobot, rplidar, scanTimeDelta, logger)
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

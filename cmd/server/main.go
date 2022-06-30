package main

import (
	"context"
	"fmt"

	"go.viam.com/rplidar"
	"go.viam.com/rplidar/helper"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rlog"
	"go.viam.com/utils"

	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
)

var (
	defaultDataFolder = "data"
	logger            = rlog.Logger.Named("server")
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

// Arguments for the command.
type Arguments struct {
	Port       utils.NetPortFlag `flag:"0"`
	DevicePath string            `flag:"device,usage=device path"`
	DataFolder string            `flag:"datafolder,usage=datafolder path"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments

	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	argsParsed.Port = helper.GetPort(argsParsed.Port, utils.NetPortFlag(rplidar.DefaultPort), logger)

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

	return runServer(ctx, int(argsParsed.Port), lidarDevice, logger)
}

func runServer(ctx context.Context, port int, lidarDevice config.Component, logger golog.Logger) (err error) {

	cfg := &config.Config{
		Components: []config.Component{lidarDevice},
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				BindAddress: fmt.Sprintf("localhost:%v", port),
			},
		},
	}

	myRobot, err := robotimpl.RobotFromConfig(ctx, cfg, logger)
	if err != nil {
		return err
	}

	options, err := weboptions.FromConfig(cfg)
	if err != nil {
		return err
	}
	options.Pprof = true
	return web.RunWeb(ctx, myRobot, options, logger)
}

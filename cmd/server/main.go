package main

import (
	"context"

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
	defaultPort       = 8081
	defaultDataFolder = "data"
	logger            = rlog.Logger.Named("server")
	name              = "rplidar"
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
	argsParsed.Port = helper.GetPort(argsParsed.Port, utils.NetPortFlag(defaultPort), logger)

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
	ctx = context.Background() // , err = helper.GetServiceContext(ctx)
	if err != nil {
		return err
	}

	cfg := &config.Config{Components: []config.Component{lidarDevice}}
	//myRobot, err := robotimpl.New(ctx, cfg, logger, client.WithDialOptions(rpc.WithInsecure()))
	myRobot, err := robotimpl.RobotFromConfig(ctx, cfg, logger)
	if err != nil {
		return err
	}

	// options := web.NewOptions()
	// options.Network = config.NetworkConfig{BindAddress: fmt.Sprintf(":%d", port)}
	options, err := weboptions.FromConfig(cfg)
	if err != nil {
		return err
	}
	options.Pprof = true
	return web.RunWeb(ctx, myRobot, options, logger)
}

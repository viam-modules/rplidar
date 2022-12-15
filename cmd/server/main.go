package main

import (
	"context"
	"fmt"

	"go.viam.com/rplidar"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"

	"go.viam.com/utils"

	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"

	viamutils "go.viam.com/utils"
)

var (
	logger = golog.NewLogger("server")
	name   = "rplidar"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

// Arguments for the command.
type Arguments struct {
	Port       utils.NetPortFlag `flag:"0"`
	DevicePath string            `flag:"device,usage=device path"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments

	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	argsParsed.Port = getPort(argsParsed.Port, utils.NetPortFlag(rplidar.DefaultPort), logger)

	lidarDevice, err := rplidar.CreateRplidarComponent(name,
		rplidar.ModelName,
		argsParsed.DevicePath,
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

	defer func() {
		rpl, err := camera.FromRobot(myRobot, name)
		if err != nil {
			logger.Errorf("failed to get rplidar from robot: %s", err)
		}
		if err = viamutils.TryClose(ctx, rpl); err != nil {
			logger.Errorf("failed to close rplidar: %s", err)
		}
	}()

	return web.RunWeb(ctx, myRobot, options, logger)
}

func getPort(port utils.NetPortFlag, defaultPort utils.NetPortFlag, logger golog.Logger) utils.NetPortFlag {
	if port == 0 {
		logger.Debugf("using default port %d ", defaultPort)
		return defaultPort
	} else {
		logger.Debugf("using user defined port %d ", port)
	}
	return port
}

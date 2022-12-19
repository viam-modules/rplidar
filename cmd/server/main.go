package main

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rplidar"

	"go.viam.com/utils"

	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"

	viamutils "go.viam.com/utils"
)

const (
	name        = "rplidar"
	defaultPort = 4444
)

func main() {
	utils.ContextualMain(mainWithArgs, golog.NewLogger("server"))
}

// Arguments for the command.
type Arguments struct {
	DevicePath string `flag:"device,usage=device path"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments

	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	rplidarComponent := config.Component{
		Name:      name,
		Namespace: resource.ResourceNamespaceRDK,
		Type:      camera.SubtypeName,
		Model:     rplidar.Model,
		Attributes: config.AttributeMap{
			"device_path": argsParsed.DevicePath,
		},
	}

	return runServer(ctx, rplidarComponent, logger)
}

func runServer(ctx context.Context, lidarDevice config.Component, logger golog.Logger) error {

	cfg := &config.Config{
		Components: []config.Component{lidarDevice},
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				BindAddress: fmt.Sprintf("localhost:%v", defaultPort),
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

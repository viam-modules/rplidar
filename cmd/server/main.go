package main

import (
	"context"
	"errors"
	"fmt"

	"go.viam.com/rdk/services/web"
	"go.viam.com/rplidar"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/metadata/service"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/usb"
	"go.viam.com/utils"

	"go.viam.com/rdk/grpc/client"
	robotimpl "go.viam.com/rdk/robot/impl"
	webserver "go.viam.com/rdk/web/server"
	"go.viam.com/utils/rpc"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

var (
	defaultPort = 8080
	logger      = rlog.Logger.Named("server")
)

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

	if argsParsed.Port == 0 {
		golog.Global.Debugf("using default port %d ", defaultPort)
		argsParsed.Port = utils.NetPortFlag(defaultPort)
	}

	// Check if USB device is available
	// TODO: add search filter for product and model info to confirm rplidar is present instead of assuming
	usbDevices := usb.Search(
		usb.SearchFilter{},
		func(vendorID, productID int) bool {
			return true
		})

	if len(usbDevices) != 0 {
		golog.Global.Debugf("detected %d lidar devices", len(usbDevices))
		for _, comp := range usbDevices {
			golog.Global.Debug(comp)
		}
	} else {
		return errors.New("no usb devices found")
	}

	// Create rplidar component
	lidarDevice := config.Component{
		Name:       "rplidar",
		Type:       config.ComponentTypeCamera,
		Model:      rplidar.ModelName,
		Attributes: config.AttributeMap{"device_path": "/dev/ttyUSB0"},
	}

	// Add device path if specified
	if argsParsed.DevicePath != "" {
		lidarDevice.Attributes = config.AttributeMap{"device_path": argsParsed.DevicePath}
	}

	return runServer(ctx, int(argsParsed.Port), lidarDevice, logger)
}

func runServer(ctx context.Context, port int, lidarComponent config.Component, logger golog.Logger) (err error) {

	metadataSvc, err := service.New()
	if err != nil {
		return err
	}
	ctx = service.ContextWithService(ctx, metadataSvc)

	cfg := &config.Config{Components: []config.Component{lidarComponent}}
	myRobot, err := robotimpl.New(ctx, cfg, logger, client.WithDialOptions(rpc.WithInsecure()))
	if err != nil {
		return err
	}

	options := web.NewOptions()
	options.Network = config.NetworkConfig{BindAddress: fmt.Sprintf(":%d", port)}
	return webserver.RunWeb(ctx, myRobot, options, logger)
}

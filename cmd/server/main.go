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

var (
	defaultPort = 8081
	logger      = rlog.Logger.Named("server")
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

	if argsParsed.Port == 0 {
		logger.Debugf("using default port %d ", defaultPort)
		argsParsed.Port = utils.NetPortFlag(defaultPort)
	}

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
	} else {
		return errors.New("no usb devices found")
	}

	// Create rplidar component
	lidarDevice := config.Component{
		Name:       "rplidar",
		Type:       config.ComponentTypeCamera,
		Model:      rplidar.ModelName,
		Attributes: config.AttributeMap{"device_path": usbDevices[0].Path},
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

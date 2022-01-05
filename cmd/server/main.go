package main

import (
	"context"
	"errors"

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

	_ "go.viam.com/rplidar/serial" //register
)

type closeable interface {
	Close(ctx context.Context) error
}

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

var (
	defaultPort = 4444
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

	usbDevices := usb.Search(
		usb.SearchFilter{},
		func(vendorID, productID int) bool {
			return true
		})

	//filter := serial.SearchFilter{} //Type: serial.TypeUnknown usb.NewSearchFilter()
	//lidarComponents := serial.Search(filter)
	//fmt.Printf("Serial DEVICES: %v \n", lidarComponents)

	if len(usbDevices) != 0 {
		golog.Global.Debugf("detected %d lidar devices", len(usbDevices))
		for _, comp := range usbDevices {
			golog.Global.Debug(comp)
		}
	} else {
		return errors.New("no usb devices found")
	}

	lidarDevices := []config.Component{}

	if argsParsed.DevicePath != "" {
		lidarDevices = []config.Component{{Type: config.ComponentTypeCamera, Model: rplidar.ModelName, Attributes: config.AttributeMap{"device_path": argsParsed.DevicePath}, Name: "rplidar"}}
	} else {
		lidarDevices = []config.Component{{Type: config.ComponentTypeCamera, Model: rplidar.ModelName, Attributes: config.AttributeMap{"device_path": "/dev/ttyUSB0"}, Name: "rplidar"}}
	}

	if len(lidarDevices) == 0 {
		return errors.New("no lidar devices found")
	}

	lidarDevice := lidarDevices[0]
	if lidarDevice.Model != rplidar.ModelName {
		return errors.New("device is not rplidar")
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
	defer myRobot.Close(ctx)

	options := web.NewOptions()
	options.Network = config.NetworkConfig{BindAddress: ":4444"}
	return webserver.RunWeb(ctx, myRobot, options, logger)
}

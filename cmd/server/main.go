package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"go.viam.com/rplidar"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/usb"
	"go.viam.com/utils"

	grpcserver "go.viam.com/rdk/grpc/server"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	pb "go.viam.com/rdk/proto/api/v1"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/subtype"
	"go.viam.com/utils/rpc"
	rpcserver "go.viam.com/utils/rpc"

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
		lidarDevices = []config.Component{{Type: config.ComponentTypeCamera, Model: rplidar.ModelName, Host: argsParsed.DevicePath, Name: "fuckinganything"}}
	} else {
		lidarDevices = []config.Component{{Type: config.ComponentTypeCamera, Model: rplidar.ModelName, Host: "/dev/ttyUSB0", Name: "fuckinganything"}}
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
	r, err := robotimpl.New(ctx, &config.Config{Components: []config.Component{lidarComponent}}, logger)
	if err != nil {
		return err
	}
	fmt.Println(lidarComponent)
	cameraDevice, ok := r.CameraByName(r.CameraNames()[0])
	if !ok {
		// some error
		panic("wtf")
	}

	defer func() {
		closeableCamera, ok := cameraDevice.(closeable)
		if !ok {
			// some error
			panic("wtf2")
		} else {
			closeableCamera.Close(ctx)
		}
	}()

	fmt.Println("HEYHO THE WATCHER GOES")

	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return err
	}

	rpcServer, err := rpcserver.NewServer(logger, rpc.WithUnauthenticated())
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, rpcServer.Stop())
	}()

	robotServer := grpcserver.New(r)
	fmt.Println(robotServer.Config(context.Background(), &pb.ConfigRequest{}))

	if err := rpcServer.RegisterServiceServer(
		ctx,
		&pb.RobotService_ServiceDesc,
		grpcserver.New(r),
		pb.RegisterRobotServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}

	cameras := map[resource.Name]interface{}{
		camera.Named("fuckinganything"): cameraDevice,
	}
	stype, err := subtype.New(cameras)
	if err != nil {
		return err
	}

	if err := rpcServer.RegisterServiceServer(
		ctx,
		&componentpb.CameraService_ServiceDesc,
		camera.NewServer(stype),
		componentpb.RegisterCameraServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		if err := rpcServer.Stop(); err != nil {
			panic(err)
		}

	}()

	//fmt.Println("HEYHO THE WATCHER GOES")

	utils.ContextMainReadyFunc(ctx)()
	err = rpcServer.Serve(listener)
	fmt.Printf("Error: %v \n", err)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := ctx.Err()
		if err != nil {
			// cancelled
			return err
		}

		if !utils.SelectContextOrWait(ctx, time.Second) {
			return nil
		}
	}
}

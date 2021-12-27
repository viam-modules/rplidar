package main

import (
	"context"
	"errors"
	"fmt"
	"net"

	"go.viam.com/rplidar"

	_ "go.viam.com/rplidar/serial" //register

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/rdk/config"
	grpcserver "go.viam.com/rdk/grpc/server"
	"go.viam.com/rdk/serial"

	//"go.viam.com/core/lidar/search"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rlog"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/utils"
	rpcserver "go.viam.com/utils/rpc"
)

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
		argsParsed.Port = utils.NetPortFlag(defaultPort)
	}

	filter := serial.SearchFilter{Type: serial.TypeUnknown} //usb.NewSearchFilter()
	lidarComponents := serial.Search(filter)
	if len(lidarComponents) != 0 {
		golog.Global.Debugf("detected %d lidar devices", len(lidarComponents))
		for _, comp := range lidarComponents {
			golog.Global.Debug(comp)
		}
	}

	lidarDevices := []config.Component{}

	if argsParsed.DevicePath != "" {
		lidarDevices = []config.Component{{Type: config.ComponentTypeCamera, Model: rplidar.ModelName, Host: argsParsed.DevicePath}}
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

	//cameraDevice, _ := r.CameraByName(r.CameraNames()[0])

	// defer func() {
	// 	err = multierr.Combine(err, cameraDevice.Close(ctx))
	// }()

	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return err
	}

	rpcServer, err := rpcserver.NewServer(logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, rpcServer.Stop())
	}()

	if err := rpcServer.RegisterServiceServer(
		ctx,
		&pb.RobotService_ServiceDesc,
		grpcserver.New(r),
		pb.RegisterRobotServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		if err := rpcServer.Stop(); err != nil {
			panic(err)
		}
	}()
	utils.ContextMainReadyFunc(ctx)()
	return rpcServer.Serve(listener)
}

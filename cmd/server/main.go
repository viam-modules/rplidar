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
	"go.viam.com/core/config"
	"go.viam.com/core/grpc/server"
	"go.viam.com/core/lidar/search"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rlog"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/rpc"
	"go.viam.com/core/utils"
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

	lidarComponents := search.Devices()
	if len(lidarComponents) != 0 {
		golog.Global.Debugf("detected %d lidar devices", len(lidarComponents))
		for _, comp := range lidarComponents {
			golog.Global.Debug(comp)
		}
	}
	if argsParsed.DevicePath != "" {
		lidarComponents = []config.Component{{Type: config.ComponentTypeLidar, Model: rplidar.ModelName, Host: argsParsed.DevicePath}}
	}

	if len(lidarComponents) == 0 {
		return errors.New("no lidar devices found")
	}

	lidarComponent := lidarComponents[0]
	if lidarComponent.Model != rplidar.ModelName {
		return errors.New("device is not rplidar")
	}

	return runServer(ctx, int(argsParsed.Port), lidarComponent, logger)
}

func runServer(ctx context.Context, port int, lidarComponent config.Component, logger golog.Logger) (err error) {
	r, err := robotimpl.NewRobot(ctx, &config.Config{Components: []config.Component{lidarComponent}}, logger)
	if err != nil {
		return err
	}
	lidarDevice := r.LidarByName(r.LidarNames()[0])

	info, err := lidarDevice.Info(ctx)
	if err != nil {
		return err
	}
	golog.Global.Infow("rplidar", "info", info)
	defer func() {
		err = multierr.Combine(err, lidarDevice.Stop(context.Background()))
	}()

	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return err
	}

	rpcServer, err := rpc.NewServer()
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, rpcServer.Stop())
	}()

	if err := rpcServer.RegisterServiceServer(
		ctx,
		&pb.RobotService_ServiceDesc,
		server.New(r),
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

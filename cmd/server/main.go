package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"go.viam.com/rplidar"

	_ "go.viam.com/rplidar/serial" //register

	"github.com/edaniels/golog"
	"github.com/edaniels/wsapi"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/lidar/search"
)

func main() {
	var devicePath string
	flag.StringVar(&devicePath, "device", "", "device path")
	flag.Parse()

	port := 4444
	if flag.NArg() >= 1 {
		portParsed, err := strconv.ParseInt(flag.Arg(0), 10, 32)
		if err != nil {
			golog.Global.Fatal(err)
		}
		port = int(portParsed)
	}

	deviceDescs, err := search.Devices()
	if err != nil {
		golog.Global.Debugw("error searching for lidar devices", "error", err)
	}
	if len(deviceDescs) != 0 {
		golog.Global.Debugf("detected %d lidar devices", len(deviceDescs))
		for _, desc := range deviceDescs {
			golog.Global.Debugf("%s (%s)", desc.Type, desc.Path)
		}
	}
	if devicePath != "" {
		deviceDescs = []lidar.DeviceDescription{{Type: rplidar.DeviceType, Path: devicePath}}
	}

	if len(deviceDescs) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	deviceDesc := deviceDescs[0]
	if deviceDesc.Type != rplidar.DeviceType {
		golog.Global.Fatal("device is not rplidar")
	}

	lidarDevice, err := lidar.CreateDevice(context.Background(), deviceDesc)
	if err != nil {
		golog.Global.Fatal(err)
	}
	info, err := lidarDevice.Info(context.Background())
	if err != nil {
		golog.Global.Fatal(err)
	}
	golog.Global.Infow("rplidar", "info", info)
	defer func() {
		if err := lidarDevice.Stop(context.Background()); err != nil {
			golog.Global.Errorw("error stopping lidar device", "error", err)
		}
	}()

	httpServer := &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	wsServer := wsapi.NewServer()
	registerCommands(wsServer, lidarDevice)
	httpServer.Handler = wsServer.HTTPHandler()

	errChan := make(chan error, 1)
	go func() {
		golog.Global.Infow("listening", "url", fmt.Sprintf("http://localhost:%d", port), "port", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	select {
	case err := <-errChan:
		golog.Global.Errorw("failed to serve", "error", err)
	case <-sig:
	}

	if err := httpServer.Shutdown(context.Background()); err != nil {
		golog.Global.Fatal(err)
	}
}

func registerCommands(server wsapi.Server, lidarDev lidar.Device) {
	server.RegisterCommand(lidar.WSCommandInfo, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return lidarDev.Info(ctx)
	}))
	server.RegisterCommand(lidar.WSCommandStart, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, lidarDev.Start(ctx)
	}))
	server.RegisterCommand(lidar.WSCommandStop, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, lidarDev.Stop(ctx)
	}))
	server.RegisterCommand(lidar.WSCommandClose, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return nil, lidarDev.Close(ctx)
	}))
	server.RegisterCommand(lidar.WSCommandScan, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return lidarDev.Scan(ctx, lidar.ScanOptions{})
	}))
	server.RegisterCommand(lidar.WSCommandRange, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return lidarDev.Range(ctx)
	}))
	server.RegisterCommand(lidar.WSCommandBounds, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return lidarDev.Bounds(ctx)
	}))
	server.RegisterCommand(lidar.WSCommandAngularResolution, wsapi.CommandHandlerFunc(func(ctx context.Context, cmd *wsapi.Command) (interface{}, error) {
		return lidarDev.AngularResolution(ctx)
	}))
}

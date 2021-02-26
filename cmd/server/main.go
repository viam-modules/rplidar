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

	"github.com/viamrobotics/rplidar"
	rplidarws "github.com/viamrobotics/rplidar/ws"

	rplidarserial "github.com/viamrobotics/rplidar/serial"

	"github.com/edaniels/golog"
	"github.com/edaniels/wsapi"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/lidar/search"
	"nhooyr.io/websocket"
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
	if rpl, ok := lidarDevice.(*rplidarserial.Device); ok {
		info := rpl.Info()
		golog.Global.Infow("rplidar",
			"dev_path", deviceDesc.Path,
			"model", info.Model,
			"serial", info.SerialNumber,
			"firmware_ver", info.FirmwareVersion,
			"hardware_rev", info.HardwareRevision)
	}
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

	httpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			golog.Global.Error("error making websocket connection", "error", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		for {
			select {
			case <-r.Context().Done():
				return
			default:
			}

			cmd, err := wsapi.ReadCommand(r.Context(), conn)
			if err != nil {
				golog.Global.Errorw("error reading command", "error", err)
				return
			}
			result, err := processCommand(r.Context(), cmd, lidarDevice.(*rplidarserial.Device))
			if err != nil {
				resp := wsapi.NewErrorResponse(err)
				if err := wsapi.WriteJSONResponse(r.Context(), resp, conn); err != nil {
					golog.Global.Errorw("error writing", "error", err)
					continue
				}
				continue
			}
			if err := wsapi.WriteJSONResponse(r.Context(), wsapi.NewSuccessfulResponse(result), conn); err != nil {
				golog.Global.Errorw("error writing", "error", err)
				continue
			}
		}
	})

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

func processCommand(ctx context.Context, cmd *wsapi.Command, lidarDev *rplidarserial.Device) (interface{}, error) {
	switch cmd.Name {
	case rplidarws.CommandInfo:
		return lidarDev.Info(), nil
	case rplidarws.CommandStart:
		return nil, lidarDev.Start(ctx)
	case rplidarws.CommandStop:
		return nil, lidarDev.Stop(ctx)
	case rplidarws.CommandClose:
		return nil, lidarDev.Close(ctx)
	case rplidarws.CommandScan:
		return lidarDev.Scan(ctx, lidar.ScanOptions{})
	case rplidarws.CommandRange:
		return lidarDev.Range(ctx)
	case rplidarws.CommandBounds:
		return lidarDev.Bounds(ctx)
	case rplidarws.CommandAngularResolution:
		return lidarDev.AngularResolution(ctx)
	default:
		return nil, fmt.Errorf("unknown command %s", cmd.Name)
	}
}

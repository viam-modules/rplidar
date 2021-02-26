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

	"github.com/edaniels/golog"
	"github.com/edaniels/wsapi"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/lidar/search"
	"go.viam.com/rplidar"
	rplidarserial "go.viam.com/rplidar/serial"
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
				resp := wsapi.NewErrorCommandResponse(err)
				if err := wsapi.WriteJSONCommandResponse(r.Context(), resp, conn); err != nil {
					golog.Global.Errorw("error writing", "error", err)
					continue
				}
				continue
			}
			if err := wsapi.WriteJSONCommandResponse(r.Context(), wsapi.NewSuccessfulCommandResponse(result), conn); err != nil {
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

func processCommand(ctx context.Context, cmd *wsapi.Command, lidarDev lidar.Device) (interface{}, error) {
	switch cmd.Name {
	case lidar.WSCommandInfo:
		return lidarDev.Info(ctx)
	case lidar.WSCommandStart:
		return nil, lidarDev.Start(ctx)
	case lidar.WSCommandStop:
		return nil, lidarDev.Stop(ctx)
	case lidar.WSCommandClose:
		return nil, lidarDev.Close(ctx)
	case lidar.WSCommandScan:
		return lidarDev.Scan(ctx, lidar.ScanOptions{})
	case lidar.WSCommandRange:
		return lidarDev.Range(ctx)
	case lidar.WSCommandBounds:
		return lidarDev.Bounds(ctx)
	case lidar.WSCommandAngularResolution:
		return lidarDev.AngularResolution(ctx)
	default:
		return nil, fmt.Errorf("unknown command %s", cmd.Name)
	}
}

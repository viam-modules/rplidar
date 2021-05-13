package main

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/core/grpc/client"
	"go.viam.com/core/lidar"
	"go.viam.com/core/rlog"
	"go.viam.com/core/utils"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

var logger = rlog.Logger.Named("client")

// Arguments for the command.
type Arguments struct {
	DeviceAddress string `flag:"device,required,default=localhost:4444,usage=device address"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	return runClient(ctx, argsParsed.DeviceAddress, logger)
}

func runClient(ctx context.Context, deviceAddress string, logger golog.Logger) (err error) {
	robotClient, err := client.NewRobotClient(ctx, deviceAddress, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, robotClient.Close())
	}()
	names := robotClient.LidarNames()
	if len(names) == 0 {
		return errors.New("no lidar devices found")
	}
	lidarDevice := robotClient.LidarByName(names[0])

	if err := lidarDevice.Start(ctx); err != nil {
		return err
	}

	defer func() {
		err = multierr.Combine(err, lidarDevice.Stop(context.Background()))
	}()

	info, err := lidarDevice.Info(ctx)
	if err != nil {
		return err
	}
	logger.Infow("lidar", "info", info)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	utils.ContextMainReadyFunc(ctx)()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		measurements, err := lidarDevice.Scan(context.Background(), lidar.ScanOptions{})
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		logger.Infow("scanned", "num_measurements", len(measurements))
	}
}

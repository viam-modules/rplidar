package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"

	_ "go.viam.com/rdk/component/camera/register"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/rlog"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

var logger = rlog.Logger.Named("client")

// Arguments for the command.
type Arguments struct {
	DeviceAddress string `flag:"device,required,default=localhost:8081,usage=device address"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	return runClient(ctx, argsParsed.DeviceAddress, logger)
}

func runClient(ctx context.Context, deviceAddress string, logger golog.Logger) (err error) {

	robotClient, err := client.New(ctx, deviceAddress, logger, client.WithDialOptions(rpc.WithInsecure()))
	if err != nil {
		return err
	}

	defer func() {
		err = multierr.Combine(err, robotClient.Close(ctx))
	}()

	cameraDevice, ok := robotClient.CameraByName(robotClient.CameraNames()[0])
	if !ok {
		return fmt.Errorf("failed to find component")
	}

	// Run loop
	ticker := time.NewTicker(5 * time.Second)
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

		pc, err := cameraDevice.NextPointCloud(context.Background())
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		logger.Infow("scanned", "pointcloud_size", pc.Size())
	}
}

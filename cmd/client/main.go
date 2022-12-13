package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"go.uber.org/multierr"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/camera"
	_ "go.viam.com/rdk/components/camera/register"
	"go.viam.com/rdk/robot/client"

	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

var logger = golog.NewLogger("client")

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

func runClient(ctx context.Context, deviceAddress string, logger golog.Logger) error {

	robotClient, err := client.New(ctx, deviceAddress, logger, client.WithDialOptions(rpc.WithInsecure()))
	if err != nil {
		return err
	}

	defer func() {
		err = multierr.Combine(err, robotClient.Close(ctx))
	}()

	res, err := robotClient.ResourceByName(camera.Named("rplidar"))
	if err != nil {
		return fmt.Errorf("failed to find component")
	}

	cameraDevice := res.(camera.Camera)

	// Run loop
	for {
		if !utils.SelectContextOrWait(ctx, 5*time.Second) {
			return ctx.Err()
		}

		pc, err := cameraDevice.NextPointCloud(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		logger.Infow("scanned", "pointcloud_size", pc.Size())
	}
}

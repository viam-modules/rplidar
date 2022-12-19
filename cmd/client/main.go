package main

import (
	"context"
	"io"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/camera"
	_ "go.viam.com/rdk/components/camera/register"
	"go.viam.com/rdk/robot/client"

	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
)

func main() {
	utils.ContextualMain(mainWithArgs, golog.NewLogger("client"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	return runClient(ctx, logger)
}

func runClient(ctx context.Context, logger golog.Logger) error {

	// Connect to the default localhost port for the rplidar.
	robotClient, err := client.New(
		ctx,
		"localhost:4444",
		logger,
		client.WithDialOptions(rpc.WithInsecure()))
	if err != nil {
		return err
	}

	defer func() {
		err = multierr.Combine(err, robotClient.Close(ctx))
	}()

	res, err := robotClient.ResourceByName(camera.Named("rplidar"))
	if err != nil {
		return errors.Wrap(err, "failed to find component")
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

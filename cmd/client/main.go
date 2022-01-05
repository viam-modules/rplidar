package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"

	"go.viam.com/rdk/component/camera"
	_ "go.viam.com/rdk/component/camera/register"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rlog"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
)

// type closeable interface {
// 	Close() error
// }

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
	fmt.Printf("deviceAddress: %v\n", deviceAddress)
	robotClient, err := client.New(ctx, deviceAddress, logger, client.WithDialOptions(rpc.WithInsecure()))

	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, robotClient.Close(ctx))
	}()

	fmt.Printf("ResourceNames: %v \n", robotClient.ResourceNames())
	fmt.Printf("CameraNames: %v \n", robotClient.CameraNames())
	fmt.Printf("CameraDevice: %v \n", robotClient.CameraNames()[0])
	fmt.Printf("robotClient: %v \n", robotClient)
	name := camera.Named(robotClient.CameraNames()[0])
	fmt.Printf("camera named: %v \n", name)
	fmt.Printf("subtype named: %v \n", name.Subtype)
	fmt.Printf("registry: %v \n", registry.ResourceSubtypeLookup(name.Subtype))

	cameraDevice, ok := robotClient.CameraByName(robotClient.CameraNames()[0])
	if !ok {
		return fmt.Errorf("failure to find component, nil pointer")
	}
	fmt.Println(cameraDevice)

	fmt.Println("HEYHO THE SEEKER COMES")

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

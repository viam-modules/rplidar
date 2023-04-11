package main

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/module"
	"go.viam.com/rplidar"

	"go.viam.com/utils"
)

func main() {
	utils.ContextualMain(mainWithArgs, golog.NewLogger("rplidarModule"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	// Instantiate the module
	rplidarModule, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	// Add the rplidar model to the module
	if err = rplidarModule.AddModelFromRegistry(ctx, camera.Subtype, rplidar.Model); err != nil {
		return err
	}

	// Start the module
	err = rplidarModule.Start(ctx)
	defer rplidarModule.Close(ctx)
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}

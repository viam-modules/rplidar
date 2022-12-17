package main

import (
	"context"

	"go.viam.com/rplidar"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/module"

	"go.viam.com/utils"
)

func main() {
	utils.ContextualMain(mainWithArgs, golog.NewLogger("rplidarServer"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	// Instantiate the module itself
	rpModule, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	// Add the rplidar model to the module
	rpModule.AddModelFromRegistry(ctx, camera.Subtype, rplidar.Model)

	// Start the module
	err = rpModule.Start(ctx)
	defer rpModule.Close(ctx)
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}

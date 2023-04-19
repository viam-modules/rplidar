// Package main is a module with a rplidar component model.
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
	utils.ContextualMain(mainWithArgs, golog.NewDebugLogger("rplidarModule"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	logger.Debugf("ARGS %#v\n", args)
	// Instantiate the module itself
	rpModule, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	// Add the rplidar model to the module
	if err = rpModule.AddModelFromRegistry(ctx, camera.Subtype, rplidar.Model); err != nil {
		return err
	}

	// Start the module
	err = rpModule.Start(ctx)
	defer rpModule.Close(ctx)
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}

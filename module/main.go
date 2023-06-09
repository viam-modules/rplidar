// Package main is a module with a rplidar component model.
package main

import (
	"context"
	"strings"

	"go.viam.com/rplidar"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/module"

	"go.viam.com/utils"
)

// Versioning variables which are replaced by LD flags.
var (
	Version     = "development"
	GitRevision = ""
)

func main() {
	utils.ContextualMain(mainWithArgs, golog.NewLogger("rplidarModule"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var versionFields []interface{}
	if Version != "" {
		versionFields = append(versionFields, "version", Version)
	}
	if GitRevision != "" {
		versionFields = append(versionFields, "git_rev", GitRevision)
	}
	if len(versionFields) != 0 {
		logger.Infow(rplidar.Model.String(), versionFields...)
	} else {
		logger.Info(rplidar.Model.String() + " built from source; version unknown")
	}

	if len(args) == 2 && strings.HasSuffix(args[1], "-version") {
		return nil
	}

	// Instantiate the module itself
	rpModule, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	// Add the rplidar model to the module
	err = rpModule.AddModelFromRegistry(ctx, camera.API, rplidar.Model)
	if err != nil {
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

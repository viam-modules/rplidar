package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"time"

	rplidarws "github.com/viamrobotics/rplidar/ws"

	"github.com/edaniels/golog"
	"github.com/viamrobotics/robotcore/lidar"
)

func main() {
	port := 4444
	if flag.NArg() >= 1 {
		portParsed, err := strconv.ParseInt(flag.Arg(0), 10, 32)
		if err != nil {
			golog.Global.Fatal(err)
		}
		port = int(portParsed)
	}

	lidarDev, err := rplidarws.NewDevice(context.Background(), fmt.Sprintf("ws://localhost:%d", port))
	if err != nil {
		golog.Global.Fatal(err)
	}

	if rpl, ok := lidarDev.(*rplidarws.Device); ok {
		info, err := rpl.Info(context.Background())
		if err != nil {
			golog.Global.Fatal(err)
		}
		golog.Global.Infow("rplidar",
			"model", info.Model,
			"serial", info.SerialNumber,
			"firmware_ver", info.FirmwareVersion,
			"hardware_rev", info.HardwareRevision)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

READ:
	for {
		time.Sleep(time.Second)
		select {
		case <-sig:
			break READ
		default:
		}

		measurements, err := lidarDev.Scan(context.Background(), lidar.ScanOptions{})
		if err != nil {
			if errors.Is(err, io.EOF) {
				break READ
			}
			golog.Global.Fatal(err)
		}
		golog.Global.Infow("scanned", "num_measurements", len(measurements))
	}
}

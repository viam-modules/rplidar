package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"os/signal"
	"time"

	"go.viam.com/robotcore/lidar"

	"github.com/edaniels/golog"
)

func main() {
	var deviceAddress string
	flag.StringVar(&deviceAddress, "device", "ws://localhost:4444", "device ws address")
	flag.Parse()

	lidarDev, err := lidar.NewWSDevice(context.Background(), deviceAddress)
	if err != nil {
		golog.Global.Fatal(err)
	}

	info, err := lidarDev.Info(context.Background())
	if err != nil {
		golog.Global.Fatal(err)
	}
	golog.Global.Infow("lidar", "info", info)

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

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

	"go.viam.com/robotcore/lidar"

	"github.com/edaniels/golog"
)

func main() {
	flag.Parse()

	port := 4444
	if flag.NArg() >= 1 {
		portParsed, err := strconv.ParseInt(flag.Arg(0), 10, 32)
		if err != nil {
			golog.Global.Fatal(err)
		}
		port = int(portParsed)
	}

	lidarDev, err := lidar.NewWSDevice(context.Background(), fmt.Sprintf("ws://localhost:%d", port))
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

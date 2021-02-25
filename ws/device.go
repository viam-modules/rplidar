package rplidarws

import (
	"context"
	"fmt"
	"image"
	"math"

	"github.com/viamrobotics/rplidar"

	"github.com/edaniels/wsapi"
	"github.com/viamrobotics/robotcore/lidar"
	"nhooyr.io/websocket"
)

func init() {
	lidar.RegisterDeviceType(rplidar.DeviceType, lidar.DeviceTypeRegistration{
		New: func(ctx context.Context, desc lidar.DeviceDescription) (lidar.Device, error) {
			return NewDevice(ctx, fmt.Sprintf("ws://%s:%d", desc.Host, desc.Port))
		},
	})
}

const (
	CommandInfo              = "info"
	CommandStart             = "start"
	CommandStop              = "stop"
	CommandClose             = "close"
	CommandScan              = "scan"
	CommandRange             = "range"
	CommandBounds            = "bounds"
	CommandAngularResolution = "angular_resolution"
)

type Device struct {
	conn *websocket.Conn
}

func NewDevice(ctx context.Context, address string) (lidar.Device, error) {
	conn, _, err := websocket.Dial(ctx, address, nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(10 * (1 << 24))

	return &Device{conn}, nil
}

func (d *Device) Info(ctx context.Context) (*rplidar.DeviceInfo, error) {
	if err := wsapi.WriteCommand(ctx, wsapi.NewCommand(CommandInfo), d.conn); err != nil {
		return nil, err
	}
	var info rplidar.DeviceInfo
	err := wsapi.ReadJSONResponse(ctx, d.conn, &info)
	return &info, err
}

func (d *Device) Start(ctx context.Context) error {
	if err := wsapi.WriteCommand(ctx, wsapi.NewCommand(CommandStart), d.conn); err != nil {
		return err
	}
	return wsapi.ExpectResponse(ctx, d.conn)
}

func (d *Device) Stop(ctx context.Context) error {
	if err := wsapi.WriteCommand(ctx, wsapi.NewCommand(CommandStop), d.conn); err != nil {
		return err
	}
	return wsapi.ExpectResponse(ctx, d.conn)
}

func (d *Device) Close(ctx context.Context) error {
	defer d.conn.Close(websocket.StatusNormalClosure, "")
	if err := wsapi.WriteCommand(ctx, wsapi.NewCommand(CommandClose), d.conn); err != nil {
		return err
	}
	return wsapi.ExpectResponse(ctx, d.conn)
}

// TODO(erd): send options
func (d *Device) Scan(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
	if err := wsapi.WriteCommand(ctx, wsapi.NewCommand(CommandScan), d.conn); err != nil {
		return nil, err
	}
	var measurements lidar.Measurements
	err := wsapi.ReadJSONResponse(ctx, d.conn, &measurements)
	return measurements, err
}

func (d *Device) Range(ctx context.Context) (int, error) {
	if err := wsapi.WriteCommand(ctx, wsapi.NewCommand(CommandRange), d.conn); err != nil {
		return 0, err
	}
	var devRange int
	err := wsapi.ReadJSONResponse(ctx, d.conn, &devRange)
	return devRange, err
}

func (d *Device) Bounds(ctx context.Context) (image.Point, error) {
	if err := wsapi.WriteCommand(ctx, wsapi.NewCommand(CommandBounds), d.conn); err != nil {
		return image.Point{}, err
	}
	var bounds struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	err := wsapi.ReadJSONResponse(ctx, d.conn, &bounds)
	return image.Point(bounds), err
}

func (d *Device) AngularResolution(ctx context.Context) (float64, error) {
	if err := wsapi.WriteCommand(ctx, wsapi.NewCommand(CommandAngularResolution), d.conn); err != nil {
		return math.NaN(), err
	}
	var angRes float64
	err := wsapi.ReadJSONResponse(ctx, d.conn, &angRes)
	return angRes, err
}

package rplidar

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/test"
)

const (
	testExecutableName  = "true" // the program "true", not the boolean value
	testDataFreqHz      = "5"
	testIMUDataFreqHz   = "20"
	testLidarDataFreqHz = "5"
)

var (
	_zeroTime = time.Time{}
	_true     = true
	_false    = false
)

func TestValidate(t *testing.T) {

	t.Run("min range is zero", func(t *testing.T) {
		cfg := Config{
			MinRangeMM: 0,
		}

		deps, err := cfg.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(deps), test.ShouldEqual, 0)
	})
	t.Run("min range is greater than zero", func(t *testing.T) {
		cfg := Config{
			MinRangeMM: 1,
		}

		deps, err := cfg.Validate("")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(deps), test.ShouldEqual, 0)
	})
	t.Run("min range is less than zero", func(t *testing.T) {
		cfg := Config{
			MinRangeMM: -1,
		}

		deps, err := cfg.Validate("")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "min_range must be positive")
		test.That(t, len(deps), test.ShouldEqual, 0)
	})
}

func TestScan(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		description        string
		rp                 *Rplidar
		scanCount          int
		expectedErr        error
		expectedPointCloud pointcloud.PointCloud
	}{
		{
			description:        "rplidar driver that fails to grab new data but zero scans",
			rp:                 BadRplidarFailsToGrabScanData(),
			scanCount:          0,
			expectedErr:        nil,
			expectedPointCloud: nil,
		},
		{
			description:        "rplidar driver that fails to grab new data",
			rp:                 BadRplidarFailsToGrabScanData(),
			scanCount:          1,
			expectedErr:        errors.New("bad scan"),
			expectedPointCloud: nil,
		},
		{
			description:        "rplidar driver that is good",
			rp:                 GoodRplidarReturnsZeroPoints(),
			scanCount:          1,
			expectedErr:        nil,
			expectedPointCloud: nil,
		},
		// TODO: Add artifact test
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			pc, err := tt.rp.scan(ctx, tt.scanCount)
			if tt.expectedErr == nil {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tt.expectedErr.Error())
			}
			test.That(t, pc, test.ShouldEqual, tt.expectedPointCloud)
		})
	}
}

func TestScanArtifact(t *testing.T) {
	ctx := context.Background()

	rp := GoodRplidarReturnsZeroPoints()
	_, err := rp.scan(ctx, 1)
	test.That(t, err, test.ShouldBeNil)
}

func TestNextPointCloud(t *testing.T) {
	ctx := context.Background()
	rp := Rplidar{
		cacheMutex: sync.RWMutex{},
	}

	t.Run("returns nil pointcloud from cache", func(t *testing.T) {
		rp.cachedPointCloud = nil

		pc, err := rp.NextPointCloud(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldEqual, errors.New("pointcloud has not been saved yet").Error())
		test.That(t, pc, test.ShouldBeNil)
	})

	t.Run("returns empty pointcloud from cache", func(t *testing.T) {
		rp.cachedPointCloud = pointcloud.New()

		pc, err := rp.NextPointCloud(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pc, test.ShouldResemble, pointcloud.New())

	})

	t.Run("returns non-empty pointcloud from cache", func(t *testing.T) {
		cachedPointCloud := pointcloud.New()
		cachedPointCloud.Set(r3.Vector{X: 1, Y: 2, Z: 3}, pointcloud.NewBasicData())
		rp.cachedPointCloud = cachedPointCloud

		pc, err := rp.NextPointCloud(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pc, test.ShouldResemble, cachedPointCloud)
	})
}

func TestProperties(t *testing.T) {
	ctx := context.Background()
	rp := Rplidar{}

	prop, err := rp.Properties(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop, test.ShouldResemble, camera.Properties{SupportsPCD: true})
}

func TestClose(t *testing.T) {

	ctx := context.Background()
	rp := Rplidar{
		cancelFunc:             func() {},
		deviceMutex:            sync.Mutex{},
		cacheMutex:             sync.RWMutex{},
		cacheBackgroundWorkers: sync.WaitGroup{},
	}
	t.Run("no active background workers and or mutex blocking", func(t *testing.T) {
		err := rp.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("active background workers and no mutex blocking", func(t *testing.T) {
		rp.cacheBackgroundWorkers.Add(1)
		rp.cancelFunc = func() { rp.cacheBackgroundWorkers.Done() }

		err := rp.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("no active background workers and device mutex blocking", func(t *testing.T) {
		rp.cacheBackgroundWorkers = sync.WaitGroup{}
		rp.cancelFunc = func() {}
		rp.deviceMutex.Lock()
		go func() {
			time.Sleep(10 * time.Millisecond)
			rp.deviceMutex.Unlock()
		}()

		startTime := time.Now()
		err := rp.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, time.Since(startTime).Milliseconds(), test.ShouldBeGreaterThanOrEqualTo, 10)
	})

	t.Run("no active background workers and cache mutex blocking", func(t *testing.T) {
		rp.cacheBackgroundWorkers = sync.WaitGroup{}
		rp.cancelFunc = func() {}
		rp.cacheMutex.Lock()
		go func() {
			time.Sleep(10 * time.Millisecond)
			rp.cacheMutex.Unlock()
		}()

		startTime := time.Now()
		err := rp.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, time.Since(startTime).Milliseconds(), test.ShouldBeGreaterThanOrEqualTo, 10)
	})
}

func TestUnimplementedFunctions(t *testing.T) {
	ctx := context.Background()
	rp := Rplidar{}

	t.Run("unimplemented Images function", func(t *testing.T) {
		namedImage, metadata, err := rp.Images(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unimplemented")
		test.That(t, metadata, test.ShouldResemble, resource.ResponseMetadata{})
		test.That(t, namedImage, test.ShouldBeNil)
	})

	t.Run("unimplemented Projector function", func(t *testing.T) {
		proj, err := rp.Projector(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unimplemented")
		test.That(t, proj, test.ShouldBeNil)
	})

	t.Run("unimplemented Stream function", func(t *testing.T) {
		stream, err := rp.Stream(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unimplemented")
		test.That(t, stream, test.ShouldBeNil)
	})
}

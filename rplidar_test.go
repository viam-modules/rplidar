package rplidar

import (
	"context"
	"testing"
	"time"

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
	rp := BadScanRplidar()

	t.Run("rplidar driver that fails to grab new data but zero scans", func(t *testing.T) {
		pc, err := rp.scan(ctx, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pc, test.ShouldBeNil)
	})

	t.Run("rplidar driver that fails to grab new data", func(t *testing.T) {
		pc, err := rp.scan(ctx, 1)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "bad scan")
		test.That(t, pc, test.ShouldBeNil)
	})

	rp = GoodScanRplidar()

	t.Run("rplidar driver that is good", func(t *testing.T) {
		pc, err := rp.scan(ctx, 1)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "bad scan")
		test.That(t, pc, test.ShouldBeNil)
	})

}

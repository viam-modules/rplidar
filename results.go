// Package rplidar implements a general rplidar LIDAR as a camera.
package rplidar

import "go.viam.com/rplidar/gen"

type (
	// Result describes the status of an RPLidar operation.
	Result uint32

	// ResultError is a result that encodes an error.
	ResultError struct {
		Result
	}
)

// The set of possible results.
var (
	ResultOk                 = Result(gen.RESULT_OK)
	ResultAlreadyDone        = Result(gen.RESULT_ALREADY_DONE)
	ResultInvalidData        = Result(gen.RESULT_INVALID_DATA)
	ResultOpFail             = Result(gen.RESULT_OPERATION_FAIL)
	ResultOpTimeout          = Result(gen.RESULT_OPERATION_TIMEOUT)
	ResultOpStop             = Result(gen.RESULT_OPERATION_STOP)
	ResultOpNotSupported     = Result(gen.RESULT_OPERATION_NOT_SUPPORT)
	ResultFormatNotSupported = Result(gen.RESULT_FORMAT_NOT_SUPPORT)
	ResultInsufficientMemory = Result(gen.RESULT_INSUFFICIENT_MEMORY)
)

// Failed returns an error if the result is that of a failure.
func (r Result) Failed() error {
	if uint64(r)&gen.RESULT_FAIL_BIT == 0 {
		return nil
	}
	return ResultError{r}
}

// String returns a human readable version of a result.
func (r Result) String() string {
	switch r {
	case ResultOk:
		return "Ok"
	case ResultAlreadyDone:
		return "AlreadyDone"
	case ResultInvalidData:
		return "InvalidData"
	case ResultOpFail:
		return "OpFail"
	case ResultOpTimeout:
		return "OpTimeout"
	case ResultOpStop:
		return "OpStop"
	case ResultOpNotSupported:
		return "OpNotSupported"
	case ResultFormatNotSupported:
		return "FormatNotSupported"
	case ResultInsufficientMemory:
		return "InsufficientMemory"
	default:
		return "Unknown"
	}
}

// Error returns the error as a human readable string.
func (r ResultError) Error() string {
	return r.String()
}

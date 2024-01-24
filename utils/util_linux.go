// Package utils provides utility functions such as os specific int casting to handle swig mismatch between darwin and linux
package utils

// CastInt casts an int to int due to swig mismatch on linux (swig 4.2)
func CastInt(i int) int {
	return i
}

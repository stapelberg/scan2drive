//go:build !arm64
// +build !arm64

package main

func LocalScanner() {
	// Not implemented on non-arm64, as neonjpeg is only available on arm64 and
	// LocalScanner requires its streaming API.
}

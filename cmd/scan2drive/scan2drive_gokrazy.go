//go:build gokrazy
// +build gokrazy

package main

import "github.com/gokrazy/gokrazy"

func gokrazyInit() {
	gokrazy.WaitForClock()
}

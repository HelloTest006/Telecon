//go:build !windows

package main

import "fmt"

func runWindowsService(name string, run func()) error {
	return fmt.Errorf("Windows service mode not supported on this OS")
}

//go:build windows

package fileatomic

import "golang.org/x/sys/windows"

// Replace atomically publishes a staged file over an existing destination.
func Replace(source, destination string) error {
	return windows.Rename(source, destination)
}

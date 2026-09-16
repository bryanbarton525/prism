//go:build !windows

package fileatomic

import "os"

// Replace atomically publishes a staged file over an existing destination.
func Replace(source, destination string) error {
	return os.Rename(source, destination)
}

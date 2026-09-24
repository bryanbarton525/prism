//go:build windows

package filelock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

var errWouldBlock = windows.ERROR_LOCK_VIOLATION

func tryExclusive(file *os.File) (func() error, error) {
	overlapped := &windows.Overlapped{}
	err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlapped)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
		return nil, errWouldBlock
	}
	if err != nil {
		return nil, err
	}
	return func() error { return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, overlapped) }, nil
}

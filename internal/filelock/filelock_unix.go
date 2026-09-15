//go:build !windows

package filelock

import (
	"errors"
	"os"
	"syscall"
)

var errWouldBlock = syscall.EWOULDBLOCK

func tryExclusive(file *os.File) (func() error, error) {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EAGAIN) {
		return nil, errWouldBlock
	}
	if err != nil {
		return nil, err
	}
	return func() error { return syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }, nil
}

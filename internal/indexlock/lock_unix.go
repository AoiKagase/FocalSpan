//go:build linux || darwin

package indexlock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func tryLock(f *os.File, mode Mode) (bool, error) {
	operation := unix.LOCK_SH
	if mode == Exclusive {
		operation = unix.LOCK_EX
	}
	err := unix.Flock(int(f.Fd()), operation|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
		return false, nil
	}
	return err == nil, err
}

func unlock(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }

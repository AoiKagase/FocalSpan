//go:build windows

package benchcli

import "golang.org/x/sys/windows"

func publishOutputExclusive(from, to string) error {
	src, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	dst, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(src, dst, 0)
}

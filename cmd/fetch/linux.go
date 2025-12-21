//go:build linux
// +build linux

package fetch

import (
	"syscall"
)

func getControl(ifname string) func(network string, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var controlErr error
		err := c.Control(func(fd uintptr) {
			controlErr = syscall.BindToDevice(int(fd), ifname)
		})
		if err != nil {
			return err
		}
		return controlErr
	}
}

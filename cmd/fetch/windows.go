//go:build !linux
// +build !linux

package fetch

import (
	"syscall"
)

func getControl(ifname string) func(network string, address string, c syscall.RawConn) error {
	return nil
}

//go:build linux
// +build linux

package fetch

import (
	"syscall"

	log "github.com/sirupsen/logrus"
)

func getControl(ifname string) func(network string, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		c.Control(func(fd uintptr) {
			if err := syscall.BindToDevice(int(fd), ifname); err != nil {
				log.Errorf("bind socket to %s error: %v", ifname, err)
			}
		})
		return nil
	}
}

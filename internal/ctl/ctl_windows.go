//go:build windows

package ctl

import (
	"net"
	"time"

	winio "github.com/Microsoft/go-winio"
)

func ctlListen(socketPath string) (net.Listener, error) {
	return winio.ListenPipe(socketPath, nil)
}

func ctlDial(socketPath string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(socketPath, &timeout)
}

package ctl

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

func DialSocket(socketPath string, timeout time.Duration) (net.Conn, error) {
	return ctlDial(socketPath, timeout)
}

func SendCommand(socketPath, cmd string, args []string) (*CtlResult, error) {
	conn, err := ctlDial(socketPath, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("must is not running: %w", err)
	}
	defer func() { _ = conn.Close() }()

	req := struct {
		Cmd  string   `json:"cmd"`
		Args []string `json:"args"`
	}{Cmd: cmd, Args: args}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	var result CtlResult
	if err := json.NewDecoder(conn).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &result, nil
}

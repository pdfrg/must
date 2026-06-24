package ctl

import (
	"encoding/json"
	"net"
	"os"
	"runtime"
	"sync"

	tea "charm.land/bubbletea/v2"
)

type Server struct {
	listener net.Listener
	wg       sync.WaitGroup
	sockPath string
}

func StartServer(socketPath string, program *tea.Program) (*Server, error) {
	listener, err := ctlListen(socketPath)
	if err != nil {
		return nil, err
	}

	s := &Server{listener: listener, sockPath: socketPath}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			s.wg.Add(1)
			go func(c net.Conn) {
				defer s.wg.Done()
				defer func() { _ = c.Close() }()
				handleConn(c, program)
			}(conn)
		}
	}()

	return s, nil
}

func (s *Server) Stop() {
	_ = s.listener.Close()
	s.wg.Wait()
	if runtime.GOOS != "windows" {
		_ = os.Remove(s.sockPath)
	}
}

func handleConn(conn net.Conn, program *tea.Program) {
	var req struct {
		Cmd  string   `json:"cmd"`
		Args []string `json:"args"`
	}

	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}

	resultCh := make(chan CtlResult)
	program.Send(CtlMessage{
		Cmd:      req.Cmd,
		Args:     req.Args,
		ResultCh: resultCh,
	})

	result := <-resultCh
	_ = json.NewEncoder(conn).Encode(result)
}

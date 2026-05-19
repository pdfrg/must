package ctl

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"

	tea "charm.land/bubbletea/v2"
)

type Server struct {
	listener net.Listener
	wg       sync.WaitGroup
	sockPath string
}

func StartServer(socketPath string, program *tea.Program) (*Server, error) {
	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
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
	_ = os.Remove(s.sockPath)
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

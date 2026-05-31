package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
)

type agentRequest struct {
	VM     string `json:"vm"`
	Method string `json:"method"`
	Args   any    `json:"args,omitempty"`
}

type agentResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type vmExecutor interface {
	Execute(vmName, method string, args any) (json.RawMessage, error)
}

type Server struct {
	addr string
	vms  vmExecutor
}

func NewServer(addr string, vms vmExecutor) *Server {
	return &Server{addr: addr, vms: vms}
}

func (s *Server) Listen() error {
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.addr, err)
	}
	defer l.Close()

	log.Printf("agent listening on %s", s.addr)

	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accepting connection: %w", err)
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	for {
		var req agentRequest
		if err := dec.Decode(&req); err != nil {
			return
		}

		result, err := s.vms.Execute(req.VM, req.Method, req.Args)
		if err != nil {
			_ = enc.Encode(agentResponse{Error: err.Error()})
			continue
		}

		_ = enc.Encode(agentResponse{Result: result})
	}
}

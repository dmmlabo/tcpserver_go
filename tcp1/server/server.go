package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
)

const (
	TCP_PROTOCOL           = "tcp"
	LISTENER_CLOSE_MATCHER = "use of closed network connection"
)

type ServerError struct {
	S   *Server
	Op  string
	Err error
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("Server[%v] (%v): %v", e.S.Addr, e.Op, e.Err)
}

type Server struct {
	listener *net.TCPListener
	Addr     string
	ChClosed chan bool
	Wg       sync.WaitGroup
}

func NewServer(addr string) (*Server, error) {
	chClosed := make(chan bool)
	server := Server{
		Addr:     addr,
		ChClosed: chClosed,
	}
	return &server, nil
}

func (s *Server) StopListener() {
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			log.Println(&ServerError{s, "CloseListener", err})
		}
	}
}

func (s *Server) Listen(shutdownCtx, gshutdownCtx context.Context) (context.Context, error) {
	tcpAddr, err := net.ResolveTCPAddr(TCP_PROTOCOL, s.Addr)
	if err != nil {
		return nil, &ServerError{s, "ResolveTCPAddr", err}
	}

	l, err := net.ListenTCP(TCP_PROTOCOL, tcpAddr)
	if err != nil {
		return nil, &ServerError{s, "ListenTCP", err}
	}
	s.listener = l

	errCtx, errCancel := context.WithCancel(context.Background())

	go s.handleListener(shutdownCtx, gshutdownCtx, errCtx, errCancel)

	return errCtx, nil
}

func (s *Server) handleListener(shutdownCtx, gshutdownCtx, errCtx context.Context, errCancel context.CancelFunc) {
	defer func() {
		if err := s.listener.Close(); err != nil && !listenerCloseError(err) {
			log.Println(&ServerError{s, "CloseListener", err})
		}
		close(s.ChClosed)
	}()

	for {
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			if ne, ok := err.(*net.OpError); ok {
				if ne.Temporary() {
					log.Println(&ServerError{s, "AcceptTCP", err})
					continue
				}
			}
			// there is no direct way to detect this error because it is not exposed
			if listenerCloseError(err) {
				select {
				case <-shutdownCtx.Done():
					return
				case <-gshutdownCtx.Done():
					return
				default:
					// fallthrough
				}
			}
			log.Println(&ServerError{s, "AcceptTCP", err})
			errCancel()
			return
		}

		c := NewConn(shutdownCtx, gshutdownCtx, errCtx, s, conn)
		s.Wg.Add(1)
		go c.handleConnection()
	}
}

func listenerCloseError(err error) bool {
	return strings.Contains(err.Error(), LISTENER_CLOSE_MATCHER)
}

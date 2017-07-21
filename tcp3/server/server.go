package server

import (
	"context"
	"log"
	"net"
	"strings"
	"sync"
)

const (
	listenerCloseMatcher = "use of closed network connection"
)

func handleConnection(conn *net.TCPConn, ctx context.Context, wg *sync.WaitGroup) {
	defer func() {
		conn.Close()
		wg.Done()
	}()

	readCtx, errRead := context.WithCancel(context.Background())

	go handleRead(conn, errRead)

	select {
	case <-readCtx.Done():
	case <-ctx.Done():
	}
}

func handleRead(conn *net.TCPConn, errRead context.CancelFunc) {
	defer errRead()

	buf := make([]byte, 4*1024)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok {
				switch {
				case ne.Temporary():
					continue
				}
			}
			log.Println("Read", err)
			return
		}

		n, err = conn.Write(buf[:n])
		if err != nil {
			log.Println("Write", err)
			return
		}
	}
}

type Server struct {
	addr     string
	listener *net.TCPListener
	ctx      context.Context
	shutdown context.CancelFunc
	Wg       sync.WaitGroup
	ChClosed chan struct{}
}

func NewServer(parent context.Context, addr string) *Server {
	ctx, shutdown := context.WithCancel(parent)
	chClosed := make(chan struct{})
	return &Server{
		addr:     addr,
		ctx:      ctx,
		shutdown: shutdown,
		ChClosed: chClosed,
	}
}

func (s *Server) Shutdown() {
	select {
	case <-s.ctx.Done():
		// already shutdown
	default:
		s.shutdown()
		s.listener.Close()
	}
}

func (s *Server) Listen() error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", s.addr)
	if err != nil {
		return err
	}

	l, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}
	s.listener = l

	go s.handleListener()
	return nil
}

func (s *Server) handleListener() {
	defer func() {
		s.listener.Close()
		close(s.ChClosed)
	}()
	for {
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			if ne, ok := err.(*net.OpError); ok {
				if ne.Temporary() {
					log.Println("AcceptTCP", err)
					continue
				}
			}
			if listenerCloseError(err) {
				select {
				case <-s.ctx.Done():
					return
				default:
					// fallthrough
				}
			}

			log.Println("AcceptTCP", err)
			return
		}

		s.Wg.Add(1)
		go handleConnection(conn, s.ctx, &s.Wg)
	}
}

func listenerCloseError(err error) bool {
	return strings.Contains(err.Error(), listenerCloseMatcher)
}

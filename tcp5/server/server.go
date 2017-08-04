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

type Server struct {
	addr      string
	listener  *net.TCPListener
	ctx       context.Context
	shutdown  context.CancelFunc
	AcceptCtx context.Context
	errAccept context.CancelFunc
	Wg        sync.WaitGroup
	ChClosed  chan struct{}
}

func NewServer(parent context.Context, addr string) *Server {
	ctx, shutdown := context.WithCancel(parent)
	acceptCtx, errAccept := context.WithCancel(context.Background())
	chClosed := make(chan struct{})
	return &Server{
		addr:      addr,
		ctx:       ctx,
		shutdown:  shutdown,
		AcceptCtx: acceptCtx,
		errAccept: errAccept,
		ChClosed:  chClosed,
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
			if ne, ok := err.(net.Error); ok {
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
			s.errAccept()
			return
		}

		c := newConn(s, conn)
		s.Wg.Add(1)
		go c.handleConnection()
	}
}

func listenerCloseError(err error) bool {
	return strings.Contains(err.Error(), listenerCloseMatcher)
}

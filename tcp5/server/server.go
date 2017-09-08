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
	AcceptCtx context.Context
	errAccept context.CancelFunc
	Wg        sync.WaitGroup
	ChClosed  chan struct{}
	ctx       *contexts
	chCtx     chan *contexts
}

type contexts struct {
	ctxShutdown context.Context
	shutdown    context.CancelFunc
	ctxGraceful context.Context
	gshutdown   context.CancelFunc
}

func NewServer(parent context.Context, addr string) *Server {
	ctx := newContext(parent)
	acceptCtx, errAccept := context.WithCancel(context.Background())
	chClosed := make(chan struct{})
	chCtx := make(chan *contexts, 1)
	return &Server{
		addr:      addr,
		AcceptCtx: acceptCtx,
		errAccept: errAccept,
		ChClosed:  chClosed,
		ctx:       ctx,
		chCtx:     chCtx,
	}
}

func newContext(parent context.Context) *contexts {
	ctxShutdown, shutdown := context.WithCancel(parent)
	ctxGraceful, gshutdown := context.WithCancel(context.Background())
	return &contexts{
		ctxShutdown: ctxShutdown,
		shutdown:    shutdown,
		ctxGraceful: ctxGraceful,
		gshutdown:   gshutdown,
	}
}

func (s *Server) Shutdown() {
	select {
	case <-s.ctx.ctxShutdown.Done():
		// already shutdown
	default:
		s.ctx.shutdown()
		s.listener.Close()
	}
}

func (s *Server) GracefulShutdown() {
	select {
	case <-s.ctx.ctxGraceful.Done():
		// already shutdown
	default:
		s.ctx.gshutdown()
		s.listener.Close()
	}
}

func (s *Server) Restart(parent context.Context, addr string) (*Server, error) {
	if addr == s.addr {
		// update contexts. not close listener
		prevCtx := s.ctx
		s.ctx = newContext(parent)
		select {
		case <-s.chCtx:
			// clear s.chCtx if previous contexts have not been popped
		default:
		}
		s.chCtx <- s.ctx
		prevCtx.gshutdown()
		return s, nil
	} else {
		// create new listener
		nextServer := NewServer(parent, addr)
		err := nextServer.Listen()
		if err != nil {
			return nil, err
		}
		s.GracefulShutdown()
		s.Wg.Wait()
		<-s.ChClosed
		return nextServer, nil
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

	ctx := s.ctx

	for {
		conn, err := s.listener.AcceptTCP()

		select {
		case ctx = <-s.chCtx:
			// update ctx if changed
		default:
		}

		if err != nil {
			if ne, ok := err.(net.Error); ok {
				if ne.Temporary() {
					log.Println("AcceptTCP", err)
					continue
				}
			}
			if listenerCloseError(err) {
				select {
				case <-ctx.ctxShutdown.Done():
					return
				case <-ctx.ctxGraceful.Done():
					return
				default:
					// fallthrough
				}
			}

			log.Println("AcceptTCP", err)
			s.errAccept()
			return
		}

		c := newConn(s, ctx, conn)
		s.Wg.Add(1)
		go c.handleConnection()
	}
}

func listenerCloseError(err error) bool {
	return strings.Contains(err.Error(), listenerCloseMatcher)
}

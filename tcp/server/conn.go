package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

const (
	readBufSize = 4 * 1024
)

var (
	errWriteCancelled = errors.New("Write Context Canceled")
)

type ConnError struct {
	c   *Conn
	Op  string
	err error
}

func (e *ConnError) Error() string {
	return fmt.Sprintf("Conn[%v] (%v): %v", e.c.conn.RemoteAddr(), e.Op, e.err)
}

type Conn struct {
	s            *Server
	conn         *net.TCPConn
	wg           sync.WaitGroup
	shutdownCtx  context.Context
	gshutdownCtx context.Context
	serverErrCtx context.Context
	ctxRead      context.Context
	cancelRead   context.CancelFunc
	ctxWrite     context.Context
	cancelWrite  context.CancelFunc
	sem          chan struct{}
}

func NewConn(shutdownCtx, gshutdownCtx, serverErrCtx context.Context, s *Server, conn *net.TCPConn) *Conn {
	ctxRead, cancelRead := context.WithCancel(context.Background())
	ctxWrite, cancelWrite := context.WithCancel(context.Background())
	sem := make(chan struct{}, 1)
	newConn := Conn{
		s:            s,
		conn:         conn,
		shutdownCtx:  shutdownCtx,
		gshutdownCtx: gshutdownCtx,
		serverErrCtx: serverErrCtx,
		ctxRead:      ctxRead,
		cancelRead:   cancelRead,
		ctxWrite:     ctxWrite,
		cancelWrite:  cancelWrite,
		sem:          sem,
	}
	return &newConn
}

func (c *Conn) handleConnection() {
	var isShutdown bool
	defer func() {
		if err := c.conn.CloseRead(); err != nil {
			log.Println(&ConnError{c, "CloseRead", err})
		}
		if !isShutdown {
			c.wg.Wait()
		}
		c.cancelWrite()
		if err := c.conn.CloseWrite(); err != nil {
			log.Println(&ConnError{c, "CloseWrite", err})
		}
		log.Println("Conn(", c.conn.RemoteAddr(), "): closed")
		c.s.Wg.Done()
	}()
	log.Println("Conn(", c.conn.RemoteAddr(), "): Accepted")

	if err := c.conn.SetKeepAlive(true); err != nil {
		log.Println(&ConnError{c, "SetKeepAlive", err})
		return
	}
	if err := c.conn.SetKeepAlivePeriod(10 * time.Second); err != nil {
		log.Println(&ConnError{c, "SetKeepAlivePeriod", err})
		return
	}

	go c.handleRead()

	select {
	case <-c.gshutdownCtx.Done():
	case <-c.ctxRead.Done():
	case <-c.shutdownCtx.Done():
		isShutdown = true
	case <-c.serverErrCtx.Done():
	}
}

func (c *Conn) handleRead() {
	defer c.cancelRead()

	var n int
	var err error

	buf := make([]byte, readBufSize)

	for {
		n, err = c.conn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok {
				switch {
				case ne.Temporary():
					log.Println(&ConnError{c, "Read", err})
					continue
				}
			}
			select {
			case <-c.gshutdownCtx.Done():
			case <-c.ctxRead.Done():
			case <-c.shutdownCtx.Done():
			case <-c.serverErrCtx.Done():
			default:
				log.Println(&ConnError{c, "Read", err})
			}
			return
		}

		// TODO: parse

		c.wg.Add(1)
		go c.handleEcho(buf[:n])
		continue
	}
}

func (c *Conn) handleEcho(buf []byte) {
	// TODO: handle

	if err := c.write(buf); err != nil {
		log.Println("handleEcho: ", err)
		c.cancelRead()
	}
	c.wg.Done()
}

func (c *Conn) write(buf []byte) error {
	select {
	case <-c.ctxWrite.Done():
		return errWriteCancelled
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
		for {
			c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := c.conn.Write(buf)
			return err
		}
	}
}

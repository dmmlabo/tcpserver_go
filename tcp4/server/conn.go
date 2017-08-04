package server

import (
	"context"
	"log"
	"net"
)

type Conn struct {
	svr       *Server
	conn      *net.TCPConn
	ctxRead   context.Context
	stopRead  context.CancelFunc
	ctxWrite  context.Context
	stopWrite context.CancelFunc
	sem       chan struct{}
}

func newConn(svr *Server, tcpConn *net.TCPConn) *Conn {
	ctxRead, stopRead := context.WithCancel(context.Background())
	ctxWrite, stopWrite := context.WithCancel(context.Background())
	sem := make(chan struct{}, 1)
	return &Conn{
		svr:       svr,
		conn:      tcpConn,
		ctxRead:   ctxRead,
		stopRead:  stopRead,
		ctxWrite:  ctxWrite,
		stopWrite: stopWrite,
		sem:       sem,
	}
}

func (c *Conn) handleConnection() {
	defer func() {
		c.stopWrite()
		c.conn.Close()
		c.svr.Wg.Done()
	}()

	go c.handleRead()

	select {
	case <-c.ctxRead.Done():
	case <-c.svr.ctx.Done():
	case <-c.svr.AcceptCtx.Done():
	}
}

func (c *Conn) handleRead() {
	defer c.stopRead()

	buf := make([]byte, 4*1024)

	for {
		n, err := c.conn.Read(buf)
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

		wBuf := make([]byte, n)
		copy(wBuf, buf[:n])
		go c.handleEcho(wBuf)
	}
}

func (c *Conn) handleEcho(buf []byte) {
	// do something

	// write
	select {
	case <-c.ctxWrite.Done():
		return
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
		for {
			n, err := c.conn.Write(buf)
			if err != nil {
				if nerr, ok := err.(net.Error); ok {
					if nerr.Temporary() {
						buf = buf[n:]
						continue
					}
				}
				log.Println("Write error", err)
				// write error
				c.stopRead()
			}
			return
		}
	}
}

package server

import (
	"context"
	"log"
	"net"
)

type Conn struct {
	svr      *Server
	conn     *net.TCPConn
	readCtx  context.Context
	stopRead context.CancelFunc
}

func newConn(svr *Server, tcpConn *net.TCPConn) *Conn {
	readCtx, stopRead := context.WithCancel(context.Background())
	return &Conn{
		svr:      svr,
		conn:     tcpConn,
		readCtx:  readCtx,
		stopRead: stopRead,
	}
}

func (c *Conn) handleConnection() {
	defer func() {
		c.conn.Close()
		c.svr.Wg.Done()
	}()

	go c.handleRead()

	select {
	case <-c.readCtx.Done():
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

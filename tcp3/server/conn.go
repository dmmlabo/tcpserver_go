package server

import (
	"context"
	"log"
	"net"
)

type Conn struct {
	svr     *Server
	conn    *net.TCPConn
	readCtx context.Context
	errRead context.CancelFunc
}

func newConn(svr *Server, tcpConn *net.TCPConn) *Conn {
	readCtx, errRead := context.WithCancel(context.Background())
	return &Conn{
		svr:     svr,
		conn:    tcpConn,
		readCtx: readCtx,
		errRead: errRead,
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
	}
}

func (c *Conn) handleRead() {
	defer c.errRead()

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

		n, err = c.conn.Write(buf[:n])
		if err != nil {
			log.Println("Write", err)
			return
		}
	}
}

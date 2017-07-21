package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
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

func handleListener(l *net.TCPListener, ctx context.Context, wg *sync.WaitGroup, chClosed chan struct{}) {
	defer func() {
		l.Close()
		close(chClosed)
	}()
	for {
		conn, err := l.AcceptTCP()
		if err != nil {
			if ne, ok := err.(net.Error); ok {
				if ne.Temporary() {
					log.Println("AcceptTCP", err)
					continue
				}
			}
			if listenerCloseError(err) {
				select {
				case <-ctx.Done():
					return
				default:
					// fallthrough
				}
			}

			log.Println("AcceptTCP", err)
			return
		}

		wg.Add(1)
		go handleConnection(conn, ctx, wg)
	}
}

func listenerCloseError(err error) bool {
	return strings.Contains(err.Error(), listenerCloseMatcher)
}

func main() {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:12345")
	if err != nil {
		log.Println("ResolveTCPAddr", err)
		return
	}

	l, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Println("ListenTCP", err)
		return
	}

	sigChan := make(chan os.Signal, 1)
	// Ignore all signals
	signal.Ignore()
	signal.Notify(sigChan, syscall.SIGINT)

	var wg sync.WaitGroup
	chClosed := make(chan struct{})

	ctx, shutdown := context.WithCancel(context.Background())

	go handleListener(l, ctx, &wg, chClosed)

	log.Println("Server Started")

	s := <-sigChan

	switch s {
	case syscall.SIGINT:
		log.Println("Server Shutdown...")
		shutdown()
		l.Close()

		wg.Wait()
		<-chClosed
		log.Println("Server Shutdown Completed")
	default:
		panic("unexpected signal has been received")
	}
}

package main

import (
	"log"
	"net"
)

func handleConnection(conn *net.TCPConn) {
	defer conn.Close()

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

func handleListener(l *net.TCPListener) error {
	defer l.Close()
	for {
		conn, err := l.AcceptTCP()
		if err != nil {
			if ne, ok := err.(net.Error); ok {
				if ne.Temporary() {
					log.Println("AcceptTCP", err)
					continue
				}
			}
			return err
		}

		go handleConnection(conn)
	}
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

	err = handleListener(l)
	if err != nil {
		log.Println("handleListener", err)
	}
}

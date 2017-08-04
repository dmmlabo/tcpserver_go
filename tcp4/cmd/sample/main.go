package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	c, err := net.Dial("tcp", "127.0.0.1:12345")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	go write(c)

	buf := make([]byte, 100)
	for i := 0; i < 256; i++ {
		var n int
		n, err = c.Read(buf)
		fmt.Println(buf[:n])
	}
}

func write(c net.Conn) {
	for i := 0; i < 256; i++ {
		c.Write([]byte{byte(i)})
	}
}

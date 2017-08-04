package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dmmlabo/tcpserver_go/tcp5/server"
)

func main() {
	sigChan := make(chan os.Signal, 1)
	// Ignore all signals
	signal.Ignore()
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	svr := server.NewServer(context.Background(), "127.0.0.1:12345")

	err := svr.Listen()

	if err != nil {
		log.Fatal("Listen()", err)
	}

	log.Println("Server Started")

	select {
	case sig := <-sigChan:
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Println("Server Shutdown...")
			svr.Shutdown()

			svr.Wg.Wait()
			<-svr.ChClosed
			log.Println("Server Shutdown Completed")
		case syscall.SIGQUIT:
			log.Println("Server Graceful Shutdown...")
			svr.GracefulShutdown()

			svr.Wg.Wait()
			<-svr.ChClosed
			log.Println("Server Graceful Shutdown Completed")
		default:
			panic("unexpected signal has been received")
		}
	case <-svr.AcceptCtx.Done():
		log.Println("Server Error Occurred")
		svr.Wg.Wait()
		<-svr.ChClosed
		log.Println("Server Shutdown Completed")
	}
}

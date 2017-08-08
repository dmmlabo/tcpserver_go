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
	chSig := make(chan os.Signal, 1)
	// Ignore all signals
	signal.Ignore()
	signal.Notify(chSig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)

	for {
		restart := startup(chSig)
		if restart {
			continue
		}
		break
	}
}

func startup(chSig chan os.Signal) (restart bool) {
	svr := server.NewServer(context.Background(), "127.0.0.1:12345")

	err := svr.Listen()

	if err != nil {
		log.Fatal("Listen()", err)
	}

	log.Println("Server Started")

	select {
	case sig := <-chSig:
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
		case syscall.SIGHUP:
			log.Println("Server Restarting...")
			svr.GracefulShutdown()

			<-svr.ChClosed
			restart = true
		default:
			panic("unexpected signal has been received")
		}
	case <-svr.AcceptCtx.Done():
		log.Println("Server Error Occurred")
		svr.Wg.Wait()
		<-svr.ChClosed
		log.Println("Server Shutdown Completed")
	}
	return
}

package tcp1

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dmmlabo/tcpserver_go/tcp1/server"
)

var (
	InternalServerError = errors.New("server error occurred")
)

func main() {
	sigChan := make(chan os.Signal, 1)
	// Ignore all signals
	signal.Ignore()
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)
	for {
		log.Println("Server Starting Listen")
		restart, err := startup(sigChan)
		if err != nil {
			log.Println("Error:", err)
			return
		}
		if !restart {
			return
		}
	}
}

func startup(sigChan <-chan os.Signal) (restart bool, err error) {
	var svr *server.Server
	svr, err = server.NewServer("127.0.0.1:12345")
	if err != nil {
		return
	}
	shutdownCtx, shutdown := context.WithCancel(context.Background())
	gshutdownCtx, gshutdown := context.WithCancel(context.Background())

	errCtx, err := svr.Listen(shutdownCtx, gshutdownCtx)
	if err != nil {
		return
	}
	log.Println("Server Listening: ", svr.Addr)

	// Wait
	select {
	case s := <-sigChan:
		switch s {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Println("Server Shutdown...")
			shutdown()
			svr.StopListener()

			svr.Wg.Wait()
			<-svr.ChClosed
			log.Println("Server Shutdown Completed")
		case syscall.SIGQUIT:
			log.Println("Server Graceful Shutdown...")
			gshutdown()
			svr.StopListener()

			svr.Wg.Wait()
			<-svr.ChClosed
			log.Println("Server Graceful Shutdown Completed")
		case syscall.SIGHUP:
			gshutdown()
			svr.StopListener()
			restart = true
			log.Println("Server Restarting...")

			<-svr.ChClosed
		default:
			panic("unexpected signal has been received")
		}
	case <-errCtx.Done():
		svr.Wg.Wait()
		<-svr.ChClosed
		return false, InternalServerError
	}
	return restart, nil
}

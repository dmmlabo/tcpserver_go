package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/dmmlabo/tcpserver_go/tcp5/server"
)

func main() {
	chSig := make(chan os.Signal, 1)
	// Ignore all signals
	signal.Ignore()
	signal.Notify(chSig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)

	var wg sync.WaitGroup
	defer wg.Wait()

	host := loadConf()
	svr := server.NewServer(context.Background(), host)
	err := svr.Listen()

	if err != nil {
		log.Fatal("Listen()", err)
	}
	log.Println("Server Started")

	for {
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

				oldHost := host
				host = loadConf()

				// Check host have changed
				if host == oldHost {
					// TODO: load config to server
					log.Println("Server Config Loaded")
				} else {
					// Restart Server
					oldSvr := svr
					svr = server.NewServer(context.Background(), host)
					err = svr.Listen()
					if err != nil {
						log.Fatal("Listen()", err)
					}
					log.Println("Server Restarted")
					wg.Add(1)
					go func() {
						oldSvr.GracefulShutdown()
						wg.Done()
					}()
				}
				continue
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
}

var first = true

func loadConf() string {
	// TODO: load config from file or env
	if first {
		first = false
		return "127.0.0.1:12345"
	} else {
		return "127.0.0.1:12346"
	}
}

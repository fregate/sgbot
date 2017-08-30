package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/takama/daemon"
)

const (
	cookiesFileName string = "./cookies.json"
	listsFileName   string = "./gameslist.json"
	configFileName  string = "./config.json"
)

const (
	// name of the service
	serviceName        = "sgbotservice"
	serviceDescription = "SteamGifts Bot Service"

	// port which daemon should be listen
	servicePort = ":9977"
)

// Service has embedded daemon
type Service struct {
	daemon.Daemon

	bot *TheBot
}

//	dependencies that are NOT required by the service, but might be used
var dependencies = []string{"dummy.service"}

var stdlog, errlog *log.Logger

// Manage by daemon commands or run the daemon
func (service *Service) Manage() (string, error) {

	usage := "Usage: myservice install | remove | start | stop | status"

	// if received any kind of command, do it
	if len(os.Args) > 1 {
		command := os.Args[1]
		switch command {
		case "install":
			return service.Install()
		case "remove":
			return service.Remove()
		case "start":
			return service.Start()
		case "stop":
			return service.Stop()
		case "status":
			return service.Status()
		default:
			return usage, nil
		}
	}

	// Do something, call your goroutines, etc
	go startBot(service)

	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)

	// loop work cycle with accept connections or interrupt
	// by system signal
	for {
		select {
		//		case conn := <-listen:
		//			go handleClient(conn)
		case killSignal := <-interrupt:
			stdlog.Println("Got signal:", killSignal)
			//stdlog.Println("Stoping listening on ", listener.Addr())
			//listener.Close()
			if service.bot != nil {
				service.bot.SendPanicMsg("Daemon was interruped by system signal")
			}
			if killSignal == os.Interrupt {
				return "Daemon was interruped by system signal", nil
			}
			return "Daemon was killed", nil
		}
	}

	// never happen, but need to complete code
	return usage, nil
}

func init() {
	stdlog = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	errlog = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds)
}

func startBot(srv *Service) {
	stdlog.Println("bot started")

	defer func() {
		if srv != nil && srv.Daemon != nil {
			srv.Stop()
		}
	}()

	srv.bot = &TheBot{}

	err := srv.bot.InitBot(configFileName, cookiesFileName, listsFileName)
	if err != nil {
		errlog.Println("error while initialize bot.", err)
		srv.bot.SendPanicMsg(fmt.Sprintf("error while initialize bot.\n%v", err))
		return
	}

	for {
		count, err := srv.bot.Check()
		if err != nil {
			errlog.Println("error during check.", err)
			srv.bot.SendPanicMsg(fmt.Sprintf("error during check.\n%v", err))
			break
		}
		stdlog.Println("wait for", (count+1)*60, "mins")
		time.Sleep(time.Hour * time.Duration(count+1))
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "test" {
		startBot(&Service{nil, nil})
		return
	}

	srv, err := daemon.New(serviceName, serviceDescription)
	if err != nil {
		errlog.Println("Error: ", err)
		os.Exit(1)
	}
	service := &Service{srv, nil}
	status, err := service.Manage()
	if err != nil {
		errlog.Println(status, "\nError: ", err)
		os.Exit(1)
	}
	fmt.Println(status)
}

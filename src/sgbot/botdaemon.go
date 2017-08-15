package main

import (
        "fmt"
        "log"
        "net"
        "os"
        "os/signal"
        "syscall"
        "time"

        "github.com/takama/daemon"
)

const (
        configFileName string = "./cookies.json"
        listsFileName  string = "./gameslist.json"
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
}

//      dependencies that are NOT required by the service, but might be used
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

        // Set up listener for defined host and port
        //listener, err := net.Listen("tcp", servicePort)
        //if err != nil {
        //              return "Possibly was a problem with the port binding", err
        //      }

        // set up channel on which to send accepted connections
        //      listen := make(chan net.Conn, 100)
        //      go acceptConnection(listener, listen)

        // loop work cycle with accept connections or interrupt
        // by system signal
        for {
                select {
                //              case conn := <-listen:
                //                      go handleClient(conn)
                case killSignal := <-interrupt:
                        stdlog.Println("Got signal:", killSignal)
                        //stdlog.Println("Stoping listening on ", listener.Addr())
                        //listener.Close()
                        if killSignal == os.Interrupt {
                                return "Daemon was interruped by system signal", nil
                        }
                        return "Daemon was killed", nil
                }
        }

        // never happen, but need to complete code
        return usage, nil
}

// Accept a client connection and collect it in a channel
func acceptConnection(listener net.Listener, listen chan<- net.Conn) {
        for {
                conn, err := listener.Accept()
                if err != nil {
                        continue
                }
                listen <- conn
        }
}

func handleClient(client net.Conn) {
        for {
                buf := make([]byte, 4096)
                numbytes, err := client.Read(buf)
                if numbytes == 0 || err != nil {
                        return
                }
                buf = append(buf, 32, 0x30)
                client.Write(buf[:numbytes+2])
        }
}

func init() {
        stdlog = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)
        errlog = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds)
}

func startBot(srv *Service) {
        stdlog.Println("bot started")

        defer func() {
                if srv != nil {
                        srv.Stop()
                }
        } ()

        var bot TheBot
        err := bot.InitBot(configFileName, listsFileName)
        if err != nil {
                errlog.Println("error while initialize bot. stop service", err)
                return
        }

        for {
                err = bot.Check()
                if err != nil {
                        errlog.Println("error during check. stop service", err)
                        break
                }
                time.Sleep(time.Hour)
        }
}

func main() {
        if len(os.Args) > 1 && os.Args[1] == "test" {
                startBot(nil)
                return
        }

        srv, err := daemon.New(serviceName, serviceDescription)
        if err != nil {
                errlog.Println("Error: ", err)
                os.Exit(1)
        }
        service := &Service{srv}
        status, err := service.Manage()
        if err != nil {
                errlog.Println(status, "\nError: ", err)
                os.Exit(1)
        }
        fmt.Println(status)
}

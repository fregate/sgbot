package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/takama/daemon"
	gomail "gopkg.in/gomail.v2"
)

//go:generate python3 pathes.py pathes.go

const (
	// name of the service
	serviceName        = "sgbotservice"
	serviceDescription = "SteamGifts Bot Service"
)

// Service has embedded daemon
type Service struct {
	daemon.Daemon

	bot    *TheBot
	server *WebConfig

	config             Configuration
	lastTimeDigestSent time.Time
}

func (s *Service) readGameLists() (games map[uint64]bool, err error) {
	games = make(map[uint64]bool)

	var ccc interface{}
	err = ReadConfig(listsFileName, &ccc)
	if err == nil {
		m := ccc.(map[string]interface{})
		for k := range m {
			q, err := strconv.ParseUint(k, 10, 32)
			if err != nil {
				break
			}
			games[q] = true
		}
	} else {
		stdlog.Println(err)
	}

	stdlog.Printf("successfully load external games list [total entries:%d]\n", len(games))
	return
}

// Check - check page and enter for gifts (repeat by timeout)
func (s *Service) check(b *TheBot, digest []string) (count int, err error) {
	stdlog.Println("bot checking...")

	defer b.clean()

	games, err := s.readGameLists()
	if err != nil {
		return
	}

	cookies, err := ReadCookies(cookiesFileName)
	if err != nil {
		return
	}

	b.setCookies(cookies)

	err = b.getUserInfo()
	if err != nil {
		return
	}

	// parse main page
	defer stdlog.Println("bot check finished")

	// parse main page
	return b.parseGiveaways(games)
}

// sendPanicMsg sends msg (usually error and stops service) + current digest
func (s *Service) sendPanicMsg(msg string, digest []string) {
	if s.config.SendDigest {
		msg = msg + "\n\n" + strings.Join(digest, "\n")
	}

	s.sendMail("Panic Message!", msg)
}

func (s *Service) sendDigest(digest []string) bool {
	if !s.config.SendDigest {
		return true
	}

	if time.Now().Hour() == 0 || time.Since(s.lastTimeDigestSent) > time.Hour*24 {
		stdlog.Println("sending digest")
		s.sendMail("Daily digest", strings.Join(digest, "\n"))
		s.lastTimeDigestSent = time.Now()

		return true
	}

	return false
}

// send digest
func (s *Service) sendMail(subject, msg string) (err error) {
	if msg == "" || !s.config.MailSettings.isValid() {
		return nil
	}

	mailer := gomail.NewDialer(
		s.config.MailSettings.SMTPServer,
		s.config.MailSettings.Port,
		s.config.MailSettings.SMTPUsername,
		s.config.MailSettings.SMTPUserpassword)

	m := gomail.NewMessage()
	m.SetHeader("From", s.config.MailSettings.SMTPUsername)
	m.SetHeader("To", s.config.MailSettings.EmailRecipient)
	m.SetHeader("Subject", fmt.Sprintf("%s %s", s.config.MailSettings.EmailSubjectTag, subject))
	m.SetBody("text/plain", msg)

	err = mailer.DialAndSend(m)
	if err != nil {
		errlog.Println(err)
	}

	return
}

// Manage by daemon commands or run the daemon
func (service *Service) manage() (string, error) {
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
			service.sendPanicMsg("Daemon was interruped by system signal", make([]string, 0))
			if killSignal == os.Interrupt {
				return "Daemon was interruped by system signal", nil
			}
			return "Daemon was killed", nil
		}
	}

	// never happen, but need to complete code
	return usage, nil
}

func startBot(srv *Service) {
	stdlog.Println("bot started")

	defer func() {
		if srv != nil && srv.Daemon != nil {
			srv.Stop()
		}
	}()

	srv.server = &WebConfig{}
	err := srv.server.InitWebConfig(configFileName, cookiesFileName, listsFileName)
	if err != nil {
		stdlog.Println("error while initialize web app.", err)
	} else {
		go srv.server.Serve()
	}

	srv.bot = &TheBot{}
	err = srv.bot.InitBot(srv.config.SteamProfile)
	if err != nil {
		errlog.Println("error while initialize bot.", err)
		srv.sendPanicMsg(fmt.Sprintf("error while initialize bot.\n%v", err), make([]string, 0))
		return
	}

	digest := make([]string, 0)
	for {
		count, err := srv.check(srv.bot, digest)
		if err != nil {
			errlog.Println("error during check.", err)
			srv.sendPanicMsg(fmt.Sprintf("error during check.\n%v", err), digest)
			break
		}
		stdlog.Println("wait for", (count+1)*60, "mins")
		digest = append(digest, srv.bot.enteredGiveAways...)
		srv.bot.enteredGiveAways = make([]string, 0)
		if srv.sendDigest(digest) {
			digest = make([]string, 0)
		}
		time.Sleep(time.Hour * time.Duration(count+1))
	}
}

func runTest(config *Configuration) {
	lastDigest := time.Now()
	if len(os.Args) >= 3 {
		milli, err := strconv.ParseInt(os.Args[2], 10, 64)
		if err == nil {
			lastDigest = time.UnixMilli(milli)
		}
	}
	startBot(&Service{nil, nil, nil, *config, lastDigest})
}

func main() {
	if _, err := os.Stat(configFileName); os.IsNotExist(err) {
		f, err := os.Open(configFileName)
		if err != nil {
			errlog.Println("Error: ", err)
			os.Exit(1)
		}
		f.Close()
	}

	if _, err := os.Stat(cookiesFileName); os.IsNotExist(err) {
		f, err := os.Open(cookiesFileName)
		if err != nil {
			errlog.Println("Error: ", err)
			os.Exit(1)
		}
		f.Close()
	}

	if _, err := os.Stat(listsFileName); os.IsNotExist(err) {
		f, err := os.Open(listsFileName)
		if err != nil {
			errlog.Println("Error: ", err)
			os.Exit(1)
		}
		f.Close()
	}

	// read steam profile, mail-smtp settings, web config settings
	config, err := ReadConfiguration(configFileName)
	if err != nil {
		errlog.Println("Invalid configureation file\nError: ", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 && os.Args[1] == "test" {
		runTest(&config)
		return
	}

	daemonType := daemon.SystemDaemon
	if runtime.GOOS == "darwin" {
		daemonType = daemon.UserAgent
	}

	srv, err := daemon.New(serviceName, serviceDescription, daemonType)
	if err != nil {
		errlog.Println("Error: ", err)
		os.Exit(1)
	}

	service := &Service{srv, nil, nil, config, time.Now()}
	status, err := service.manage()
	if err != nil {
		errlog.Println(status, "\nError: ", err)
		os.Exit(1)
	}
	fmt.Println(status)
}

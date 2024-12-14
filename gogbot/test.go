package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

const (
	// cookiesFileName string = "cookies.json"
	cookiesFileName string = "assets/cookies..json"
)

var stdlog, errlog *log.Logger

func readJsonInterface(uri string, c interface{}) (err error) {
	raw, err := os.ReadFile(uri)

	if err != nil {
		return
	}

	err = json.Unmarshal(raw, c)
	if err != nil {
		return
	}

	return nil
}

func readCookies(fileName string) (cookies []*http.Cookie, err error) {
	var ccc interface{}
	err = readJsonInterface(fileName, &ccc)
	if err != nil {
		stdlog.Println("Invalid cookie file", fileName)
		return
	}

	m := ccc.(map[string]interface{})
	for k, v := range m {
		ck := strings.Split(v.(string), ":")
		if len(ck) >= 3 {
			cookies = append(cookies, &http.Cookie{Name: k, Value: ck[0], Domain: ck[1], Path: ck[2]})
		} else {
			stdlog.Println("wrong cookie (< 3 params)", k, v.(string))
		}
	}

	return
}

func startBot() {
	stdlog.Println("bot started")

	bot := &TheBot{}
	err := bot.initBot()
	if err != nil {
		errlog.Println("error while initialize bot.", err)
		return
	}

	cookies, err := readCookies(cookiesFileName)
	if err != nil {
		return
	}

	bot.setCookies(cookies)

	ret, err := bot.claimGiveaway()
	if err != nil {
		errlog.Println("Error during claim:", err)
	} else {
		stdlog.Println("Check result:", ret)
	}
}

func main()  {
	stdlog = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	errlog = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds)

	if _, err := os.Stat(cookiesFileName); os.IsNotExist(err) {
		f, err := os.Open(cookiesFileName)
		if err != nil {
			errlog.Println("Error: ", err)
			os.Exit(1)
		}
		f.Close()
	}

	startBot()
}

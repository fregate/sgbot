package main

import (
	"os"
	"path"
	"path/filepath"
	"fmt"
	"net/http"
	"html/template"
	"strconv"
	"strings"

	"github.com/abbot/go-http-auth"
)

type serveConfig struct {
	httpAuthLogin string `json:"httpauth"`
	httpAuthPwd string `json:"httppwd"`

	listenPort uint16 `json:"port"`
	staticFiles string `json:"files"`
}

func (c serveConfig) isWebConfigValid() bool {
	return c.httpAuthLogin != "" && c.httpAuthPwd != ""
}

// WebConfig serve web application for manage json configuration for the Bot
type WebConfig struct {
	// config files
	gamesListFileName string
	configFileName    string
	cookiesFileName   string

	serveConfig serveConfig
}

// InitWebConfig reads configuration file for web application
// fill structures
func (w *WebConfig) InitWebConfig(configFile, cookieFile, listFile string) (err error) {
	if _, err = os.Stat(configFile); os.IsNotExist(err) {
		f, err := os.Open(configFile)
		if err != nil {
			return err
		}
		f.Close()
	}

	if _, err = os.Stat(cookieFile); os.IsNotExist(err) {
		f, err := os.Open(cookieFile)
		if err != nil {
			return err
		}
		f.Close()
	}

	if _, err = os.Stat(listFile); os.IsNotExist(err) {
		f, err := os.Open(listFile)
		if err != nil {
			return err
		}
		f.Close()
	}

	w.gamesListFileName, w.cookiesFileName, w.configFileName = listFile, cookieFile, configFile

	// read steam profile, parse wishlist and followed games (also read mail-smtp settings)
	{
		err = ReadConfig(w.configFileName, &w.serveConfig)
		if err != nil {
			stdlog.Println(err)
			return
		}

		if !w.serveConfig.isWebConfigValid() {
			stdlog.Println("No WebConfig settings. No WebUI")
			return
		}

		if w.serveConfig.staticFiles == "" {
			w.serveConfig.staticFiles = path.Join(path.Dir(w.configFileName), "static")
		}

		if w.serveConfig.listenPort < 1024 {
			w.serveConfig.listenPort = 8080
		}
	}

	return
}

/* func HelloWorld(w http.ResponseWriter, r *http.Request) {
  fmt.Fprintf(w, "Hello World!")
} */

/* func BasicAuth(w *WebConfig, handler http.HandlerFunc, realm string) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		user, pass, ok := req.BasicAuth()
		if !ok || user != w.serveConfig.httpAuthLogin || pass != w.serveConfig.httpAuthPwd {
			writer.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			writer.WriteHeader(401)
			writer.Write([]byte("You are Unauthorized to access the application.\n"))
			return
		}
		fileServer := http.FileServer(http.Dir(w.serveConfig.staticFiles))
		handler(writer, req)
	}
} */

func getSteamPage(id uint64) string {
	return fmt.Sprint("https://store.steampowered.com/app/", id)
}

// Serve runs http service on configured port
func (w *WebConfig) Serve() (err error) {
	var games map[string]uint64
	var ccc interface{}
	err = ReadConfig(w.gamesListFileName, &ccc)
	if err != nil {
		stdlog.Println(err)
		return
	}

	m := ccc.(map[string]string)
	for k, name := range m {
		q, err := strconv.ParseUint(k, 10, 32)
		if err != nil {
			break
		}
		games[name] = q
	}

	cookies, err := ReadCookies(w.cookiesFileName)
	if err != nil {
		return
	}

	c := ccc.(map[string]interface{})
	for k, v := range c {
		ck := strings.Split(v.(string), ":")
		if len(ck) >= 3 {
			cookies = append(cookies, &http.Cookie{Name: k, Value: ck[0], Domain: ck[1], Path: ck[2]})
		} else {
			stdlog.Println("wrong cookie (< 3 params)", k, v.(string))
		}
	}

	configStruct, err := ReadConfiguration(w.configFileName)
	if err != nil {
		return
	}

	authenticator := auth.NewBasicAuthenticator("Enter login and password", func(user, realm string) string {
		if user == w.serveConfig.httpAuthLogin {
			return w.serveConfig.httpAuthPwd
		}

		return ""
	})

	http.HandleFunc("/", authenticator.Wrap(func(writer http.ResponseWriter, req *auth.AuthenticatedRequest) {
		fp := filepath.Join(w.serveConfig.staticFiles, filepath.Clean(req.URL.Path))
		tmpl, _ := template.ParseFiles(fp)
		data := struct {
			Title string
			Body string
		} {
			Title: "First template",
			Body: "Body tezxt",
		}
		tmpl.ExecuteTemplate(writer, "layout", data)
	}))

	http.NotFoundHandler()

	err = http.ListenAndServe(fmt.Sprint(":", w.serveConfig.listenPort), nil)
	if err != nil {
		stdlog.Println("Fatal. Error starting http server : ", err)
		return
	}

	return
}

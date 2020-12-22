package main

import (
	"os"
	"path"
	"fmt"
	"net/http"
	"html/template"
	"errors"
	// "strconv"
	// "strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/abbot/go-http-auth"
)

//go:generate python3 generator.py ../assets/static/index.html index.go

// ServeConfig holds web application settings
type ServeConfig struct {
	HTTPAuthLogin  string  `json:"httpauth"`
	HTTPAuthPwd    string  `json:"httppwd"`

	ListenPort   uint16  `json:"port"`
	StaticFiles  string  `json:"files"`
}

func (c ServeConfig) isWebConfigValid() bool {
	return c.HTTPAuthLogin != "" && c.HTTPAuthPwd != ""
}

// WebConfig serve web application for manage json configuration for the Bot
type WebConfig struct {
	// config files
	gamesListFileName string
	configFileName    string
	cookiesFileName   string

	serveConfig ServeConfig
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

	{
		err = ReadConfig(w.configFileName, &w.serveConfig)
		if err != nil {
			stdlog.Println(err)
			return
		}

		if !w.serveConfig.isWebConfigValid() {
			stdlog.Println("No WebConfig settings. No WebUI")
			return errors.New("Invalid web application settings")
		}

		if w.serveConfig.StaticFiles == "" {
			w.serveConfig.StaticFiles = path.Join(path.Dir(w.configFileName), "static")
		}

		if w.serveConfig.ListenPort < 1024 {
			w.serveConfig.ListenPort = 8080
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
	authenticator := auth.NewBasicAuthenticator("Enter login and password", func(user, realm string) string {
		if user == w.serveConfig.HTTPAuthLogin {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(w.serveConfig.HTTPAuthPwd), bcrypt.DefaultCost)
			if err != nil {
				stdlog.Println("Error crypt pwd", err)
				return ""
			}

			return string(hashedPassword)
		}

		return ""
	})

	http.HandleFunc("/", authenticator.Wrap(func(writer http.ResponseWriter, req *auth.AuthenticatedRequest) {
		tmpl, _ := template.New("layout").Parse(indexTemplate)
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

	err = http.ListenAndServe(fmt.Sprint(":", w.serveConfig.ListenPort), nil)
	if err != nil {
		stdlog.Println("Fatal. Error starting http server : ", err)
		return
	}

	return
}

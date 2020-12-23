package main

import (
	"os"
	"path"
	"fmt"
	"net/http"
	"html/template"
	"errors"
	"io/ioutil"
	"strings"

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
		games, err := ioutil.ReadFile(w.gamesListFileName)
		if err != nil {
			http.Error(writer, fmt.Sprint("Error reading games list file {}", err), http.StatusInternalServerError)
		}

		tmpl, _ := template.New("layout").Parse(indexTemplate)
		data := struct {
			GamesList string
			CookiesList string
			Config string
		} {
			GamesList: string(games),
			Config: `{ "profile" : "fregate" }`,
			CookiesList: `{"PHPSESSID": "<PHPSESSID>:www.steamgifts.com:/"}`,
		}
		tmpl.ExecuteTemplate(writer, "layout", data)
	}))

	http.HandleFunc("/savegames", authenticator.Wrap(func(writer http.ResponseWriter, req *auth.AuthenticatedRequest) {
		if req.Method != http.MethodPost {
			http.Error(writer, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			http.Error(writer, "Error reading request body", http.StatusInternalServerError)
			return
		}

		f, err := os.OpenFile(w.gamesListFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			http.Error(writer, "Error opening gamelist file", http.StatusInternalServerError)
			return
		}

		_, err = f.Write(body)
		if err != nil {
			http.Error(writer, "Error writing gamelist file", http.StatusInternalServerError)
			return
		}
		f.Close()

		fmt.Fprint(writer, "ok")
	}))

	http.HandleFunc("/savecookies", authenticator.Wrap(func(writer http.ResponseWriter, req *auth.AuthenticatedRequest) {
	}))

	http.HandleFunc("/saveconfig", authenticator.Wrap(func(writer http.ResponseWriter, req *auth.AuthenticatedRequest) {
	}))

	http.HandleFunc("/parsepage", authenticator.Wrap(func(writer http.ResponseWriter, req *auth.AuthenticatedRequest) {
		if req.Method != http.MethodPost {
			http.Error(writer, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			http.Error(writer, "Error reading request body", http.StatusInternalServerError)
			return
		}

		// TODO: parse real steam page

		var response string
		parts := strings.Split(string(body), "/")
		for i := len(parts)-1; i >= 0; i-- {
			if len(parts[i]) == 0 {
				continue
			}

			if parts[i] == "app" {
				break
			}

			response = ":" + parts[i] + response
		}
		response = "#" + response[1:]
		fmt.Fprint(writer, response)
	}))

	err = http.ListenAndServe(fmt.Sprint(":", w.serveConfig.ListenPort), nil)
	if err != nil {
		stdlog.Println("Fatal. Error starting http server : ", err)
		return
	}

	return
}

package main

import (
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/abbot/go-http-auth"
)

//go:generate python3 generator.py ../assets/static/index.html index.go

// ServeConfig holds web application settings
type ServeConfig struct {
	HTTPAuthLogin string `json:"httpauth"`
	HTTPAuthPwd   string `json:"httppwd"`

	ListenPort  uint16 `json:"web-port-num"`
	StaticFiles string `json:"files"`
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
	stdlog.Println("init web application")
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
			return errors.New("invalid web application settings")
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

func handleFileSave(writer http.ResponseWriter, req *auth.AuthenticatedRequest, fileName string) (err error) {
	if req.Method != http.MethodPost {
		http.Error(writer, "Invalid request method", http.StatusMethodNotAllowed)
		return errors.New("invalid request method")
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		http.Error(writer, "Error reading request body", http.StatusInternalServerError)
		return errors.New("error reading request")
	}

	f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		http.Error(writer, "Error opening file", http.StatusInternalServerError)
		return errors.New("error opening file")
	}

	_, err = f.Write(body)
	if err != nil {
		http.Error(writer, "Error writing file", http.StatusInternalServerError)
		return errors.New("error writing file")
	}
	f.Close()

	fmt.Fprint(writer, "ok")
	return
}

// Serve runs http service on configured port
func (w *WebConfig) Serve() (err error) {
	stdlog.Println("serve")
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
			return
		}

		cookies, err := ioutil.ReadFile(w.cookiesFileName)
		if err != nil {
			http.Error(writer, fmt.Sprint("Error reading games list file {}", err), http.StatusInternalServerError)
			return
		}

		config, err := ioutil.ReadFile(w.configFileName)
		if err != nil {
			http.Error(writer, fmt.Sprint("Error reading games list file {}", err), http.StatusInternalServerError)
			return
		}

		tmpl, _ := template.New("layout").Parse(indexTemplate)
		data := struct {
			GamesList   string
			CookiesList string
			Config      string
		}{
			GamesList:   string(games),
			Config:      string(config),
			CookiesList: string(cookies),
		}
		tmpl.ExecuteTemplate(writer, "layout", data)
	}))

	http.HandleFunc("/savegames", authenticator.Wrap(func(writer http.ResponseWriter, req *auth.AuthenticatedRequest) {
		err := handleFileSave(writer, req, w.gamesListFileName)
		if err != nil {
			stdlog.Println("Error in games list handler", err)
		}
	}))

	http.HandleFunc("/savecookies", authenticator.Wrap(func(writer http.ResponseWriter, req *auth.AuthenticatedRequest) {
		handleFileSave(writer, req, w.cookiesFileName)
		if err != nil {
			stdlog.Println("Error in cookies file handler", err)
		}
	}))

	http.HandleFunc("/saveconfig", authenticator.Wrap(func(writer http.ResponseWriter, req *auth.AuthenticatedRequest) {
		handleFileSave(writer, req, w.configFileName)
		if err != nil {
			stdlog.Println("Error in config file handler", err)
		}
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
		for i := len(parts) - 1; i >= 0; i-- {
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

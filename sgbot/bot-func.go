package main

import (
	"fmt"
	"net/http"
)

type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

type Game struct {
	Name string `json:"name"`
	Id   uint64 `json:"id"`
}

type Request struct {
	SteamProfile string   `json:"profile"`
	Cookies      []Cookie `json:"cookies"`
	Games        []Game   `json:"games"`
}

func populateCookies(b *TheBot, botCookies []Cookie) {
	cookies := make([]*http.Cookie, 0)
	for _, c := range botCookies {
		cookies = append(cookies, &http.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path})
	}
	b.setCookies(cookies)
}

func populateGames(games []Game) (mapped map[uint64]bool) {
	mapped = make(map[uint64]bool)
	for _, game := range games {
		mapped[game.Id] = true
	}
	return
}

// Check - check page and enter for gifts (repeat by timeout)
func runCheck(b *TheBot, games map[uint64]bool) (digest []string, err error) {
	err = b.getUserInfo()
	if err != nil {
		return
	}

	// parse main page
	defer fmt.Println("bot check finished")

	// parse main page
	_, err = b.parseGiveaways(games)
	return b.enteredGiveAways, err
}

func RunBot(botRequest *Request) (digest []string, err error) {
	// body, err := ioutil.ReadAll(request.Body)
	// if err != nil {
	// 	io.WriteString(rw, fmt.Sprintf("error during request reading.", err))
	// 	rw.WriteHeader(http.StatusBadRequest)
	// 	return
	// }

	// botRequest := &Request{}
	// err = json.Unmarshal(body, &botRequest)
	// if err != nil {
	// 	io.WriteString(rw, fmt.Sprintf("error during request parse. %v", err))
	// 	rw.WriteHeader(http.StatusBadRequest)
	// 	return
	// }

	bot := &TheBot{}
	err = bot.InitBot(botRequest.SteamProfile)
	if err != nil {
		fmt.Println("error while initialize bot.", err)
		return
	}

	populateCookies(bot, botRequest.Cookies)
	games := populateGames(botRequest.Games)

	digest, err = runCheck(bot, games)
	if err != nil {
		fmt.Println("error during check.", err)
	}
	return
}

func RunSGBOTFunc(rw http.ResponseWriter, request *http.Request) {
	// get games, cookies and steam profile from db
	// make request suited for checking
	r := &Request{}
	_, err := RunBot(r)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	// write digest to database (digest will send by another function by trigger)

	rw.WriteHeader(http.StatusOK)
}

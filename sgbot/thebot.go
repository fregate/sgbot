package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	scraperapi "github.com/zenrows/zenrows-go-sdk/service/api"
)

var stdlog, errlog *log.Logger

// BotError Description of BOT error
type BotError struct {
	When time.Time
	What string
}

func (e *BotError) Error() string {
	return fmt.Sprintf("at %v, %s", e.When, e.What)
}

// ErrKeysExhausted is returned when a scrapeapi answer is 402/AUTH004
// (zenrows "usage exceeded") and there are no more keys to rotate to:
// the bot has to stop in that case.
var ErrKeysExhausted = errors.New("zenrows keys exhausted (402/AUTH004)")

// GiveAway Definition of GA
type GiveAway struct {
	SGID string
	GID  uint64
	URL  string
	Name string
	Time time.Time
}

// {"type":"success","entry_count":"108","points":"147"}
type postResponse struct {
	Type    string `json:"type"`
	Entries string `json:"entry_count"`
	Points  string `json:"points"`
}

const (
	baseURL             string = "https://www.steamgifts.com"
	sgWishlistURL       string = "/giveaways/search?type=wishlist"
	sgAccountInfo       string = "/giveaways/won"
	steamProfileURL     string = "https://steamcommunity.com/profiles/%s/followedgames/"
)

type ApiResponse[T any] struct {
	Resp T	`json:"response"`
}

func fetchFollowedList(steamID string, apiKey string) (map[uint64]bool, error) {
	url := fmt.Sprintf("https://api.steampowered.com/IStoreService/GetGamesFollowed/v1/?id=%s&steamid=%s", apiKey, steamID)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed process request: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	answer, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("can't read body: %d", resp.StatusCode)
	}

	type Items struct {
		Items []uint64	`json:"appids"`
	}

	games := ApiResponse[Items]{}
	err = json.Unmarshal(answer, &games)
	if err != nil {
		return nil, fmt.Errorf("failed unmarshall json response: %v", err)
	}

	out := make(map[uint64]bool)
	for _, app := range games.Resp.Items {
		out[app] = true
	}

	return out, nil
}

func fetchWishlist(steamID string, apiKey string) (map[uint64]bool, error) {
	url := fmt.Sprintf("https://api.steampowered.com/IWishlistService/GetWishlist/v1?id=%s&steamid=%s", apiKey, steamID)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed process request: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	answer, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("can't read body: %d", resp.StatusCode)
	}

	type GameInfo struct {
		AppID     uint64 `json:"appid"`
		Priority  int    `json:"priority"`
		DateAdded int    `json:"added"`
	}

	type WishlistItems struct {
		Items []GameInfo	`json:"items"`
	}

	games := ApiResponse[WishlistItems]{}
	err = json.Unmarshal(answer, &games)
	if err != nil {
		return nil, fmt.Errorf("failed unmarshall json response: %v", err)
	}

	out := make(map[uint64]bool)
	for _, app := range games.Resp.Items {
		out[app.AppID] = true
	}

	return out, nil
}

// fetchSteamPackApps returns the app ids included in a steam store
// package (sub/PACKID page) using the storefront packagedetails api
// instead of parsing the sub page html.
func fetchSteamPackApps(packID string) ([]uint64, error) {
	url := fmt.Sprintf("https://store.steampowered.com/api/packagedetails?packageids=%s", packID)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed process request: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	answer, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("can't read body: %d", resp.StatusCode)
	}

	type PackApp struct {
		ID uint64 `json:"id"`
	}

	type PackData struct {
		Apps []PackApp `json:"apps"`
	}

	type Pack struct {
		Success bool      `json:"success"`
		Data    *PackData `json:"data"`
	}

	// the api answer is a map keyed by the requested package id
	packs := make(map[string]Pack)
	if err := json.Unmarshal(answer, &packs); err != nil {
		return nil, fmt.Errorf("failed unmarshall json response: %v", err)
	}

	pack, ok := packs[packID]
	if !ok || !pack.Success || pack.Data == nil {
		return nil, fmt.Errorf("no app list for pack %s", packID)
	}

	apps := make([]uint64, 0, len(pack.Data.Apps))
	for _, a := range pack.Data.Apps {
		apps = append(apps, a.ID)
	}

	return apps, nil
}

// isAuth004 reports whether a scrapeapi answer is a zenrows 402/AUTH004
// "usage exceeded" one: the current api key has spent its allowance and
// the next key from the rotor has to be used instead.
func isAuth004(status int, body []byte) bool {
	return status == http.StatusPaymentRequired && strings.Contains(string(body), "AUTH004")
}

// TheBot class for work with SteamGifts pages
type TheBot struct {
	// client
	client  *scraperapi.Client
	cookies []*http.Cookie

	// keys and auth
	steamID string
	steamAPIKey string
	zenrowsKeys *Rotor

	// games
	gamesWhitelist map[uint64]bool

	// digest update
	digest []string
}

// InitBot initilize bot fields, load configs
func (b *TheBot) InitBot(steamProfile string, steamAPIKey string, zenrowsAPIKeys []string) error {
	b.steamID = steamProfile
	b.steamAPIKey = steamAPIKey
	b.gamesWhitelist = make(map[uint64]bool)
	b.digest = make([]string, 0)

	// all zenrows keys from the db go into the rotor,
	// the first key is used until a 402/AUTH004 forces a rotation
	keysRotor, err := NewRotor(zenrowsAPIKeys)
	if err != nil {
		return fmt.Errorf("can't create zenrows keys rotor: %v", err)
	}
	b.zenrowsKeys = keysRotor
	b.client = scraperapi.NewClient(scraperapi.WithAPIKey(b.zenrowsKeys.Value()))

	return nil
}

// rotateKey moves the zenrows key rotor to the next key and recreates
// the scrapeapi client with it. It returns an error when there are no
// more rotations left: the bot has to stop in that case.
func (b *TheBot) rotateKey() error {
	if err := b.zenrowsKeys.Rotate(); err != nil {
		return fmt.Errorf("can't rotate zenrows key: %v", err)
	}

	b.client = scraperapi.NewClient(scraperapi.WithAPIKey(b.zenrowsKeys.Value()))
	stdlog.Println("zenrows key rotated, scrapeapi client recreated")
	return nil
}

func (b *TheBot) getSteamLists() (err error) {
	if b.steamID == "" {
		return &BotError{time.Now(), "steam profile empty"}
	}

	// parse wish list entries
	wg, err := fetchWishlist(b.steamID, b.steamAPIKey)
	if err != nil {
		stdlog.Println("can't fetch steam wishlist", err)
		return &BotError{time.Now(), "can't fetch steam wishlist"}
	}

	stdlog.Println("wishlist entries", len(wg))
	maps.Copy(b.gamesWhitelist, wg)

	// parse followed games entries
	wg, err = fetchFollowedList(b.steamID, b.steamAPIKey)
	if err != nil {
		stdlog.Println("can't fetch followed games", err)
		return &BotError{time.Now(), "can't fetch followed games"}
	}
	stdlog.Println("followed entries", len(wg))
	maps.Copy(b.gamesWhitelist, wg)

	stdlog.Println("steam profile parsed successfully")
	return nil
}

func (b *TheBot) postRequest(path string, queryParams url.Values) (status bool, err error) {
	pageURL, err := url.Parse(baseURL + path)
	if err != nil {
		return
	}

	params := &scraperapi.RequestParameters{
		CustomHeaders: http.Header{
			"Referer": []string{baseURL},
			"User-Agent": []string{"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:153.0) Gecko/20100101 Firefox/153.0"},
			"Origin": []string{baseURL},
			"Content-Type": []string{"application/x-www-form-urlencoded; charset=UTF-8"},
		},
	}

	for _, k := range b.cookies {
		if k.Domain != pageURL.Host {
			continue
		}

		params.CustomHeaders.Add("Cookie", k.String())
	}

	for {
		resp, err := b.client.Post(
			context.Background(),
			pageURL.String(),
			params,
			queryParams.Encode())
		if err != nil {
			return false, err
		}

		stdlog.Println("giveaway post request answer", pageURL.String(), resp.StatusCode(), string(resp.Body()))

		r := postResponse{}
		if isAuth004(resp.StatusCode(), resp.Body()) {
			stdlog.Println("zenrows 402/AUTH004 (usage exceeded) on", pageURL.String())
			if err = b.rotateKey(); err != nil {
				return false, ErrKeysExhausted
			}
			// retry the same request with the new key
			continue
		} else if resp.StatusCode() == http.StatusOK {
			err = json.Unmarshal(resp.Body(), &r)
			return r.Type == "success", err
		} else if resp.StatusCode() == http.StatusUnprocessableEntity {
			stdlog.Printf("internal error (%s)", resp.Body())
			return true, nil
		} else {
			return false, nil
		}
	}
}

func (b *TheBot) getPageCustom(uri string) (retDoc *goquery.Document, err error) {
	pageURL, err := url.Parse(uri)
	if err != nil {
		return
	}

	params := &scraperapi.RequestParameters{
		JSRender:          true,
		WaitForSelector:   "body",
		CustomHeaders:     http.Header{},
	}

	for _, k := range b.cookies {
		if k.Domain != pageURL.Host {
			continue
		}

		params.CustomHeaders.Add("Cookie", k.String())
	}

	for {
		resp, err := b.client.Get(context.Background(), pageURL.String(), params)
		if err != nil {
			return nil, err
		}

		if isAuth004(resp.StatusCode(), resp.Body()) {
			stdlog.Println("zenrows 402/AUTH004 (usage exceeded) on", pageURL.String())
			if err = b.rotateKey(); err != nil {
				return nil, ErrKeysExhausted
			}
			// retry the same page with the new key
			continue
		}

		return goquery.NewDocumentFromReader(bytes.NewReader(resp.Body()))
	}
}

func (b *TheBot) parseToken(str string) string {
	return str[len(str)-32:]
}

func (b *TheBot) setCookies(cookies []*http.Cookie) {
	b.cookies = cookies
}

func (b *TheBot) getToken(doc *goquery.Document) (token string, err error) {
	userName, _ := doc.Find("a.nav__avatar-outer-wrap").First().Attr("href")
	ttt, res := doc.Find("div.js__logout").First().Attr("data-form")
	if res {
		token = b.parseToken(ttt)
	}

	if userName == "" || token == "" {
		return "", &BotError{time.Now(), "no user information. please refresh cookies or parser"}
	}

	return token, nil
}

func (b *TheBot) enterGiveaway(game GiveAway, token string) (status bool, err error) {
	params := url.Values{}
	params.Add("xsrf_token", token)
	params.Add("code", game.SGID)
	params.Add("do", "entry_insert")

	return b.postRequest("/ajax.php", params)
}

func (b *TheBot) getGiveaways(doc *goquery.Document) (giveaways []GiveAway) {
	re := regexp.MustCompile(`[0-9]+`)
	doc.Find("div.giveaway__row-outer-wrap").Each(func(idx int, s *goquery.Selection) {
		sgCode, _ := s.Find("a.giveaway__heading__name").First().Attr("href")
		sgCode = strings.Split(sgCode, "/")[2]

		game := s.Find("a.giveaway__heading__name").First().Text()

		x, ok := s.Find("a.giveaway__icon[target='_blank']").First().Attr("href")
		if !ok {
			errlog.Println("no link?", sgCode)
			return
		}

		// get steamgifts giveaway code (unique url)
		sgURL, ok := s.Find("a.giveaway__heading__name").First().Attr("href")
		if !ok {
			errlog.Println("skip giveaway - can't find url", sgCode)
			return
		}

		// get giveaway timestamp
		y, ok := s.Find("span[data-timestamp]").First().Attr("data-timestamp")
		if !ok {
			errlog.Println("can't parse timestamp for", sgCode)
			return
		}

		t, _ := strconv.ParseInt(y, 10, 64)

		if strings.Contains(x, "/sub/") { // parse sub page
			stdlog.Println("parse 'sub' giveaway", x)
			// get steam pack id from the sub page url
			strpack := re.FindAllString(x, -1)
			if len(strpack) == 0 {
				stdlog.Println("skip giveaway - can't find pack id", x)
				return
			}

			// ask the storefront api for the apps the pack contains
			apps, err := fetchSteamPackApps(strpack[0])
			if err != nil {
				errlog.Println("can't get pack apps", x, err)
				return
			}

			for _, gid := range apps {
				_, ok := b.gamesWhitelist[gid]
				if !ok {
					// stdlog.Println("skip giveaway by whitelist", gid)
					continue
				}

				// add nanoseconds to split giveaways which will be ended on same time
				giveaways = append(giveaways, GiveAway{sgCode, gid, sgURL, game, time.Unix(t, 0)})
				// stop parse sub page - we're decided to be in!
				break
			}
		} else { // parse single game GA
			// get steam game id and check it whitelisted
			strgid := re.FindAllString(x, -1)
			if len(strgid) == 0 {
				stdlog.Println("skip giveaway - can't find steam id", x)
				return
			}
			gid, _ := strconv.ParseUint(strgid[0], 10, 64)
			// stdlog.Println(gid)
			_, ok := b.gamesWhitelist[gid]
			if !ok {
				// stdlog.Println("skip giveaway by whitelist", gid)
				return
			}

			// add nanoseconds to split giveaways which will be ended on same time
			giveaways = append(giveaways, GiveAway{sgCode, gid, sgURL, game, time.Unix(t, 0)})
		}
	})

	// stdlog.Println(giveaways)
	return giveaways
}

func (b *TheBot) processGiveaways(giveaways []GiveAway, token string, period time.Duration) (entries int, err error) {
	if len(giveaways) == 0 {
		return 0, nil
	}

	// sort giveaways by time asc
	sec := func(t1, t2 *GiveAway) bool {
		return t1.Time.UnixNano() < t2.Time.UnixNano()
	}
	By(sec).sortGAs(giveaways)

	timeNow := time.Now().Add(period)
	for _, game := range giveaways {
		if game.Time.After(timeNow) {
			stdlog.Println("enough parsing", game)
			break
		}

		// add some human behaviour - pause bot for a few seconds (1-3)
		d := time.Second * time.Duration(rand.Intn(3) + 1)
		if game.Time.After(time.Now().Add(d)) {
			time.Sleep(d)
		}

		status, err := b.enterGiveaway(game, token)
		if err != nil {
			if errors.Is(err, ErrKeysExhausted) {
				return 0, err
			}
			stdlog.Printf("internal error (%s) when enter for [%+v]", err, game)
			continue
		}
		if !status {
			stdlog.Printf("external error when enter for [%+v]. wait\n", game)
			break
		}
		duration := game.Time.Sub(time.Now())
		timeDesc := fmt.Sprintf("Draw in %.f hour(s)", duration.Hours())
		if duration.Minutes() < 60 {
			timeDesc = fmt.Sprintf("Draw in %.f minutes", duration.Minutes())
		}

		b.addDigest(fmt.Sprintf("%s. Apply for %d : %s. %s", time.Now().Format("15:04:05"), game.GID, game.Name, timeDesc))
		entries = entries + 1
	}

	return entries, nil
}

func (b *TheBot) parseGiveaways(externalGamesList map[uint64]bool) (err error) {
	b.gamesWhitelist = externalGamesList
	err = b.getSteamLists()
	if err != nil {
		return
	}

	if len(b.gamesWhitelist) == 0 {
		stdlog.Println("there is no game you want to win, please add some in json list or steam account. bye")
		return errors.New("empty white list")
	}

	stdlog.Println("check wishlist")
	doc, err := b.getPageCustom(baseURL + sgWishlistURL)
	if err != nil {
		return
	}
	token, err := b.getToken(doc)
	if err != nil {
		return
	}

	giveaways := b.getGiveaways(doc)
	stdlog.Println("found giveaways on page:", len(giveaways))
	entriesWishlist, err := b.processGiveaways(giveaways, token, time.Hour * 24 * 7 * 5) // 5 weeks - all
	if err != nil {
		return
	}
	stdlog.Println("processed giveaways", entriesWishlist)

	stdlog.Println("check main page")
	doc, err = b.getPageCustom(baseURL)
	if err != nil {
		return
	}
	token, err = b.getToken(doc)
	if err != nil {
		return
	}

	giveaways = b.getGiveaways(doc)
	stdlog.Println("found giveaways on page:", len(giveaways))
	entriesMainPage, err := b.processGiveaways(giveaways, token, time.Hour)
	if err != nil {
		return
	}

	defer stdlog.Printf("processed giveaways (w: %d, m: %d)", entriesWishlist, entriesMainPage)

	return nil
}

func (b *TheBot) addDigest(msg string) {
	b.digest = append(b.digest, msg)
}

func init() {
	stdlog = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	errlog = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds)
}

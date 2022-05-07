package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
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

// GiveAway Definition of GA
type GiveAway struct {
	SGID string
	GID  uint64
	URL  string
	Name string
	Time time.Time
}

// By is the type of a "less" function that defines the ordering of its Time arguments.
// time sorter function
type By func(p1, p2 *GiveAway) bool

func (by By) sortGAs(entries []GiveAway) {
	ps := &timeSorter{
		entries: entries,
		by:      by, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(ps)
}

// timeSorter joins a By function and a slice of Time to be sorted.
type timeSorter struct {
	entries []GiveAway
	by      func(p1, p2 *GiveAway) bool // Closure used in the Less method.
}

// Len is part of sort.Interface.
func (s *timeSorter) Len() int {
	return len(s.entries)
}

// Swap is part of sort.Interface.
func (s *timeSorter) Swap(i, j int) {
	s.entries[i], s.entries[j] = s.entries[j], s.entries[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *timeSorter) Less(i, j int) bool {
	return s.by(&s.entries[i], &s.entries[j])
}

// {"type":"success","entry_count":"108","points":"147"}
type postResponse struct {
	Type    string `json:"type"`
	Entries string `json:"entry_count"`
	Points  string `json:"points"`
}

type pair struct {
	name  string
	value string
}

var requestHeaders = []pair{
	pair{name: "Accept", value: "application/json, text/javascript, */*; q=0.01"},
	pair{name: "Content-Type", value: "application/x-www-form-urlencoded; charset=UTF-8"},
	pair{name: "User-Agent", value: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/59.0.3071.115 Safari/537.36"},
	pair{name: "X-Requested-With", value: "XMLHttpRequest"},
}

type wginfo struct {
	AppID     int `json:"appid"`
	Priority  int `json:"priority"`
	DateAdded int `json:"added"`
}

const (
	baseURL             string = "https://www.steamgifts.com"
	sgWishlistURL       string = "/giveaways/search?type=wishlist"
	sgAccountInfo       string = "/giveaways/won"
	baseSteamProfileURL string = "https://steamcommunity.com/id/"
	steamWishlist       string = "/wishlist/"
	steamFollowed       string = "/followedgames/"
)

// TheBot class for work with SteamGifts pages
type TheBot struct {
	token string

	// http client
	client  *http.Client
	cookies []*http.Cookie
	// TODO move to botdaemon
	// cookiesFileModifiedTime time.Time

	steamProfile string

	gamesWhitelist map[uint64]bool
	gamesWon       []uint64

	// page cache
	currentDocument *goquery.Document
	currentURL      string

	enteredGiveAways []string
}

func (b *TheBot) clean() {
	b.token = ""
	b.currentURL = ""
}

// InitBot initilize bot fields, load configs
func (b *TheBot) InitBot(steamProfile string) error {
	b.clean()

	b.steamProfile = steamProfile
	b.gamesWhitelist = make(map[uint64]bool)
	b.enteredGiveAways = make([]string, 0)

	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return err
	}

	b.client = &http.Client{Jar: jar}
	return nil
}

func (b *TheBot) getSteamLists() (err error) {
	if b.steamProfile == "" {
		return &BotError{time.Now(), "steam profile empty"}
	}

	// parse wish list entries
	_, doc, err := b.getPageCustom(baseSteamProfileURL + b.steamProfile + steamWishlist)
	if err != nil {
		stdlog.Println(err)
	} else {
		doc.Find("script").Each(func(_ int, s *goquery.Selection) {
			if idx := strings.Index(s.Text(), "g_rgWishlistData"); idx >= 0 {
				parseText := s.Text()[idx:]
				idx = strings.Index(parseText, "[{")
				idxEnd := strings.Index(parseText, "}];")
				if idx < 0 || idxEnd < 0 {
					stdlog.Println("parseText", parseText)
					stdlog.Println("indeces", idx, idxEnd)
					return
				}
				ww := []wginfo{}
				err = json.Unmarshal([]byte(parseText[idx:idxEnd+2]), &ww)
				if err == nil {
					stdlog.Println("wishlist entries", len(ww))
					for arridx := range ww {
						id := uint64(ww[arridx].AppID)
						b.gamesWhitelist[id] = true
					}
				} else {
					stdlog.Println(err)
				}
			}
		})
	}

	// parse followed games entries
	_, doc, err = b.getPageCustom(baseSteamProfileURL + b.steamProfile + steamFollowed)
	doc.Find("div[data-appid]").Each(func(_ int, s *goquery.Selection) {
		id, _ := s.Attr("data-appid")
		numID, _ := strconv.ParseUint(id, 10, 64)
		b.gamesWhitelist[numID] = true
	})

	stdlog.Println("steam profile parsed successfully")
	return nil
}

func (b *TheBot) postRequest(path string, params url.Values) (status bool, err error) {
	pageURL, err := url.Parse(baseURL + path)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, pageURL.String(), bytes.NewBufferString(params.Encode()))
	if err != nil {
		return
	}

	for _, h := range requestHeaders {
		req.Header.Add(h.name, h.value)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	answer, err := ioutil.ReadAll(resp.Body)
	stdlog.Println("giveaway post request answer", string(answer))
	if err != nil {
		return
	}

	r := postResponse{}
	err = json.Unmarshal(answer, &r)

	return r.Type == "success", err
}

func (b *TheBot) getPageCustom(uri string) (retPath string, retDoc *goquery.Document, err error) {
	pageURL, err := url.Parse(uri)
	if err != nil {
		return
	}

	req, err := http.NewRequest("GET", pageURL.String(), nil)
	if err != nil {
		return
	}

	for _, h := range requestHeaders {
		req.Header.Add(h.name, h.value)
	}

	for _, k := range b.cookies {
		if k.Domain != pageURL.Host {
			continue
		}

		req.AddCookie(k)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	retPath = pageURL.String()
	retDoc, err = goquery.NewDocumentFromReader(resp.Body)

	return
}

func (b *TheBot) getPage(path string) (err error) {
	if b.currentURL == baseURL+path {
		return nil
	}

	b.currentURL, b.currentDocument, err = b.getPageCustom(baseURL + path)
	return err
}

func (b *TheBot) parseToken(str string) string {
	return str[len(str)-32:]
}

func (b *TheBot) setCookies(cookies []*http.Cookie) {
	b.cookies = cookies
}

func (b *TheBot) getUserInfo() (err error) {
	err = b.getPage(sgAccountInfo)
	if err != nil {
		return
	}

	b.token = ""
	points := 0

	userName, _ := b.currentDocument.Find("a.nav__avatar-outer-wrap").First().Attr("href")
	ttt, res := b.currentDocument.Find("div.js__logout").First().Attr("data-form")
	if res {
		b.token = b.parseToken(ttt)
		points, _ = strconv.Atoi(b.currentDocument.Find("span.nav__points").First().Text())
	}

	if userName == "" || b.token == "" {
		return &BotError{time.Now(), "no user information. please refresh cookies or parser"}
	}

	stdlog.Printf("receive info [user:%s][pts:%d]\n", userName, points)

	b.gamesWon = make([]uint64, 0)
	if b.currentDocument.Find("div.nav__notification").First() != nil { // won something
		b.currentDocument.Find("div.table__row-inner-wrap").Each(func(_ int, s *goquery.Selection) {
			if s.Find("div[class='table__gift-feedback-received is-hidden']").Size() != 0 {
				// steam id
				steamid, _ := s.Find("a.table_image_thumbnail").First().Attr("style")
				// background-image:url(https://steamcdn-a.akamaihd.net/steam/apps/265930/capsule_184x69.jpg); - [5]
				n, _ := strconv.ParseUint(strings.Split(steamid, "/")[5], 10, 64)

				b.gamesWon = append(b.gamesWon, n)
			}
		})
	}

	if len(b.gamesWon) > 0 {
		stdlog.Println("you've won", b.gamesWon)
	}

	return nil
}

func (b *TheBot) checkWonList(gid uint64) bool {
	if len(b.gamesWon) == 0 {
		return false
	}

	for _, v := range b.gamesWon {
		if v == gid {
			return true
		}
	}

	return false
}

func (b *TheBot) getGiveawayStatus(path string) (status bool, err error) {
	_, doc, err := b.getPageCustom(baseURL + path)
	if err != nil {
		return true, err
	}

	sel := doc.Find("div.widget-container")
	if sel.Size() == 0 {
		return true, &BotError{time.Now(), "strange page " + path}
	}

	sel = doc.Find("div.sidebar__error")
	if sel.Size() != 0 {
		return false, &BotError{time.Now(), "not enough points"}
	}

	sel = doc.Find("div.sidebar__entry-insert")
	// no buttons - exist or not enough points
	if sel.Size() == 0 {
		return false, nil
	}

	// skip already entered
	result := true
	sel.EachWithBreak(func(i int, s *goquery.Selection) bool {
		class, _ := s.Attr("class")
		result = !strings.Contains(class, "is-hidden")
		return result
	})

	return result, nil
}

func (b *TheBot) enterGiveaway(game GiveAway) (status bool, err error) {
	params := url.Values{}
	params.Add("xsrf_token", b.token)
	params.Add("code", game.SGID)
	params.Add("do", "entry_insert")

	return b.postRequest("/ajax.php", params)
}

func (b *TheBot) getGiveaways(doc *goquery.Document) (giveaways []GiveAway) {
	doc.Find("div.giveaway__row-outer-wrap").Each(func(idx int, s *goquery.Selection) {
		sgCode, ok := s.Find("a.giveaway__heading__name").First().Attr("href")
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
			_, subDoc, err := b.getPageCustom(x)
			if err != nil {
				errlog.Println("can't get page", x)
				return
			}

			// doc := subDoc.Find("html")
			// html, eee := doc.Html()
			// if eee != nil {
			// 	log.Fatal(err)
			// }
			// stdlog.Printf("[[[[%s]]]]", html)

			subDoc.Find("div.tab_item").EachWithBreak(func(subIdx int, subs *goquery.Selection) bool {
				subID, subOk := subs.Attr("data-ds-appid")
				if !subOk {
					return true
				}

				gid, _ := strconv.ParseUint(subID, 10, 64)

				_, ok := b.gamesWhitelist[gid]
				if !ok {
					// stdlog.Println("skip giveaway by whitelist", gid)
					return true
				}

				if b.checkWonList(gid) {
					// stdlog.Println("skip - already won! receve your gift!")
					return true
				}

				// add nanoseconds to split giveaways which will be ended at one time
				giveaways = append(giveaways, GiveAway{sgCode, gid, sgURL, game, time.Unix(t, 0)})
				// stop parse sub page - we're decided to be in!
				return false
			})
		} else { // parse single game GA
			// get steam game id and check it whitelisted
			gid, _ := strconv.ParseUint(strings.Trim(x[strings.LastIndex(strings.Trim(x, "/"), "/")+1:], "/"), 10, 64)
			// stdlog.Println(gid)
			_, ok := b.gamesWhitelist[gid]
			if !ok {
				// stdlog.Println("skip giveaway by whitelist", gid)
				return
			}

			if b.checkWonList(gid) {
				// stdlog.Println("skip - already won! receve your gift!")
				return
			}

			// add nanoseconds to split giveaways which will be ended at one time
			giveaways = append(giveaways, GiveAway{sgCode, gid, sgURL, game, time.Unix(t, 0)})
		}
	})

	// stdlog.Println(giveaways)
	return giveaways
}

func (b *TheBot) processGiveaways(giveaways []GiveAway, period time.Duration) (count, entries int) {
	if len(giveaways) == 0 {
		return
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

		status, err := b.getGiveawayStatus(game.URL)
		if err != nil {
			stdlog.Println(err)
			if !status { // not enough points
				break
			}
		}

		if !status {
			continue
		}

		// add some human behaviour - pause bot for a few seconds (3-6)
		d := time.Second * time.Duration(rand.Intn(3)+3)
		if game.Time.After(time.Now().Add(d)) {
			time.Sleep(d)
		}

		status, err = b.enterGiveaway(game)
		if err != nil {
			stdlog.Printf("internal error (%s) when enter for [%+v]", err, game)
			continue
		}
		if !status {
			stdlog.Printf("external error when enter for [%+v]. wait\n", game)
			count = count + 1
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

	return count, entries
}

func (b *TheBot) parseGiveaways(externalGamesList map[uint64]bool) (count int, err error) {
	b.gamesWhitelist = externalGamesList
	err = b.getSteamLists()
	if err != nil {
		return
	}

	if len(b.gamesWhitelist) == 0 {
		stdlog.Println("there is no game you want to win, please add some in json list or steam account. bye")
		return 0, errors.New("math: square root of negative number")
	}

	err = b.getPage("/")
	if err != nil {
		return
	}

	stdlog.Println("check wishlist")
	_, doc, err := b.getPageCustom(baseURL + sgWishlistURL)
	if err != nil {
		return 0, err
	}
	giveaways := b.getGiveaways(doc)
	stdlog.Println("found giveaways on page:", len(giveaways))
	count, entriesWishlist := b.processGiveaways(giveaways, time.Hour*24*7*5) // 5 weeks - all
	stdlog.Println("processed giveaways", entriesWishlist)

	stdlog.Println("check main page")
	giveaways = b.getGiveaways(b.currentDocument)
	stdlog.Println("found giveaways on page:", len(giveaways))
	count, entriesMainPage := b.processGiveaways(giveaways, time.Hour)

	defer stdlog.Println("processed giveaways", entriesWishlist+entriesMainPage)

	return count, nil
}

func (b *TheBot) addDigest(msg string) {
	b.enteredGiveAways = append(b.enteredGiveAways, msg)
}

func init() {
	stdlog = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	errlog = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds)
}

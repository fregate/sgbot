package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	gomail "gopkg.in/gomail.v2"

	"config"

	"github.com/PuerkitoBio/goquery"
)

type BotError struct {
	When time.Time
	What string
}

func (e *BotError) Error() string {
	return fmt.Sprintf("at %v, %s", e.When, e.What)
}

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

// Sort is a method on the function type, By, that sorts the argument slice according to the function.
func (by By) Sort(entries []GiveAway) {
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

type mailinfo struct {
	SMTPServer       string `json:"smtp"`
	Port             int    `json:"port"`
	SMTPUsername     string `json:"username"`
	SMTPUserpassword string `json:"password"`
}

type cfg struct {
	SteamProfile string `json:"profile"`
	SendDigest   bool   `json:"digest"`

	SMTPSettings    mailinfo `json:"mail"`
	EmailSubjectTag string   `json:"subjecttag"`
	EmailRecipient  string   `json:"recipient"`
}

func (c mailinfo) isValid() bool {
	return c.Port != 0 && c.SMTPServer != "" && c.SMTPUsername != ""
}

func (c cfg) isMailValid() bool {
	return c.SMTPSettings.isValid() && c.EmailRecipient != ""
}

// {"type":"success","entry_count":"108","points":"147"}
type PostResponse struct {
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
	//pair{name: "Accept-Encoding", value: "gzip, deflate, br"},
	pair{name: "Content-Type", value: "application/x-www-form-urlencoded; charset=UTF-8"},
	pair{name: "User-Agent", value: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/59.0.3071.115 Safari/537.36"},
	pair{name: "X-Requested-With", value: "XMLHttpRequest"},
}

type wginfo struct {
	AppId     int  `json:"appid"`
	Priority  int  `json:"priority"`
	DateAdded int  `json:"added"`
}

const (
	baseURL             string = "https://www.steamgifts.com"
	sgWishlistURL       string = "/giveaways/search?type=wishlist"
	sgAccountInfo       string = "/giveaways/won"
	baseSteamProfileURL string = "https://steamcommunity.com/id/"
	steamWishlist       string = "/wishlist/"
	steamFollowed       string = "/followedgames/"
)

// TheBot - class for work with SteamGifts pages
type TheBot struct {
	userName string
	token    string
	points   int
	baseURL  *url.URL

	// http client
	client                  *http.Client
	cookiesFileModifiedTime time.Time

	botConfig         cfg
	dialer            *gomail.Dialer
	gamesListFileName string
	configFileName    string
	cookiesFileName   string

	gamesWhitelist map[uint64]string
	gamesWon       []uint64

	// page cache
	currentDocument *goquery.Document
	currentURL      string

	// digest
	digestMsgs         []string
	lastTimeDigestSent time.Time
}

func (b *TheBot) clean() {
	b.userName = ""
	b.token = ""
	b.currentURL = ""
}

func (b *TheBot) createClient() error {
	fi, err := os.Stat(b.cookiesFileName)
	if b.cookiesFileModifiedTime.After(fi.ModTime()) {
		return nil
	}

	var ccc interface{}
	err = config.ReadConfig(b.cookiesFileName, &ccc)
	if err != nil {
		return err
	}

	var cookies []*http.Cookie
	m := ccc.(map[string]interface{})
	for k, v := range m {
		cookies = append(cookies, &http.Cookie{Name: k, Value: v.(string)})
	}

	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return err
	}

	jar.SetCookies(b.baseURL, cookies)
	b.client = &http.Client{
		Jar: jar,
	}

	b.cookiesFileModifiedTime = fi.ModTime()
	return nil
}

// InitBot initilize bot fields, load configs
func (b *TheBot) InitBot(configFile, cookieFile, listFile string) (err error) {
	// init bot class default fields
	b.clean()

	b.baseURL, err = url.Parse(baseURL)

	b.lastTimeDigestSent = time.Now()
	b.digestMsgs = make([]string, 0)

	b.gamesListFileName, b.cookiesFileName, b.configFileName = listFile, cookieFile, configFile

	// read steam profile, parse wishlist and followed games (also read mail-smtp settings)
	{
		err = config.ReadConfig(configFile, &b.botConfig)
		if err != nil {
			stdlog.Println(err)
		}

		if b.botConfig.isMailValid() {
			b.dialer = gomail.NewDialer(b.botConfig.SMTPSettings.SMTPServer, b.botConfig.SMTPSettings.Port, b.botConfig.SMTPSettings.SMTPUsername, b.botConfig.SMTPSettings.SMTPUserpassword)
		}
	}

	return
}

func (b *TheBot) readGameLists(listFile string) (err error) {
	b.gamesWhitelist = make(map[uint64]string)

	if b.botConfig.SteamProfile != "" {
		err = b.getSteamLists()
		if err != nil {
			stdlog.Println(err)
		}
	}

	var ccc interface{}

	err = config.ReadConfig(listFile, &ccc)
	if err == nil {
		m := ccc.(map[string]interface{})
		for k, v := range m {
			q, err := strconv.ParseUint(k, 10, 32)
			if err != nil {
				break
			}
			b.gamesWhitelist[q] = v.(string)
		}

		if len(b.gamesWhitelist) == 0 {
			stdlog.Println("there is no game you want to win, please add some in json list or steam account. bye")
			return &BotError{time.Now(), "no games to win"}
		}
	} else {
		stdlog.Println("err %v", err)
	}

	stdlog.Printf("successfully load games list [total entries:%d]\n", len(b.gamesWhitelist))
	return nil
}

// SendMail bot can send some mail (TODO need refactor!!!)
func (b *TheBot) sendMail(subject, msg string) (err error) {
	if b.dialer == nil {
		return nil
	}

	if msg == "" {
		return nil
	}

	m := gomail.NewMessage()
	m.SetHeader("From", b.botConfig.SMTPSettings.SMTPUsername)
	m.SetHeader("To", b.botConfig.EmailRecipient)
	m.SetHeader("Subject", fmt.Sprintf("%s %s", b.botConfig.EmailSubjectTag, subject))
	m.SetBody("text/plain", msg)

	err = b.dialer.DialAndSend(m)
	if err != nil {
		errlog.Println(err)
	}
	return
}

func (b *TheBot) getSteamLists() (err error) {
	if b.botConfig.SteamProfile == "" {
		return &BotError{time.Now(), "steam profile empty"}
	}

	// parse wish list entries
	_, doc, err := b.getPageCustom(baseSteamProfileURL + b.botConfig.SteamProfile + steamWishlist)
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
				ww := []wginfo {}
				err = json.Unmarshal([]byte(parseText[idx:idxEnd+2]), &ww)
				if err == nil {
					stdlog.Println("wishlist entries", len(ww))
					for arridx := range ww {
						id := uint64(ww[arridx].AppId)
						b.gamesWhitelist[id] = fmt.Sprintf("Whishlist %s", id)
						stdlog.Println("wihlist entry", id)
					}
				} else {
					stdlog.Println(err)
				}
			}
		})
	}

	// parse followed games entries
	_, doc, err = b.getPageCustom(baseSteamProfileURL + b.botConfig.SteamProfile + steamFollowed)
	doc.Find("div[data-appid]").Each(func(_ int, s *goquery.Selection) {
		id, _ := s.Attr("data-appid")
		numID, _ := strconv.ParseUint(id, 10, 64)
		name := s.Find("div.gameListRowItemName > a").Text()
		b.gamesWhitelist[numID] = name
	})

	stdlog.Println("steam profile parsed successfully")
	return nil
}

func (b *TheBot) postRequest(path string, params url.Values) (status bool, pts string, err error) {
	pageURL, err := url.Parse(b.baseURL.String() + path)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", pageURL.String(), bytes.NewBufferString(params.Encode()))
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

	r := PostResponse{}
	err = json.Unmarshal(answer, &r)

	return r.Type == "success", r.Points, err
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
	if b.currentURL == b.baseURL.String()+path {
		return nil
	}

	b.currentURL, b.currentDocument, err = b.getPageCustom(b.baseURL.String() + path)

	return err
}

func (b *TheBot) parseToken(str string) string {
	return str[len(str)-32:]
}

func (b *TheBot) getUserInfo() (err error) {
	err = b.getPage(sgAccountInfo)
	if err != nil {
		return
	}

	//	doc := b.currentDocument.Find("html")
	//	html, eee := doc.Html()
	//	if eee != nil {
	//		log.Fatal(err)
	//	}
	//	stdlog.Printf("[[[[%s]]]]", html)

	b.userName = ""
	b.token = ""

	b.userName, _ = b.currentDocument.Find("a.nav__avatar-outer-wrap").First().Attr("href")
	ttt, res := b.currentDocument.Find("div.js__logout").First().Attr("data-form")
	if res == true {
		b.token = b.parseToken(ttt)
		b.points, _ = strconv.Atoi(b.currentDocument.Find("span.nav__points").First().Text())
	}

	if b.userName == "" || b.token == "" {
		return &BotError{time.Now(), "no user information. please refresh cookies or parser"}
	}

	stdlog.Printf("receive info [user:%s][pts:%d]\n", b.userName, b.points)

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
	_, doc, err := b.getPageCustom(b.baseURL.String() + path)
	if err != nil {
		return true, err
	}

	sel := doc.Find("div.sidebar--wide").First()
	if sel.Size() == 0 {
		return true, &BotError{time.Now(), "strange page " + path}
	}

	sel = doc.Find("div.sidebar__error")
	if sel.Size() != 0 {
		return false, &BotError{time.Now(), "not enough points"}
	}

	sel = doc.Find("div[data-do='entry_insert']")
	// no buttons - exist or not enough points
	if sel.Size() == 0 {
		return false, nil
	}

	result := true
	sel.EachWithBreak(func(i int, s *goquery.Selection) bool {
		class, _ := s.Attr("class")
		if class == "" || strings.Contains(class, "is-hidden") {
			result = false
			return false
		}
		return true
	})

	return result, nil
}

func (b *TheBot) enterGiveaway(g GiveAway) (status bool, pts string, err error) {
	params := url.Values{}
	params.Add("xsrf_token", b.token)
	params.Add("code", g.SGID)
	params.Add("do", "entry_insert")

	return b.postRequest("/ajax.php", params)
}

func (b *TheBot) getGiveaways(doc *goquery.Document) (giveaways []GiveAway) {
	doc.Find("div.giveaway__row-outer-wrap").Each(func(idx int, s *goquery.Selection) {
		sgCode, ok := s.Find("a.giveaway__heading__name").First().Attr("href")
		sgCode = strings.Split(sgCode, "/")[2]

		x, ok := s.Find("a.giveaway__icon[rel='nofollow']").First().Attr("href")
		if !ok {
			errlog.Println("no link?", sgCode)
			return
		}

		// TODO there is app/NUM and sub/NUM need to work with it somehow
		if strings.Contains(x, "/sub/") {
			// stdlog.Println("skip sub giveaway", x)
			return
		}

		// get steam game id and check it whitelisted
		gid, _ := strconv.ParseUint(strings.Trim(x[strings.LastIndex(strings.Trim(x, "/"), "/")+1:len(x)], "/"), 10, 64)
		// stdlog.Println(gid)
		game, ok := b.gamesWhitelist[gid]
		if !ok {
			// stdlog.Println("skip giveaway by whitelist", gid)
			return
		}

		if b.checkWonList(gid) {
			// stdlog.Println("skip - already won! receve your gift!")
			return
		}

		// get steamgifts giveaway code (unique url)
		sgURL, ok := s.Find("a.giveaway__heading__name").First().Attr("href")
		if !ok {
			errlog.Println("skip giveaway - can't find url", gid)
			return
		}

		// get giveaway timestamp
		y, ok := s.Find("span[data-timestamp]").First().Attr("data-timestamp")
		if !ok {
			errlog.Println("can't parse timestamp for", gid)
			return
		}

		t, _ := strconv.ParseInt(y, 10, 64)

		// add nanoseconds to split giveaways which will be ended at one time
		giveaways = append(giveaways, GiveAway{sgCode, gid, sgURL, game, time.Unix(t, 0)})
	})

	// stdlog.Println(giveaways)
	return giveaways
}

func (b *TheBot) processGiveaways(giveaways []GiveAway, period time.Duration) (count, entries int) {
	// sort giveaways by time asc
	sec := func(t1, t2 *GiveAway) bool {
		return t1.Time.UnixNano() < t2.Time.UnixNano()
	}
	By(sec).Sort(giveaways)

	strpts := ""
	for _, g := range giveaways {
		if g.Time.After(time.Now().Add(period)) {
			stdlog.Println("enough parsing", g)
			break
		}

		status, err := b.getGiveawayStatus(g.URL)
		if err != nil {
			stdlog.Println(err)
			if !status { // not enough points
				count = count + 1
				break
			}
		}

		if !status {
			continue
		}

		// add some human behaviour - pause bot for a few seconds (3-6)
		d := time.Second * time.Duration(rand.Intn(3)+3)
		if g.Time.After(time.Now().Add(d)) {
			time.Sleep(d)
		}

		status, strpts, err = b.enterGiveaway(g)
		if err != nil {
			stdlog.Printf("internal error (%s) when enter for [%+v]", err, g)
			continue
		}
		if !status {
			stdlog.Printf("external error when enter for [%+v]. wait\n", g)
			count = count + 1
			break
		}
		var timeDesc string
		duration := g.Time.Sub(time.Now())
		if duration.Minutes() < 60 {
			timeDesc = fmt.Sprintf("Draw in %.f minutes", duration.Minutes())
		} else {
			timeDesc = fmt.Sprintf("Draw in %.f hour(s)", duration.Hours())
		}
		b.addDigest(fmt.Sprintf("%s. Apply for %d : %s. %s", time.Now().Format("15:04:05"), g.GID, g.Name, timeDesc))
		b.points, _ = strconv.Atoi(strpts)
		entries = entries + 1
	}

	return count, entries
}

func (b *TheBot) parseGiveaways() (count int, err error) {
	err = b.getPage("/")
	if err != nil {
		return
	}

	stdlog.Println("check wishlist")
	_, doc, err := b.getPageCustom(b.baseURL.String() + sgWishlistURL)
	if err != nil {
		return 0, err
	}
	var entries int
	giveaways := b.getGiveaways(doc)
	stdlog.Println("found giveaways on page:", len(giveaways))
	count, entries = b.processGiveaways(giveaways, time.Hour*24*7*5) // 5 weeks - all
	stdlog.Println("processed giveaways", entries)

	stdlog.Println("check main page")
	giveaways = b.getGiveaways(b.currentDocument)
	stdlog.Println("found giveaways on page:", len(giveaways))
	count, entries = b.processGiveaways(giveaways, time.Hour)

	defer stdlog.Println("processed giveaways", entries)

	return count, nil
}

func (b *TheBot) addDigest(msg string) {
	if !b.botConfig.SendDigest {
		return
	}

	b.digestMsgs = append(b.digestMsgs, msg)
}

func (b *TheBot) joinDigest() string {
	if len(b.digestMsgs) == 0 {
		return ""
	}

	return strings.Join(b.digestMsgs, "\n")
}

// SendPanicMsg sends msg (usually error and stops service) + current digest
func (b *TheBot) SendPanicMsg(msg string) {
	if b.botConfig.SendDigest {
		msg = msg + "\n\n" + b.joinDigest()
	}

	b.sendMail("Panic Message!", msg)
}

func (b *TheBot) sendDigest() {
	if !b.botConfig.SendDigest {
		return
	}

	if time.Now().Hour() == 0 || time.Now().Sub(b.lastTimeDigestSent) > time.Hour*24 {
		stdlog.Println("sending digest")
		b.sendMail("Daily digest", b.joinDigest())
		b.lastTimeDigestSent = time.Now()
		b.digestMsgs = make([]string, 0)
	}
}

// Check - check page and enter for gifts (repeat by timeout)
func (b *TheBot) Check() (count int, err error) {
	stdlog.Println("bot checking...")

	defer b.clean()

	err = b.createClient()
	if err != nil {
		return
	}

	err = b.readGameLists(b.gamesListFileName)
	if err != nil {
		return
	}

	err = b.getUserInfo()
	if err != nil {
		return
	}

	defer stdlog.Println("bot check finished")
	defer b.sendDigest()

	// parse main page
	return b.parseGiveaways()
}

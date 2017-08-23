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
	Url  string
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
	SmtpServer       string `json:"smtp"`
	Port             int    `json:"port"`
	SmtpUsername     string `json:"username"`
	SmtpUserpassword string `json:"password"`
}

type cfg struct {
	SteamProfile string `json:"profile"`
	SendDigest   bool   `json:"digest"`

	SmtpSettings    mailinfo `json:"mail"`
	EmailSubjectTag string   `json:"subjecttag"`
	EmailRecipient  string   `json:"recipient"`
}

func (c mailinfo) isValid() bool {
	return c.Port != 0 && c.SmtpServer != "" && c.SmtpUsername != ""
}

func (c cfg) isMailValid() bool {
	return c.SmtpSettings.isValid() && c.EmailRecipient != ""
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

const (
	baseUrl             string = "https://www.steamgifts.com"
	sgWishlistUrl		string = "/giveaways/search?type=wishlist"
	baseSteamProfileUrl string = "http://steamcommunity.com/id/"
	steamWishlist       string = "/wishlist/"
	steamFollowed       string = "/followedgames/"
)

// TheBot - class for work with SteamGifts pages
type TheBot struct {
	userName string
	token    string
	points   int
	baseUrl  *url.URL
	client   *http.Client

	botConfig         cfg
	dialer            *gomail.Dialer
	gamesListFileName string
	configFileName    string
	cookiesFileName   string

	gamesWhitelist map[uint64]string

	// page cache
	currentDocument *goquery.Document
	currentUrl      string

	// digest
	digestMsgs         []string
	lastTimeDigestSent time.Time
}

func (b *TheBot) clean() {
	b.userName = ""
	b.token = ""
	b.currentUrl = ""
}

// InitBot initilize bot fields, load configs
func (b *TheBot) InitBot(configFile, cookieFile, listFile string) (err error) {
	// init bot class default fields
	b.clean()

	b.baseUrl, err = url.Parse(baseUrl)

	b.lastTimeDigestSent = time.Now()
	b.digestMsgs = make([]string, 0)

	b.gamesListFileName, b.cookiesFileName, b.configFileName = listFile, cookieFile, configFile

	// read cookies, create client
	{
		var ccc interface{}
		err = config.ReadConfig(cookieFile, &ccc)
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

		jar.SetCookies(b.baseUrl, cookies)
		b.client = &http.Client{
			Jar: jar,
		}
	}

	// read steam profile, parse wishlist and followed games (also read mail-smtp settings)
	{
		err = config.ReadConfig(configFile, &b.botConfig)
		if err != nil {
			stdlog.Println(err)
		}

		if b.botConfig.isMailValid() {
			b.dialer = gomail.NewDialer(b.botConfig.SmtpSettings.SmtpServer, b.botConfig.SmtpSettings.Port, b.botConfig.SmtpSettings.SmtpUsername, b.botConfig.SmtpSettings.SmtpUserpassword)
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
	m.SetHeader("From", b.botConfig.SmtpSettings.SmtpUsername)
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

	_, doc, err := b.getPageCustom(baseSteamProfileUrl + b.botConfig.SteamProfile + steamWishlist)
	if err != nil {
		return
	}

	doc.Find("div[id^='game']").Each(func(_ int, s *goquery.Selection) {
		id, _ := s.Attr("id")
		id = strings.Split(id, "_")[1]
		numId, _ := strconv.ParseUint(id, 10, 64)
		name := s.Find("h4.ellipsis").Text()
		b.gamesWhitelist[numId] = name
	})

	_, doc, err = b.getPageCustom(baseSteamProfileUrl + b.botConfig.SteamProfile + steamFollowed)
	doc.Find("div[data-appid]").Each(func(_ int, s *goquery.Selection) {
		id, _ := s.Attr("data-appid")
		numId, _ := strconv.ParseUint(id, 10, 64)
		name := s.Find("div.gameListRowItemName > a").Text()
		b.gamesWhitelist[numId] = name
	})

	stdlog.Println("steam profile parsed successfully")
	return nil
}

func (b *TheBot) postRequest(path string, params url.Values) (status bool, pts string, err error) {
	pageUrl, err := url.Parse(b.baseUrl.String() + path)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", pageUrl.String(), bytes.NewBufferString(params.Encode()))
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
	pageUrl, err := url.Parse(uri)
	if err != nil {
		return
	}

	req, err := http.NewRequest("GET", pageUrl.String(), nil)
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

	retPath = pageUrl.String()
	retDoc, err = goquery.NewDocumentFromReader(resp.Body)

	return
}

func (b *TheBot) getPage(path string) (err error) {
	if b.currentUrl == b.baseUrl.String()+path {
		return nil
	}

	b.currentUrl, b.currentDocument, err = b.getPageCustom(b.baseUrl.String() + path)

	return err
}

func (b *TheBot) getUserInfo() (err error) {
	err = b.getPage("/")
	if err != nil {
		return
	}

	b.userName, _ = b.currentDocument.Find("a.nav__avatar-outer-wrap").First().Attr("href")
	b.token, _ = b.currentDocument.Find("input[name='xsrf_token']").First().Attr("value")
	b.points, _ = strconv.Atoi(b.currentDocument.Find("span.nav__points").First().Text())

	if b.userName == "" || b.token == "" {
		return &BotError{time.Now(), "no user information"}
	}

	stdlog.Printf("receive info [user:%s][pts:%d][token:%s]\n", b.userName, b.points, b.token)

	return nil
}

func (b *TheBot) getGiveawayStatus(path string) (status bool, err error) {
	_, doc, err := b.getPageCustom(b.baseUrl.String() + path)
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

		// stdlog.Println(sgCode)

		x, ok := s.Find("a.giveaway__icon[rel='nofollow']").First().Attr("href")
		if !ok {
			//stdlog.Println("no link?", sgCode)
			return
		}

		// TODO there is app/NUM and sub/NUM need to work with it somehow
		if strings.Contains(x, "/sub/") {
			// stdlog.Println("skip sub giveaway", x)
			return
		}

		// get steam game id and check it whitelisted
		gid, _ := strconv.ParseUint(strings.Trim(x[strings.LastIndex(strings.Trim(x, "/"), "/")+1:len(x)], "/"), 10, 64)
		game, ok := b.gamesWhitelist[gid]
		if !ok {
			// stdlog.Println("skip giveaway by whitelist", gid)
			return
		}

		// get steamgifts giveaway code (unique url)
		sgUrl, ok := s.Find("a.giveaway__heading__name").First().Attr("href")
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
		giveaways = append(giveaways, GiveAway{sgCode, gid, sgUrl, game, time.Unix(t, 0)})
	})

	// stdlog.Println(giveaways)
	return giveaways
}

func (b *TheBot) processGiveaways(giveaways []GiveAway, period time.Duration) (count int) {
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

		status, err := b.getGiveawayStatus(g.Url)
		if err != nil {
			stdlog.Println(err)
			if !status {
				// not enough points
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
		//stdlog.Printf(`enter for giveaway %d:"%s". start in %s`, g.GID, g.Name, g.Time.Sub(time.Now()).String())
		b.addDigest(fmt.Sprintf("%s. Apply for %d : %s. Draw at %s. Reference %s", time.Now().Format("2006-01-02 15:04:05"), g.GID, g.Name, g.Time.Format("2006-01-02 15:04:05"), g.Url))
		b.points, _ = strconv.Atoi(strpts)
	}

	return count
}

func (b *TheBot) parseGiveaways() (count int, err error) {
	err = b.getPage("/")
	if err != nil {
		return
	}

	stdlog.Println("check main page")
	giveaways := b.getGiveaways(b.currentDocument)
	count = b.processGiveaways(giveaways, time.Hour)

	if count == 0 && b.points > 0 {
		// enter for wishlist giveaways if any points left
		stdlog.Println("check wishlist")
		_, doc, err := b.getPageCustom(b.baseUrl.String() + sgWishlistUrl)
		if err != nil {
			return 0, err
		}
		giveaways = b.getGiveaways(doc)
		b.processGiveaways(giveaways, time.Hour * 24 * 7 * 5) // 5 weeks - all
	}

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

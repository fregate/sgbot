package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
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

var requestHeaders = http.Header{
	"Accept": []string{"application/json, text/javascript, */*; q=0.01"},
	"Content-Type": []string{"application/x-www-form-urlencoded; charset=UTF-8"},
	"User-Agent": []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/59.0.3071.115 Safari/537.36"},
	"X-Requested-With": []string{"XMLHttpRequest"},
}

const (
	baseURL             string = "https://www.steamgifts.com"
	sgWishlistURL       string = "/giveaways/search?type=wishlist"
	sgAccountInfo       string = "/giveaways/won"
	baseSteamProfileURL string = "https://steamcommunity.com/profiles/"
	steamFollowed       string = "/followedgames/"
)

// TheBot class for work with SteamGifts pages
type TheBot struct {
	steamID string
	steamAPIKey string

	gamesWhitelist map[uint64]bool
	gamesWon       []uint64

	cookies []*http.Cookie

	enteredGiveAways []string
}

// InitBot initilize bot fields, load configs
func (b *TheBot) InitBot(steamProfile string, apiKey string) error {
	b.steamID = steamProfile
	b.steamAPIKey = apiKey
	b.gamesWhitelist = make(map[uint64]bool)
	b.enteredGiveAways = make([]string, 0)
	return nil
}

type GameInfo struct {
	AppID     uint64 `json:"appid"`
	Priority  int `json:"priority"`
	DateAdded int `json:"added"`
}

type WishlistItems struct {
	Items []GameInfo	`json:"items"`
}

type WishlistGames struct {
	Resp WishlistItems	`json:"response"`
}

func fetchWishlist(steamID string, apiKey string) ([]GameInfo, error) {
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

	games := WishlistGames{}
	err = json.Unmarshal(answer, &games)
	if err != nil {
		return nil, fmt.Errorf("failed unmarshall json response: %v", err)
	}

	return games.Resp.Items, nil
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
	for _, game := range wg {
		b.gamesWhitelist[game.AppID] = true
	}

	// parse followed games entries
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	url, err := url.Parse(baseSteamProfileURL + b.steamID + steamFollowed)
	if err != nil {
		return
	}

	err = navigate(&ctx, url, b.cookies)
	if err != nil {
		stdlog.Println("can't fetch followed games", err)
		return &BotError{time.Now(), "can't fetch followed games"}
	}

	var followed []*cdp.Node
	err = chromedp.Run(ctx, chromedp.Nodes("div[data-appid]", &followed, chromedp.NodeVisible, chromedp.ByQueryAll))
	if err != nil {
		return
	}
	stdlog.Println("followed games entries", len(followed))

	for i := 0; i < len(followed); i++ {
		id, _ := strconv.ParseUint(followed[i].AttributeValue("data-appid"), 10, 64)
		b.gamesWhitelist[id] = true
	}

	stdlog.Println("steam profile parsed successfully")
	return nil
}

func navigate(ctx *context.Context, url *url.URL, cookies []*http.Cookie) error {
	all := chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			for i := 0; i <  len(cookies); i++ {
				if cookies[i].Domain != url.Host {
					continue
				}
				err := network.SetCookie(cookies[i].Name, cookies[i].Value).
					WithDomain(cookies[i].Domain).
					WithHTTPOnly(cookies[i].HttpOnly).
					Do(ctx)
				if err != nil {
					return err
				}
			}
			return nil
		}),
		chromedp.Navigate(url.String()),
	}

	return chromedp.Run(*ctx, all)
}

// func (b *TheBot) getPageCustom(uri string) (retPath string, retDoc *goquery.Document, err error) {
// 	pageURL, err := url.Parse(uri)
// 	if err != nil {
// 		return
// 	}

// 	req, err := http.NewRequest("GET", pageURL.String(), nil)
// 	if err != nil {
// 		return
// 	}

// 	for _, h := range requestHeaders {
// 		req.Header.Add(h.name, h.value)
// 	}

// 	for _, k := range b.cookies {
// 		if k.Domain != pageURL.Host {
// 			continue
// 		}

// 		req.AddCookie(k)
// 	}

// 	stdlog.Printf("getPageCustom. Cookies for %s : %d", pageURL.Host, len(b.cookies))
// 	resp, err := b.client.Do(req)
// 	if err != nil {
// 		return
// 	}

// 	defer resp.Body.Close()

// 	retPath = pageURL.String()
// 	retDoc, err = goquery.NewDocumentFromReader(resp.Body)

// 	return
// }

// func (b *TheBot) getPage(path string) (err error) {
// 	if b.currentURL == baseURL + path {
// 		return nil
// 	}

// 	b.currentURL, b.currentDocument, err = b.getPageCustom(baseURL + path)
// 	return err
// }

// func (b *TheBot) parseToken(str string) string {
// 	return str[len(str)-32:]
// }

func (b *TheBot) setCookies(cookies []*http.Cookie) {
	b.cookies = cookies
}

func (b *TheBot) getUserInfo() (err error) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	url, err := url.Parse(baseURL)
	if err != nil {
		return
	}

	err = navigate(&ctx, url, b.cookies)
	if err != nil {
		return
	}

	// var points string
	// err = chromedp.Run(ctx, chromedp.Text("span.nav__points", &points, chromedp.ByQuery, chromedp.NodeVisible))
	// if err != nil {
	// 	return		
	// }

	var userName string
	var ok bool
	err = chromedp.Run(ctx, chromedp.AttributeValue("a.nav__avatar-outer-wrap", "href", &userName, &ok, chromedp.ByQuery, chromedp.NodeVisible))
	if err != nil {
		return
	}
	
	if !ok || len(userName) == 0 {
		return &BotError{time.Now(), "no user information. please refresh cookies or parser"}
	}

	// stdlog.Printf("receive info [user:%s][pts:%d]\n", userName, points)

	// b.gamesWon = make([]uint64, 0)
	// if b.currentDocument.Find("div.nav__notification").First() != nil { // won something
	// 	b.currentDocument.Find("div.table__row-inner-wrap").Each(func(_ int, s *goquery.Selection) {
	// 		if s.Find("div[class='table__gift-feedback-received is-hidden']").Size() != 0 &&
	// 								s.Find("div.table__gift-feedback-not-received").Size() == 0 {
	// 			// steam id
	// 			steamid, _ := s.Find("a.table_image_thumbnail").First().Attr("style")
	// 			// background-image:url(https://steamcdn-a.akamaihd.net/steam/apps/265930/capsule_184x69.jpg); - [5]
	// 			n, _ := strconv.ParseUint(strings.Split(steamid, "/")[5], 10, 64)

	// 			b.gamesWon = append(b.gamesWon, n)
	// 		}
	// 	})
	// }

	// if len(b.gamesWon) > 0 {
	// 	stdlog.Println("you've won", b.gamesWon)
	// }

	return nil
}

// func (b *TheBot) checkWonList(gid uint64) bool {
// 	if len(b.gamesWon) == 0 {
// 		return false
// 	}

// 	for _, v := range b.gamesWon {
// 		if v == gid {
// 			return true
// 		}
// 	}

// 	return false
// }

func (b *TheBot) getGiveawayStatus(path string) (status bool, err error) {
	url, err := url.Parse(baseURL + path)
	if err != nil {
		return
	}

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	err = navigate(&ctx, url, b.cookies)
	if err != nil {
		return
	}

	var sel []*cdp.Node
	err = chromedp.Run(ctx, chromedp.Nodes("div.widget-container", &sel, chromedp.NodeVisible, chromedp.ByQueryAll))
	// sel := doc.Find("div.widget-container")
	if len(sel) == 0 {
		return true, &BotError{time.Now(), "strange page " + path}
	}

	err = chromedp.Run(ctx, chromedp.Nodes("div.sidebar__error", &sel, chromedp.NodeVisible, chromedp.ByQueryAll))
	// sel = doc.Find("div.sidebar__error")
	if len(sel) != 0 {
		return false, &BotError{time.Now(), "not enough points"}
	}

	err = chromedp.Run(ctx, chromedp.Nodes("div.sidebar__entry-insert", &sel, chromedp.NodeVisible, chromedp.ByQueryAll))
	// sel = doc.Find("div.sidebar__entry-insert")
	// no buttons - exists or not enough points
	if len(sel) != 0 {
		return false, nil
	}

	// skip already entered
	result := true
	for i := 0; i < len(sel) && result; i++ {
		class := sel[i].AttributeValue("class")
		result = !strings.Contains(class, "is-hidden")
	}
	return result, nil
}

func (b *TheBot) enterGiveaway(game GiveAway) (err error) {
	url, err := url.Parse(baseURL + game.URL)
	if err != nil {
		return
	}

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	err = navigate(&ctx, url, b.cookies)
	if err != nil {
		return
	}

	return chromedp.Run(ctx, 
		chromedp.WaitVisible(`body > footer`),
		chromedp.Click(`sidebar__entry-insert`, chromedp.NodeVisible),
	)
}

func (b *TheBot) getGiveaways(ctx *context.Context) (giveaways []GiveAway) {
	re := regexp.MustCompile(`[0-9]+`)

	var nodes []*cdp.Node
	chromedp.Run(*ctx, chromedp.Nodes("div.giveaway__row-outer-wrap", &nodes, chromedp.NodeVisible, chromedp.ByQueryAll))
	for i := 0; i < len(nodes); i++ {
		var c []*cdp.Node
		err := chromedp.Run(*ctx, chromedp.Nodes("a.giveaway__heading__name", &c, chromedp.ByQuery, chromedp.FromNode(nodes[i])))
		if err != nil || len(c) == 0 {
			continue
		}
		sgURL := c[0].AttributeValue("href")
		sgCode := strings.Split(sgURL, "/")[2]
		game := c[0].NodeValue

		err = chromedp.Run(*ctx, chromedp.Nodes("span[data-timestamp]", &c, chromedp.ByQuery, chromedp.FromNode(nodes[i])))
		if err != nil || len(c) == 0 {
			continue
		}

		// get giveaway timestamp
		t, _ := strconv.ParseInt(c[0].AttributeValue("data-timestamp"), 10, 64)

		err = chromedp.Run(*ctx, chromedp.Nodes("a.giveaway__icon[target='_blank']", &c, chromedp.ByQuery, chromedp.FromNode(nodes[i])))
		if err != nil || len(c) == 0 {
			continue
		}
		
		steamUrl := c[0].AttributeValue("href")
		if strings.Contains(steamUrl, "/sub/") { // parse sub page
			stdlog.Println("parse 'sub' giveaway", steamUrl)
			steamCtx, cancel := chromedp.NewContext(context.Background())
			defer cancel()

			url, err := url.Parse(baseURL + sgWishlistURL)
			if err != nil {
				return
			}

			err = navigate(&steamCtx, url, b.cookies)
			if err != nil {
				errlog.Println("can't get page", steamUrl)
				return
			}

			chromedp.Run(steamCtx, chromedp.Nodes("div.tab_item", &c, chromedp.NodeVisible, chromedp.ByQueryAll))
			for k := 0; k < len(c); k++ {
				gid, _ := strconv.ParseUint(c[k].AttributeValue("data-ds-appid"), 10, 64)
				_, ok := b.gamesWhitelist[gid]
				if !ok {
					// stdlog.Println("skip giveaway by whitelist", gid)
					continue;
				}

				// if b.checkWonList(gid) {
				// 	// stdlog.Println("skip - already won! receve your gift!")
				// 	continue
				// }

				giveaways = append(giveaways, GiveAway{sgCode, gid, sgURL, game, time.Unix(t, 0)})
			}
		} else {
			// parse single game GA
			// get steam game id and check it whitelisted
			strgid := re.FindAllString(steamUrl, -1)
			if len(strgid) == 0 {
				stdlog.Println("skip giveaway - can't find steam id", steamUrl)
				return
			}
			gid, _ := strconv.ParseUint(strgid[0], 10, 64)
			// stdlog.Println(gid)
			_, ok := b.gamesWhitelist[gid]
			if !ok {
				// stdlog.Println("skip giveaway by whitelist", gid)
				return
			}

			// if b.checkWonList(gid) {
			// 	// stdlog.Println("skip - already won! receve your gift!")
			// 	return
			// }

			// add nanoseconds to split giveaways which will be ended at one time
			giveaways = append(giveaways, GiveAway{sgCode, gid, sgURL, game, time.Unix(t, 0)})
		}
	}

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

		// add some human behaviour - pause bot for a few seconds (1-3)
		d := time.Second * time.Duration(rand.Intn(3) + 1)
		if game.Time.After(time.Now().Add(d)) {
			time.Sleep(d)
		}

		err = b.enterGiveaway(game)
		if err != nil {
			stdlog.Printf("internal error (%s) when enter for [%+v]", err, game)
			continue
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
		return 0, err
	}

	if len(b.gamesWhitelist) == 0 {
		stdlog.Println("there is no game you want to win, please add some in json list or steam account. bye")
		return 0, errors.New("math: square root of negative number")
	}

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	stdlog.Println("process wishlist")
	url, err := url.Parse(baseURL + sgWishlistURL)
	if err != nil {
		return 0, err
	}
	err = navigate(&ctx, url, b.cookies)
	if err != nil {
		return 0, err
	}

	giveaways := b.getGiveaways(&ctx)
	stdlog.Println("found giveaways on page:", len(giveaways))
	count, entriesWishlist := b.processGiveaways(giveaways, time.Hour * 24 * 7 * 5) // 5 weeks - all
	stdlog.Println("processed giveaways in wishlist", entriesWishlist)

	stdlog.Println("process main page")
	url, err = url.Parse(baseURL + sgWishlistURL)
	if err != nil {
		return 0, err
	}
	err = navigate(&ctx, url, b.cookies)
	if err != nil {
		return 0, err
	}
	giveaways = b.getGiveaways(&ctx)
	stdlog.Println("found giveaways on page:", len(giveaways))
	count, entriesMainPage := b.processGiveaways(giveaways, time.Hour)

	defer stdlog.Println("processed giveaways", entriesWishlist + entriesMainPage)

	return entriesWishlist + entriesMainPage, nil
}

func (b *TheBot) addDigest(msg string) {
	b.enteredGiveAways = append(b.enteredGiveAways, msg)
}

func init() {
	stdlog = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	errlog = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds)
}

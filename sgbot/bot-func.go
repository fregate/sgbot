package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	yc "github.com/yandex-cloud/go-sdk"
	ycsdk "github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result/named"
)

type Response struct {
	StatusCode int `json:"statusCode"`
}

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

	defer fmt.Println("bot check finished")

	_, err = b.parseGiveaways(games)
	return b.enteredGiveAways, err
}

func RunBot(botRequest *Request) (digest []string, err error) {
	bot := &TheBot{}
	err = bot.InitBot(botRequest.SteamProfile)
	if err != nil {
		fmt.Println("error during bot initialization.", err)
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

// Requirements for execution:
// Set STEAM_PROFILE environment variable as your steam profile id (https://steamcommunity.com/id/<profile>/)
// YDB connection:
// Set YDB_DATABASE : a name for YDB (shown in yandex cloud console)
func RunSGBOTFunc(ctx context.Context) (*Response, error) {
	dbName := os.Getenv("YDB_DATABASE")
	if len(dbName) == 0 {
		return nil, fmt.Errorf("no ydb database name")
	}
	// Determine timeout for connect or do nothing
	connectCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	creds := yc.InstanceServiceAccount()
	token, err := creds.IAMToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get iam token. %v", err)
	}

	db, err := ycsdk.Open(
		connectCtx,
		fmt.Sprintf("grpcs://ydb.serverless.yandexcloud.net:2135/?database=%s", dbName),
		ycsdk.WithAccessTokenCredentials(token.IamToken),
	)
	if err != nil {
		return nil, fmt.Errorf("ydb connect error: %w", err)
	}
	defer db.Close(connectCtx)

	// get games, cookies from db
	// make request suited for checking
	r := &Request{}
	r.SteamProfile = os.Getenv("STEAM_PROFILE")

	session, err := db.Table().CreateSession(connectCtx)
	if err == nil {
		txc := table.TxControl(
			table.BeginTx(table.WithSerializableReadWrite()),
			table.CommitTx(),
		)

		// read cookies
		_, res, err := session.Execute(connectCtx, txc,
			`--!syntax_v1
			SELECT name, value, domain, path FROM cookies
		`,
			nil,
		)
		defer res.Close()
		if err == nil {
			for res.NextResultSet(connectCtx) {
				for res.NextRow() {
					var c Cookie
					err := res.ScanNamed(
						named.OptionalWithDefault("name", &c.Name),
						named.OptionalWithDefault("value", &c.Value),
						named.OptionalWithDefault("domain", &c.Domain),
						named.OptionalWithDefault("path", &c.Path))
					if err != nil {
						fmt.Printf("error parsing cookie row. %v", err)
						continue
					}
					fmt.Printf("adding cookie. %v", c)
					r.Cookies = append(r.Cookies, c)
				}
			}
		} else {
			fmt.Printf("can't fetch row. %v", err)
		}
	} else {
		fmt.Printf("error creating session. %v", err)
	}
	defer session.Close(connectCtx)

	fmt.Printf("request. profile: %s, cookies: %d, games: %d", r.SteamProfile, len(r.Cookies), len(r.Games))
	_, err = RunBot(r)
	if err != nil {
		return nil, fmt.Errorf("bot error: %v", err)
	}

	// write digest to database (digest will send by another function by trigger)

	return &Response{
		StatusCode: http.StatusOK,
	}, nil
}

package gogbot

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
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
)

type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

func populateCookies(b *TheBot, botCookies []Cookie) {
	cookies := make([]*http.Cookie, 0)
	for _, c := range botCookies {
		cookies = append(cookies, &http.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path})
	}
	b.setCookies(cookies)
}

// Check - check page and enter for gifts (repeat by timeout)
func runCheck(b *TheBot) (digest []string, err error) {
	defer fmt.Println("gogbot check finished")

	_, err = b.parseGiveaways(games)
	return b.enteredGiveAways, err
}

func RunBot(cookies []Cookie) (digest []string, err error) {
	bot := &TheBot{}
	err = bot.initBot()
	if err != nil {
		fmt.Println("error during bot initialization.", err)
		return
	}

	populateCookies(bot, cookies)

	digest, err = runCheck(bot)
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

	creds := yc.InstanceServiceAccount()
	token, err := creds.IAMToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get iam token. %v", err)
	}

	// Determine timeout for connect or do nothing
	connectCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	db, err := ycsdk.Open(
		connectCtx,
		fmt.Sprintf("grpcs://ydb.serverless.yandexcloud.net:2135/?database=%s", dbName),
		ycsdk.WithAccessTokenCredentials(token.IamToken),
	)
	if err != nil {
		return nil, fmt.Errorf("ydb connect error: %w", err)
	}
	defer func() { _ = db.Close(connectCtx) }()

	var cookies = make([]Cookie, 0)
	// get cookies from db
	// make request suited for checking
	err = db.Table().Do(connectCtx, func(ctxSession context.Context, session table.Session) (err error) {
		txc := table.TxControl(
			table.BeginTx(table.WithOnlineReadOnly()),
			table.CommitTx(),
		)

		// read cookies
		_, res, err := session.Execute(ctxSession, txc,
			`--!syntax_v1
			SELECT name, value, domain, path FROM cookies
			`,
			nil,
		)
		if err == nil {
			for res.NextResultSet(ctxSession) {
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
					cookies = append(cookies, c)
				}
			}
			fmt.Println(len(cookies), "cookies added")
			res.Close()
		} else {
			fmt.Printf("can't select from 'cookie' table. %v", err)
			return
		}

		return
	})

	digest, err := RunBot(cookies)
	if err != nil {
		return nil, fmt.Errorf("bot error: %v", err)
	}

	if len(digest) > 0 {
		fmt.Println("update digest")
		err = db.Table().Do(connectCtx, func(ctxSession context.Context, session table.Session) (err error) {
			msgs := make([]types.Value, 0, len(digest))
			for _, m := range digest {
				msgs = append(msgs, types.StructValue(
					types.StructFieldValue("msg", types.UTF8Value(m)),
				))
			}

			txc := table.TxControl(
				table.BeginTx(table.WithSerializableReadWrite()),
				table.CommitTx(),
			)

			_, _, err = session.Execute(ctxSession, txc,
				`--!syntax_v1
				DECLARE $messages AS List<Struct<
					msg: Utf8>>;

				REPLACE INTO digest
				SELECT msg AS message FROM AS_TABLE($messages);
				`,
				table.NewQueryParameters(table.ValueParam("$messages", types.ListValue(msgs...))),
			)
			if err != nil {
				fmt.Println("can't insert into 'digest'", err)
				return
			}
			return
		})
		if err != nil {
			fmt.Println("error process 'digest'", err)
		}
	}

	return &Response{
		StatusCode: http.StatusOK,
	}, nil
}

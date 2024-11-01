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
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
)

func RunBot(cookies []*http.Cookie) (digest []string, err error) {
	bot := &TheBot{}
	err = bot.initBot()
	if err != nil {
		fmt.Println("error during bot initialization.", err)
		return
	}

	bot.setCookies(cookies)

	digest, err = bot.claimGiveaway()
	if err != nil {
		fmt.Println("error during check.", err)
	} else {
		fmt.Println("gogbot check finished")
	}

	return
}

// Requirements for execution:
// Set YDB_DATABASE : a name for YDB (shown in yandex cloud console)
func RunGOGBOTFunc(ctx context.Context) (*Response, error) {
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

	var cookies = make([]*http.Cookie, 0)
	err = db.Table().Do(connectCtx, func(ctxSession context.Context, session table.Session) (err error) {
		txc := table.TxControl(
			table.BeginTx(table.WithOnlineReadOnly()),
			table.CommitTx(),
		)

		// read cookies
		_, res, err := session.Execute(ctxSession, txc,
			`--!syntax_v1
			SELECT name, value, domain, path FROM cookies WHERE domain LIKE "%gog%"
			`,
			nil,
		)
		if err == nil {
			for res.NextResultSet(ctxSession) {
				for res.NextRow() {
					var c http.Cookie
					err := res.ScanNamed(
						named.OptionalWithDefault("name", &c.Name),
						named.OptionalWithDefault("value", &c.Value),
						named.OptionalWithDefault("domain", &c.Domain),
						named.OptionalWithDefault("path", &c.Path))
					if err != nil {
						fmt.Printf("error parsing cookie row. %v", err)
						continue
					}
					cookies = append(cookies, &c)
				}
			}
			fmt.Println(len(cookies), "cookies added")
			res.Close()
		} else {
			fmt.Printf("Can't select from 'cookie' table. %v", err)
			return
		}

		return
	})
	if err != nil {
		fmt.Println("Can't read from db", err)
	}

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

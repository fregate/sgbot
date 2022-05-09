package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	yc "github.com/yandex-cloud/go-sdk"
	ycsdk "github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/options"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
)

// Requirements for execution:
// Set YDB_DATABASE environment variable : a name for YDB (shown in yandex cloud console)
func RunInitBotDB(ctx context.Context) (*Response, error) {
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

	err = db.Table().Do(connectCtx, func(ctxSession context.Context, session table.Session) (err error) {
		// create games table
		err = session.CreateTable(ctxSession, path.Join(db.Name(), "games"),
			options.WithColumn("id", types.Optional(types.TypeUint64)),
			options.WithColumn("name", types.Optional(types.TypeString)),
			options.WithPrimaryKeyColumn("id"),
		)
		if err != nil {
			return
		}

		// create cookies table
		err = session.CreateTable(ctxSession, path.Join(db.Name(), "cookies"),
			options.WithColumn("name", types.Optional(types.TypeString)),
			options.WithColumn("value", types.Optional(types.TypeString)),
			options.WithColumn("domain", types.Optional(types.TypeString)),
			options.WithColumn("path", types.Optional(types.TypeString)),
			options.WithPrimaryKeyColumn("name"),
		)
		if err != nil {
			return
		}

		// create digest table
		err = session.CreateTable(ctxSession, path.Join(db.Name(), "digest"),
			options.WithColumn("message", types.Optional(types.TypeUTF8)),
			options.WithPrimaryKeyColumn("message"),
		)

		return
	})
	if err != nil {
		return nil, fmt.Errorf("can't prepare db. %v", err)
	}

	return &Response{
		StatusCode: http.StatusOK,
	}, nil
}

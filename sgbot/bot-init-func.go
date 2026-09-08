package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	yc "github.com/yandex-cloud/go-sdk"
	ycsdk "github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/query"
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
	defer db.Close(connectCtx)

	err = db.Query().Exec(connectCtx,
		`CREATE TABLE IF NOT EXISTS games (
			id Uint64,
			name Utf8,
			PRIMARY KEY(id)
		)`,
		query.WithTxControl(query.NoTx()),
	)
	if err != nil {
		return nil, fmt.Errorf("can't prepare db. %v", err)
	}

	err = db.Query().Exec(connectCtx,
		`CREATE TABLE IF NOT EXISTS cookies (
			name Utf8,
			value Utf8,
			domain Utf8,
			path Utf8,
			PRIMARY KEY(name, domain)
		)`,
		query.WithTxControl(query.NoTx()),
	)
	if err != nil {
		return nil, fmt.Errorf("can't prepare db. %v", err)
	}

	err = db.Query().Exec(connectCtx,
		`CREATE TABLE IF NOT EXISTS digest (
			message Utf8,
			PRIMARY KEY(message)
		)`,
		query.WithTxControl(query.NoTx()),
	)
	if err != nil {
		return nil, fmt.Errorf("can't prepare db. %v", err)
	}

	err = db.Query().Exec(connectCtx,
		`CREATE TABLE IF NOT EXISTS keys (
			id Uint64,
			type Utf8,
			value Utf8,
			PRIMARY KEY(id)
		)`,
		query.WithTxControl(query.NoTx()),
	)
	if err != nil {
		return nil, fmt.Errorf("can't prepare db. %v", err)
	}

	return &Response{
		StatusCode: http.StatusOK,
	}, nil
}

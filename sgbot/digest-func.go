package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	gomail "gopkg.in/gomail.v2"

	yc "github.com/yandex-cloud/go-sdk"
	ycsdk "github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result/named"
)

func makeMailer() (*gomail.Dialer, error) {
	smtp := os.Getenv("MAILER_SMTP")
	port, err := strconv.Atoi(os.Getenv("MAILER_PORT"))
	if err != nil {
		fmt.Println("error parsing smtp port.", err)
		return nil, err
	}
	name := os.Getenv("MAILER_AUTH_NAME")
	pwd := os.Getenv("MAILER_AUTH_PWD")
	if port == 0 || smtp == "" && name == "" {
		return nil, errors.New("invalid mailer parameters")
	}

	return gomail.NewDialer(smtp, port, name, pwd), nil
}

// Requirements for execution:
// Set MAILER_SMTP environment variable - smtp server to send from
// Set MAILER_PORT environment variable - smtp server port
// Set MAILER_AUTH_NAME environment variable - smtp server username (and 'from' field)
// Set MAILER_AUTH_PWD environment variable - smtp server username password
// Set MAILER_SUBJECT environment variable - digest subject
// Set MAILER_RECIPIENT environment variable - digest recipient
// YDB connection:
// Set YDB_DATABASE : a name for YDB (shown in yandex cloud console)
func SendDigest(ctx context.Context) (*Response, error) {
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
	connectCtx, cancel := context.WithTimeout(ctx, time.Second)
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

	// fetch digest messages
	rows := make([]string, 0)
	err = db.Table().Do(connectCtx, func(ctxSession context.Context, session table.Session) (err error) {
		txc := table.TxControl(
			table.BeginTx(table.WithSerializableReadWrite()),
			table.CommitTx(),
		)

		// read messages
		_, res, err := session.Execute(ctxSession, txc,
			`--!syntax_v1
			SELECT message FROM digest
			`,
			nil,
		)
		if err == nil {
			for res.NextResultSet(ctxSession) {
				for res.NextRow() {
					var msg string
					err := res.ScanNamed(
						named.OptionalWithDefault("message", &msg))
					if err != nil {
						fmt.Printf("error parsing digest row. %v", err)
						continue
					}
					rows = append(rows, msg)
				}
			}
			res.Close()
		} else {
			fmt.Printf("can't select from 'digest' table. %v", err)
			return
		}

		// delete digest entries (do not run digest and bot check in one time - kind of race)
		_, _, err = session.Execute(ctxSession, txc,
			`--!syntax_v1
			DELETE FROM digest
			`,
			nil,
		)
		if err != nil {
			fmt.Printf("can't delete entries in 'digest'. %v", err)
			return
		}

		return
	})
	if err != nil {
		fmt.Println("can't read from db", err)
	}

	mailer, err := makeMailer()
	if err != nil {
		return nil, fmt.Errorf("can't create mailer. %v", err)
	}

	recipient := os.Getenv("MAILER_RECIPIENT")
	if recipient == "" {
		return nil, fmt.Errorf("empty recipient")
	}

	m := gomail.NewMessage()
	m.SetHeader("From", os.Getenv("MAILER_AUTH_NAME"))
	m.SetHeader("To", os.Getenv("MAILER_RECIPIENT"))
	m.SetHeader("Subject", os.Getenv("MAILER_SUBJECT"))
	m.SetBody("text/plain", strings.Join(rows, "\n"))

	err = mailer.DialAndSend(m)
	if err != nil {
		return nil, fmt.Errorf("can't send digest. %v", err)
	}

	return &Response{
		StatusCode: http.StatusOK,
	}, nil
}

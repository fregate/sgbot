package main

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

type pair struct {
	name  string
	value string
}

var requestHeaders = []pair{
	{name: "Accept", value: "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
	{name: "User-Agent", value: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36"},
}

const (
	baseURL             string = "https://www.gog.com"
	claim               string = "/giveaway/claim"
)

type TheBot struct {
	// http client
	client          *http.Client
	cookies         []*http.Cookie
}

func (b *TheBot) initBot() error {
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return err
	}

	b.client = &http.Client{Jar: jar}
	return nil
}

func (b *TheBot) setCookies(cookies []*http.Cookie) {
	b.cookies = cookies
}

func (b *TheBot) getPageCustom(uri string) (retCode int, err error) {
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
		req.AddCookie(k)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return
	}

	retCode = resp.StatusCode

	return
}

func (b *TheBot) claimGiveaway() (digest []string, err error) {
	code, err := b.getPageCustom(baseURL + claim)

	if err != nil {
		fmt.Println("GOGBOT: returned error:", err)
		return
	}

	fmt.Println("GOGBOT: returned code:", code)

	response := make([]string, 0)

	switch code {
	case http.StatusOK:
	case http.StatusCreated:
		response = append(response, fmt.Sprintf("%s. GOGBOT: claimed something", time.Now().Format("15:04:05")))

	case http.StatusUnauthorized:
		response = append(response, fmt.Sprintf("%s. GOGBOT: unautorized", time.Now().Format("15:04:05")))
	}

	return response, err
}

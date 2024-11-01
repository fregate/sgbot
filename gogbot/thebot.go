package gogbot

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

type pair struct {
	name  string
	value string
}

var requestHeaders = []pair{
	{name: "Accept", value: "application/json, text/javascript, */*; q=0.01"},
	{name: "Content-Type", value: "application/x-www-form-urlencoded; charset=UTF-8"},
	{name: "User-Agent", value: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/59.0.3071.115 Safari/537.36"},
	{name: "X-Requested-With", value: "XMLHttpRequest"},
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

func (b *TheBot) getPageCustom(uri string) (retDoc string, err error) {
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
		if k.Domain != pageURL.Host {
			continue
		}

		req.AddCookie(k)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	retDoc = string(body)

	return
}

func (b *TheBot) getPage(path string) (message string, err error) {
	response, err := b.getPageCustom(baseURL + path)
	return []string(response), err
}

func (b *TheBot) claimGiveaway() (digest []string, err error) {

	return
}
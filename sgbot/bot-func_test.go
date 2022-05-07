package main

import (
	"testing"
)

func TestBotFunc(t *testing.T) {
	req := &Request{}
	req.SteamProfile = "fregate"
	req.Cookies = make([]Cookie, 0)
	req.Cookies = append(req.Cookies, Cookie{Name: "PHPSESSID", Value: "o3q1ob2bl581foi7d08d0g1dtvm2d5m285vushdishe5pbgc", Domain: "www.steamgifts.com", Path: "/"})

	digest, err := RunBot(req)
	if err != nil {
		t.Errorf("error during check")
	}

	if len(digest) == 0 {
		t.Errorf("no entries")
	}
}

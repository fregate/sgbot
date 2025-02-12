package main

import (
	"testing"
)

func TestBotFunc(t *testing.T) {
	req := &Request{}
	req.SteamProfile = "" // steam profile ID (64 bit number)
	req.SteamAPIKey = "" // steam API Key
	req.Cookies = make([]Cookie, 0)
	req.Cookies = append(req.Cookies,
		Cookie{Name: "PHPSESSID", Value: "<copy it from browser after sg login>", Domain: "www.steamgifts.com", Path: "/"},
	)

	digest, err := RunBot(req)
	if err != nil {
		t.Errorf("error during check: %v", err)
	}

	if len(digest) == 0 {
		t.Errorf("no entries")
	}
}

package main

import (
	"testing"
)

func TestBotFunc(t *testing.T) {
	req := &Request{}
	req.SteamProfile = "" // steam profile ID (64 bit number)
	req.SteamAPIKey = "" // steam API Key
	req.ZenrowAPIKeys = []string{  // zenrows API keys
	}
	req.Cookies = make([]Cookie, 0)
	req.Cookies = append(req.Cookies,
		Cookie{Name: "PHPSESSID", Value: "", Domain: "www.steamgifts.com", Path: "/"},
	)

	bot := &TheBot{}
	digest, err := RunBot(bot, req)
	if err != nil {
		t.Errorf("error during check: %v", err)
	}

	if len(digest) == 0 {
		t.Errorf("no entries")
	}
}

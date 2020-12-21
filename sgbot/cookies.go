package main

import (
	"net/http"
	"strings"
)

// ReadCookies reads config cookies from file
func ReadCookies(fileName string) (cookies []*http.Cookie, err error) {
	var ccc interface{}
	err = ReadConfig(fileName, &ccc)
	if err != nil {
		stdlog.Println("Can't open cookie file", fileName)
		return
	}

	m := ccc.(map[string]interface{})
	for k, v := range m {
		ck := strings.Split(v.(string), ":")
		if len(ck) >= 3 {
			cookies = append(cookies, &http.Cookie{Name: k, Value: ck[0], Domain: ck[1], Path: ck[2]})
		} else {
			stdlog.Println("wrong cookie (< 3 params)", k, v.(string))
		}
	}

	return
}

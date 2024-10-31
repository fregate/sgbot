// package config works with settings and returns named pairs

package main

import (
	"encoding/json"
	"os"
)

// ReadConfig read from file (uri) settings
func ReadConfig(uri string, c interface{}) (err error) {
	raw, err := os.ReadFile(uri)

	if err != nil {
		return
	}

	err = json.Unmarshal(raw, c)
	if err != nil {
		return
	}

	return nil
}

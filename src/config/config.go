// package config works with settings and returns named pairs

package config

import (
        "encoding/json"
        "io/ioutil"
)

// ReadConfig read from file (uri) settings
func ReadConfig(uri string, c interface{}) (err error) {
        raw, err := ioutil.ReadFile(uri)

        if err != nil {
                return
        }

        err = json.Unmarshal(raw, c)
        if err != nil {
                return
        }

        return nil
}

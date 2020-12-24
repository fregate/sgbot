package main

type mailinfo struct {
	SMTPServer       string `json:"smtp"`
	Port             int    `json:"port-num"`
	SMTPUsername     string `json:"username"`
	SMTPUserpassword string `json:"password"`
}

// Configuration holds config.json
type Configuration struct {
	SteamProfile string `json:"profile"`
	SendDigest   bool   `json:"digest"`

	SMTPSettings    mailinfo `json:"mail"`
	EmailSubjectTag string   `json:"subjecttag"`
	EmailRecipient  string   `json:"recipient"`
}

// ReadConfiguration reads struct from file
func ReadConfiguration(fileName string) (structCfg Configuration, err error) {
	err = ReadConfig(fileName, &structCfg)
	if err != nil {
		stdlog.Println(err)
		return
	}

	return
}

func (c mailinfo) isValid() bool {
	return c.Port != 0 && c.SMTPServer != "" && c.SMTPUsername != ""
}

func (c Configuration) isMailValid() bool {
	return c.SMTPSettings.isValid() && c.EmailRecipient != ""
}

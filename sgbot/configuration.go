package main

type mailinfo struct {
	SMTPServer       string `json:"smtp"`
	Port             int    `json:"port-num"`
	SMTPUsername     string `json:"username"`
	SMTPUserpassword string `json:"password"`
	EmailSubjectTag  string `json:"subjecttag"`
	EmailRecipient   string `json:"recipient"`
}

// Configuration holds config.json
type Configuration struct {
	SteamProfile string `json:"profile"`
	SendDigest   bool   `json:"digest"`

	MailSettings mailinfo `json:"mail"`
}

// ReadConfiguration reads struct from file
func ReadConfiguration(fileName string) (structCfg Configuration, err error) {
	err = ReadConfig(fileName, &structCfg)
	if err != nil {
		stdlog.Println(err)
	}

	return
}

func (c mailinfo) isValid() bool {
	return c.Port != 0 && c.SMTPServer != "" && c.SMTPUsername != "" && c.EmailRecipient != ""
}

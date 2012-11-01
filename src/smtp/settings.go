package smtp

import (
	"encoding/json"
	"log4g"
	"os"
	"path"
)

type relayCfg map[string]map[string][]string

type Settings struct {
	Bind      string
	Port      int
	MaxCli    int
	DebugMode bool
	Spool     string
	RelayCtrl relayCfg
	Retries   []int
	SendLock  int
	fileName  string
	*log4g.SysLogger
}

func (s Settings) Dump() string {
	s.RelayCtrl = relayCfg{}
	s.Retries = []int{}
	dump, err := json.Marshal(s)
	if err == nil {
		return string(dump)
	}
	return err.Error()
}

func (s Settings) postmaster(sender string) string {
	for domain, ctrl := range s.RelayCtrl {
		_, ok := ctrl[sender]
		if ok {
			return "postmaster@" + domain
		}
	}
	return "postmaster@[127.0.0.1]"
}

func LoadSettings(filename string) (*Settings, error) {
	logger, err := log4g.NewSysLogger(path.Base(os.Args[0]), log4g.DEBUG_MODE)
	if err != nil {
		return nil, err
	}
	settings := Settings{
		"127.0.0.1",       //Bind
		25,                //Port
		1,                 //MaxCli
		false,             //DebugMode
		"/var/spool/mail", //Spool
		relayCfg{
			"example.com": {
				"postmaster":        {"admin@example.com"},
				"admin@example.com": {"postmaster"},
			},
		}, //RelayCtrl
		[]int{
			900, 1800, 3600, 7200,
			14400, 28800, 57600,
		}, //Retries
		3600, //SendLock
		filename,
		logger,
	}
	var f *os.File
	f, err = os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			f, err = os.Create(filename)
			if err == nil {
				defer f.Close()
				s, err := json.MarshalIndent(settings, "", "\t")
				if err == nil {
					f.Write(s)
				}
			}
		}
	} else {
		defer f.Close()
		dec := json.NewDecoder(f)
		err = dec.Decode(&settings)
	}
	if err == nil {
		settings.Mode(settings.DebugMode)
		settings.Spool = path.Clean(settings.Spool)
		err = os.MkdirAll(settings.Spool+"/inbound", 0777)
		if err == nil {
			err = os.MkdirAll(settings.Spool+"/outbound", 0777)
		}
	}
	return &settings, err
}

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
	OpenRelay []string
	RelayCtrl relayCfg
	Gateways  []string
	Retries   []int
	SendLock  int
	fileName  string
	expire    int
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

func (s Settings) originDomain(sender string) string {
	od := "[127.0.0.1]"
	for domain, ctrl := range s.RelayCtrl {
		od = domain
		_, ok := ctrl[sender]
		if ok {
			return domain
		}
	}
	return od
}

func LoadSettings(filename string) (*Settings, error) {
	logger, err := log4g.NewSysLogger(path.Base(os.Args[0]), log4g.DEBUG_MODE)
	if err != nil {
		return nil, err
	}
	s := Settings{
		"127.0.0.1",       //Bind
		25,                //Port
		1,                 //MaxCli
		false,             //DebugMode
		"/var/spool/mail", //Spool
		[]string{},        //OpenRelay
		relayCfg{
			"example.com": {
				"postmaster":        {"admin@example.com"},
				"admin@example.com": {"postmaster"},
			},
		}, //RelayCtrl
		[]string{}, //Gateways
		[]int{
			900, 1800, 3600, 7200,
			14400, 28800, 57600,
		}, //Retries
		3600, //SendLock
		filename,
		0, //expire
		logger,
	}
	var f *os.File
	f, err = os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			f, err = os.Create(filename)
			if err == nil {
				defer f.Close()
				s, err := json.MarshalIndent(s, "", "\t")
				if err == nil {
					f.Write(s)
				}
			}
		}
	} else {
		defer f.Close()
		dec := json.NewDecoder(f)
		err = dec.Decode(&s)
	}
	if err == nil {
		s.Mode(s.DebugMode)
		s.Spool = path.Clean(s.Spool)
		err = os.MkdirAll(s.Spool+"/inbound", 0777)
		if err == nil {
			err = os.MkdirAll(s.Spool+"/outbound", 0777)
		}
	}
	if err == nil {
		if s.MaxCli <= 0 {
			s.MaxCli = 1
		}
		if s.SendLock < 3600 {
			s.SendLock = 3600
		}
		s.expire = 0
		for _, d := range s.Retries {
			s.expire += d
		}
		s.expire *= 2
		if s.expire < 0 {
			s.Retries = []int{900, 1800, 3600, 7200, 14400, 28800, 57600}
			s.expire = 228600
		} else if s.expire < 3600 {
			s.expire = 3600
		}
	}
	return &s, err
}

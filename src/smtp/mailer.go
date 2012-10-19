package smtp

import (
	"log4g"
    "net"
    "path"
	"path/filepath"
)

var (
    resolver chan chan string
)

//type relayRoute map[string]map[string]int64

func init() {
    resolver = make(chan chan string)
}

func send(server string, env *envelope, logger log4g.Logger) bool {
    logger.Log("TODO: send mail...")
    logger.Log("  ^server: " + server)
	logger.Log("  ^domain: " + env.domain)
	logger.Log("  ^file: " + env.file)
    return true
}

func sendMail(file string, ss *Settings) {
    env, err := loadEnvelope(file, 3600)
    if err != nil {
		ss.Log("RUNERR: " + err.Error())
		return        
    }
    if env == nil {
        ss.Debug("On hold: " + path.Base(file))
        return
    }
    done := false
    ch := make(chan string)
    resolver <- ch
    ch <- env.domain
    for {
        ip, ok := <-ch
        if !ok {
            break
        }
        if ip[0] == '@' {
            if !done {
                done = send(ip[1:], env, ss)
            }
        } else {
            env.errors[env.domain] = ip[1:]
        }
    }
    err = env.flush(ss)
    if err != nil {
        ss.Log("RUNERR: " + err.Error())
    }
}

func MXResolver() {
	for {
		ch := <-resolver
		go func(c chan string) {
			domain := <-c
			mx, err := net.LookupMX(domain)
			if err == nil {
				cnt := 0
				for i := 0; i < len(mx); i++ {
					ips, err := net.LookupIP(mx[i].Host)
					if err == nil {
						for _, ip := range ips {
							c <- "@" + ip.String()
							cnt++
						}
					}
				}
				if cnt == 0 {
					c <- "!Cannot get MX record for " + domain
				}
			} else {
				c <- "!" + err.Error()
			}
			close(c)
		}(ch)
	}
}

func SendMails(spool string, ss *Settings) {
	envelopes, err := filepath.Glob(spool + "/*.env")
	if err == nil {
		ss.Debugf("SendMails: queued_messages=%v", len(envelopes))
		for _, e := range envelopes {
			go sendMail(e, ss)
		}
	} else {
		ss.Log("RUNERR: " + err.Error())
	}
}

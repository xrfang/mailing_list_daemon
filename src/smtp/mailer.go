package smtp

import (
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
)

func send(server string, env *envelope, msg *os.File, ss *Settings) bool {
	cs, err := NewCliSession(server, ss)
	defer func() {
		if cs != nil {
			cs.close()
		}
	}()
	if err != nil {
		env.log("", err.Error())
		return false
	}
	err = cs.act("MAIL FROM:<"+env.Sender+">", "2")
	if err != nil {
		env.log("", err.Error())
		return false
	}
	rcnt := 0
	for _, r := range env.Recipients {
		err = cs.act("RCPT TO:<"+r+">", "2")
		if err != nil {
			env.log(r, err.Error())
			continue
		}
		rcnt++
	}
	if rcnt > 0 {
		err = cs.act("DATA", "3")
		if err != nil {
			env.log("", err.Error())
			return false
		}
		buf := make([]byte, 65536)
		for {
			in, err := io.ReadFull(msg, buf)
			if in == 0 {
				break
			}
			_, err = cs.conn.Write(buf[:in])
			if err != nil {
				env.log("", err.Error())
				return false
			}
		}
		err = cs.act("\r\n.", "2")
		if err != nil {
			env.log("", err.Error())
			return false
		}
	}
	err = cs.act("QUIT", "2")
	if err != nil {
		env.log("", err.Error())
		return false
	}
	return true
}

func sendMail(file string, ss *Settings) {
	env, err := loadEnvelope(file, 3600)
	defer func() {
		if env != nil {
			for d, e := range env.errors {
				ss.Logf("  %s:%s", d, e)
			}
			err = env.flush(ss)
			if err != nil {
				ss.Log("RUNERR: " + err.Error())
			}
		}
	}()
	if err != nil {
		ss.Log("RUNERR: " + err.Error())
		return
	}
	if env == nil {
		ss.Debug("On hold: " + path.Base(file))
		return
	}
	mxs, err := net.LookupMX(env.domain)
	if err != nil {
		env.log("", err.Error())
		ss.Debugf("GetMX: %v", err)
		return
	}
	msg, err := os.Open(env.content)
	if err != nil {
		env.log("", err.Error())
		ss.Debugf("OpenMsg: %v", err)
		return
	}
	for _, mx := range mxs {
		if send(mx.Host, env, msg, ss) {
			break
		}
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

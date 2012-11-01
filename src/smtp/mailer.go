package smtp

import (
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func fatal(err error) bool {
	return strings.HasPrefix(err.Error(), "5")
}

func send(server string, env *envelope, msg *os.File) {
	cs, err := NewCliSession(server, env)
	defer func() {
		if cs != nil {
			cs.Close()
		}
		env.flush(false)
	}()
	if err != nil {
		env.recErr("", err.Error(), false)
		return
	}
	err = cs.act("MAIL FROM:<"+env.Origin+">", "2")
	if err != nil {
		env.recErr("", err.Error(), fatal(err))
		return
	}
	rcnt := 0
	for _, r := range env.Recipients {
		err = cs.act("RCPT TO:<"+r+">", "2")
		if err != nil {
			env.recErr(r, err.Error(), fatal(err))
			continue
		}
		rcnt++
	}
	if rcnt > 0 {
		err = cs.act("DATA", "3")
		if err != nil {
			env.recErr("", err.Error(), fatal(err))
			return
		}
		buf := make([]byte, 65536)
		cnt := 0
		for {
			in, err := io.ReadFull(msg, buf)
			cnt += in
			if in == 0 {
				break
			}
			_, err = cs.Write(buf[:in])
			if err != nil {
				env.recErr("", err.Error(), false)
				return
			}
		}
		env.Debugf("%s> %s (%d bytes)", server, path.Base(env.content), cnt)
		err = cs.act("\r\n.", "2")
		if err != nil {
			env.recErr("", err.Error(), fatal(err))
			return
		}
	}
	err = cs.act("QUIT", "2")
	if err != nil {
		env.recErr("", err.Error(), fatal(err))
	}
	return
}

func sendMail(file string, ss *Settings) {
	env := loadEnvelope(file, ss)
	if env == nil {
		return
	}
	defer env.flush(true)
	msg, err := os.Open(env.content)
	if err != nil {
		env.Log("RUNERR: " + err.Error())
		env.recErr("", err.Error(), false)
		return
	}
	defer msg.Close()
	mxs, err := net.LookupMX(env.domain)
	if err != nil {
		env.Debugf("GetMX: %v", err)
		env.recErr("", err.Error(), true)
		return
	}
	for _, mx := range mxs {
		if len(env.Recipients) == 0 {
			break
		}
		msg.Seek(0, 0)
		send(mx.Host, env, msg)
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

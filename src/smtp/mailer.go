package smtp

import (
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	err, _ = cs.act("MAIL FROM:<"+env.Origin+">", "2")
	if err != nil {
		env.recErr("", err.Error(), fatal(err))
		return
	}
	rcnt := 0
	for _, r := range env.Recipients {
		err, _ = cs.act("RCPT TO:<"+r+">", "2")
		if err != nil {
			env.recErr(r, err.Error(), fatal(err))
			continue
		}
		rcnt++
	}
	if rcnt > 0 {
		err, _ = cs.act("DATA", "3")
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
		err, _ = cs.act("\r\n.", "2")
		if err != nil {
			env.recErr("", err.Error(), fatal(err))
			return
		}
	}
	err, _ = cs.act("QUIT", "2")
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
		env.recErr("", err.Error(), false)
		return
	}
	defer msg.Close()
	var mxs []string
	if len(ss.Gateways) > 0 {
		mxs = ss.Gateways
	} else {
		mxrs, err := net.LookupMX(env.domain)
		if err != nil {
			env.recErr("", err.Error(), true)
			return
		}
		mxs = make([]string, 0, len(mxrs))
		for _, mxr := range mxrs {
			mxs = append(mxs, mxr.Host)
		}
	}
	for _, mx := range mxs {
		if len(env.Recipients) == 0 {
			break
		}
		msg.Seek(0, 0)
		send(mx, env, msg)
	}
}

func SendMails(spool string, ss *Settings) {
	ecnt := 0
	files, err := filepath.Glob(spool + "/*.*")
	if err == nil {
		for _, f := range files {
			fn := path.Base(f)
			if strings.HasSuffix(f, ".env") {
				p := strings.Split(fn, "@")
				p = strings.Split(p[0], ".")
				ts, err := strconv.ParseInt(p[0], 36, 64)
				if err == nil {
					if ts+int64(ss.expire) <= time.Now().Unix() {
						ss.Debugf("SendMail: removing obsolete envelope: " + fn)
						purgeMsg(f, ss)
					} else {
						ecnt++
						go sendMail(f, ss)
					}
				} else {
					ss.Logf("RUNERR: invalid envelope: %s", fn)
				}
			} else {
				env, _ := filepath.Glob(f[0:len(f)-4] + "@*.env")
				if len(env) == 0 {
					ss.Debugf("SendMail: removing obsolete message: " + fn)
					purgeMsg(f, ss)
				}
			}
		}
		ss.Debugf("SendMails: queued_messages=%v", ecnt)
	} else {
		ss.Logf("RUNERR: %v", err)
	}
}

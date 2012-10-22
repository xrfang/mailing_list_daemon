package smtp

import (
	"log4g"
	"net"
	"path"
	"path/filepath"
)

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
	mxs, err := net.LookupMX(env.domain)
	if err != nil {
		ss.Log("RUNERR: " + err.Error())
		return
	}
	for _, mx := range mxs {
		if send(mx.Host, env, ss) {
			break
		}
	}
	err = env.flush(ss)
	if err != nil {
		ss.Log("RUNERR: " + err.Error())
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

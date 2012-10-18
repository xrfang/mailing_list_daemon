package smtp

import (
	"encoding/json"
	"log4g"
	"os"
	"path"
	"path/filepath"
	"time"
)

type relayRoute map[string]map[string]int64

func loadRoute(env string) (route relayRoute, err error) {
	route = make(relayRoute)
	ef, err := os.OpenFile(env, os.O_RDWR, 0600)
	if err != nil {
		return
	}
	defer ef.Close()
	msg := []byte(env)
	copy(msg[len(msg)-3:len(msg)], "msg")
	mf, err := os.OpenFile(string(msg), os.O_RDWR, 0600)
	if err != nil {
		return
	}
	mf.Close()
	dec := json.NewDecoder(ef)
	err = dec.Decode(&route)
	return
}

func saveRoute(env string, route relayRoute) (err error) {
	tmpfile := os.TempDir() + "/" + path.Base(env)
	tf, err := os.Create(tmpfile)
	if err != nil {
		return
	}
	defer tf.Close()
	enc := json.NewEncoder(tf)
	if err = enc.Encode(&route); err == nil {
		err = os.Rename(tmpfile, env)
	}
	return
}

func sendMail(env string, logger log4g.Logger) {
	route, err := loadRoute(env)
	if err != nil {
		logger.Log("RUNERR: " + err.Error())
		return
	}
	now := time.Now().Unix()
	sched := route["STATUS"]["schedule"]
	logger.Debugf("%s: schedued=%d, now=%d", path.Base(env[:len(env)-4]), sched, now)
	if sched > now {
		return
	}
	route["STATUS"]["schedule"] = now + 3600 //by default only retry after 1 hour
	err = saveRoute(env, route)
	if err != nil {
		logger.Log("RUNERR: " + err.Error())
		return
	}
	logger.Log("TODO: send - " + env)
}

func SendMails(spool string, logger log4g.Logger) {
	envelopes, err := filepath.Glob(spool + "/*.env")
	if err == nil {
		logger.Debugf("SendMails: queued_messages=%v", len(envelopes))
		for _, e := range envelopes {
			go sendMail(e, logger)
		}
	} else {
		logger.Log("RUNERR: " + err.Error())
	}
}

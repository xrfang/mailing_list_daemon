package smtp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type envelope struct {
	Sender     string
	Recipients []string
	Attempted  int
	domain     string
	file       string
	content    string
	errors     map[string]string
}

func loadEnvelope(file string, lock uint32) (*envelope, error) {
	p := strings.Split(file, "@")
	ts, err := strconv.ParseInt(p[2][0:len(p[2])-4], 36, 64)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	if ts > now {
		//the scheduled time for this mail is not reached yet
		return nil, nil
	}
	var env envelope
	ef, err := os.OpenFile(file, os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer ef.Close()
	msg := p[0] + ".msg"
	mf, err := os.OpenFile(msg, os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	mf.Close()
	dec := json.NewDecoder(ef)
	err = dec.Decode(&env)
	if err != nil {
		return nil, err
	}
	ef.Close()
	if lock > 0 {
		now += int64(lock)
		newfile := fmt.Sprintf("%s@%s@%s.env", p[0], p[1], strconv.FormatInt(now, 36))
		err = os.Rename(file, newfile)
		env.file = newfile
	} else {
		env.file = file
	}
	env.content = msg
	env.domain = p[1]
	env.errors = make(map[string]string)
	return &env, err
}

func (e *envelope) log(rcpt string, msg string) {
	println("Logging envelope error: " + msg)
	e.errors[rcpt] = msg
}

func (e *envelope) flush(ss *Settings) error {
	ss.Log("TODO: update envelope")
	if e.file == "" {
		return errors.New("Attempted to flush a flushed envelope")
	}
	for d, msg := range e.errors {
		ss.Log("TODO: process error: " + d + "=" + msg)
	}
	e.file = ""
	return nil
}

package smtp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

type envelope struct {
	Sender     string
	Recipients []string
	Attempted  int
	Origin     string
	domain     string
	file       string
	content    string
	errors     map[string]string
}

func LoadEnvelope(file string, lock uint32) (*envelope, error) {
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

func (e *envelope) log(rcpt string, msg string, fatal bool) {
	if fatal {
		e.errors[rcpt] = "!" + msg
	} else {
		e.errors[rcpt] = "?" + msg
	}
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

func (e envelope) Bounce(rcpts []string, errmsg string) (err error) {
	if e.Sender == e.Origin {
		return //Bounce of bounced messages are not allowed
	}
	omsg, err := os.Open(e.content)
	if err != nil {
		return
	}
	defer omsg.Close()
	s := strings.SplitN(e.Sender, "@", 2)
	msgid := newMsgId() + ".1"
	path := path.Dir(e.content) + "/" + msgid
	mfn := path + ".msg"
	efn := path + "@" + s[len(s)-1] + "@0.env"
	bmsg, err := os.Create(mfn)
	if err != nil {
		return
	}
	defer bmsg.Close()
	var msg bytes.Buffer
	msg.Write([]byte("From: " + e.Origin + "\r\n"))
	msg.Write([]byte("To: " + e.Sender + "\r\n"))
	msg.Write([]byte("Subject: Delivery Status Notification (Failure)\r\n"))
	msg.Write([]byte("Message-ID: <" + msgid + ">\r\n"))
	msg.Write([]byte("Date: " + time.Now().String() + "\r\n"))
	msg.Write([]byte("Content-Type: text/plain; charset=ISO-8859-1\r\n"))
	msg.Write([]byte("Content-Transfer-Encoding: quoted-printable\r\n\r\n"))
	msg.Write([]byte("Delivery to the following recipient(s) failed:\r\n\r\n"))
	for _, r := range rcpts {
		msg.Write([]byte("    " + r + "\r\n"))
	}
	msg.Write([]byte(fmt.Sprintf("\r\nWe have tried to deliver this message for %d time(s), and=\r\n", e.Attempted)))
	msg.Write([]byte("the last error encountered was: \r\n\r\n"))
	msg.Write([]byte("    " + errmsg + "\r\n\r\n"))
	msg.Write([]byte("Please check if you have used correct recpient address, or=\r\n"))
	msg.Write([]byte("contact the other email provider for further information=\r\n"))
	msg.Write([]byte("about the course of this error.\r\n"))
	msg.Write([]byte("\r\n----- Original message -----\r\n\r\n"))
	msg.Write([]byte("TODO: read header of original message"))
	msg.WriteTo(bmsg)
	//todo: check error of the writeto!
	benv, err := os.Create(efn)
	if err != nil {
		return
	}
	defer benv.Close()
	env := envelope{
		Sender:     e.Origin,
		Recipients: []string{e.Sender},
		Attempted:  0,
		Origin:     e.Origin,
	}
	enc := json.NewEncoder(benv)
	err = enc.Encode(&env)
	if err != nil {
		os.Remove(mfn)
		os.Remove(efn)
	}
	return
}

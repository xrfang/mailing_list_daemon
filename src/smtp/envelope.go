package smtp

import (
	"bufio"
	"bytes"
	"encoding/json"
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
	*Settings
}

func loadEnvelope(file string, ss *Settings) *envelope {
	var err error
	var env envelope
	defer func() {
		if err != nil {
			ss.Log("RUNERR: " + err.Error())
		}
	}()
	p := strings.Split(file, "@")
	ts, err := strconv.ParseInt(p[2][0:len(p[2])-4], 36, 64)
	if err != nil {
		return nil
	}
	now := time.Now().Unix()
	if ts > now {
		ss.Debug("On hold: " + path.Base(file))
		return nil //scheduled time for this mail is not reached yet
	}
	ef, err := os.OpenFile(file, os.O_RDWR, 0600)
	if err != nil {
		return nil
	}
	defer ef.Close()
	msg := p[0] + ".msg"
	mf, err := os.OpenFile(msg, os.O_RDWR, 0600)
	if err != nil {
		return nil
	}
	mf.Close()
	dec := json.NewDecoder(ef)
	err = dec.Decode(&env)
	if err != nil {
		return nil
	}
	ef.Close()
	now += int64(3600) //TODO: add locking period to Settings!
	newfile := fmt.Sprintf("%s@%s@%s.env", p[0], p[1], strconv.FormatInt(now, 36))
	err = os.Rename(file, newfile)
	if err != nil {
		return nil
	}
	env.Attempted += 1
	env.file = newfile
	env.content = msg
	env.domain = p[1]
	env.errors = make(map[string]string)
	env.Settings = ss
	return &env
}

func (e *envelope) log(rcpt string, msg string, fatal bool) {
	if fatal {
		e.errors[rcpt] = "!" + msg
	} else {
		e.errors[rcpt] = "?" + msg
	}
}

func (e *envelope) flush(final bool) {
	if e.file == "" {
		return
	}
	//TODO: save envelope
	e.file = ""
	if final {
		e.Attempted += 1
	}
	var (
		rcpts []string
		errmsg string
	)
	errmsg, found := e.errors[""]
	if found {
		if errmsg[0] == '!' || e.Attempted >= len(e.Retries) {
			e.bounce(e.Recipients, errmsg[1:], 0)
			//TODO: REMOVE envelope
		}
	} else {
		for r, msg := range e.errors {
			if msg[0] == '!' {
				e.bounce([]string{r}, msg[1:], 0)
				delete(e.errors, r)
			}
		}
	}
	return
}

func (e envelope) bounce(failed []string, errmsg string, route int) {
	if e.Sender == e.Origin {
		return //Bounce of bounced messages are not allowed
	}
	var (
		err error
		dest string
	)
	s := strings.SplitN(dest, "@", 2)
	msgid := newMsgId() + ".0"
	path := path.Dir(e.content) + "/" + msgid
	mfn := path + ".msg"
	efn := path + "@" + s[len(s)-1] + "@0.env"
	if route == 0 {
		dest = e.Sender //TODO: Get Return-Path
	} else {
		dest = e.Origin
	}
	defer func() {
		if err != nil {
			e.Log("RUNERR: " + err.Error())
			os.Remove(mfn)
			os.Remove(efn)
		}
	}()
	omsg, err := os.Open(e.content)
	if err != nil {
		return
	}
	defer omsg.Close()
	bmsg, err := os.Create(mfn)
	if err != nil {
		return
	}
	defer bmsg.Close()
	var msg bytes.Buffer
	msg.Write([]byte("From: " + e.Origin + "\r\n"))
	msg.Write([]byte("To: " + dest + "\r\n"))
	msg.Write([]byte("Subject: Delivery Status Notification (Failure)\r\n"))
	msg.Write([]byte("Message-ID: <" + msgid + ">\r\n"))
	msg.Write([]byte("Date: " + time.Now().String() + "\r\n"))
	msg.Write([]byte("Content-Type: text/plain; charset=ISO-8859-1\r\n"))
	msg.Write([]byte("Content-Transfer-Encoding: quoted-printable\r\n\r\n"))
	msg.Write([]byte("Delivery to the following recipient(s) failed:\r\n\r\n"))
	for _, r := range failed {
		msg.Write([]byte("    " + r + "\r\n"))
	}
	msg.Write([]byte("\r\nWe have tried our best to deliver this message, unfortunately=\r\n"))
	msg.Write([]byte("it didn't work.  The last error encountered was:\r\n\r\n"))
	msg.Write([]byte("    " + errmsg + "\r\n\r\n"))
	msg.Write([]byte("Please check if you have used correct recpient address, or=\r\n"))
	if route == 0 {
		msg.Write([]byte("contact the other email provider for further information=\r\n"))
	} else {
		msg.Write([]byte("contact " + e.Origin + " for further information=\r\n"))
	}
	msg.Write([]byte("about the cause of this error.\r\n"))
	msg.Write([]byte("\r\n----- Original message -----\r\n\r\n"))
	br := bufio.NewReader(omsg)
	cnt := 0
	for cnt < 100 {
		line, err := br.ReadString('\n')
		if err != nil || line == "\r\n" {
			break
		}
		msg.Write([]byte(line))
		cnt++
	}
	_, err = msg.WriteTo(bmsg)
	if err != nil {
		return
	}
	benv, err := os.Create(efn)
	if err != nil {
		return
	}
	defer benv.Close()
	env := envelope{
		Sender:     e.Origin,
		Recipients: []string{dest},
		Attempted:  0,
		Origin:     e.Origin,
	}
	enc := json.NewEncoder(benv)
	err = enc.Encode(&env)
	return
}


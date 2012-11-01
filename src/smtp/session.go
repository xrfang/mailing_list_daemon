package smtp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	PROC_QUEUED = iota //add mail to queue
	PROC_SUBMIT        //confirm relay of queue (move inbound to outbound)
	PROC_FLUSH         //discard queued mail for this svrSession
)

func newMsgId() string {
	now := int64(time.Now().UnixNano() / 1000)
	rand.Seed(now)
	sec := now / 1000000
	mic := now % 1000000
	return fmt.Sprintf("%s.%s%s",
		strconv.FormatInt(sec, 36),
		strconv.FormatInt(mic, 36),
		strconv.FormatInt(int64(rand.Intn(1024)), 36))
}

func normalize(addr string) (string, string) {
	cmd := ""
	parts := strings.Split(addr, ":")
	if len(parts) >= 2 {
		cmd = strings.ToUpper(strings.TrimSpace(parts[0]))
		parts = strings.SplitN(parts[len(parts)-1], "<", 2)
		addr = strings.TrimSpace(strings.SplitN(parts[len(parts)-1], ">", 2)[0])
		parts = strings.SplitN(addr, "@", 2)
		if len(parts) > 1 && len(parts[1]) > 0 {
			addr = parts[0] + "@" + strings.ToLower(parts[1])
		} else {
			addr = parts[0]
		}
	}
	return cmd, addr
}

type svrSession struct {
	conn       net.Conn
	path       string
	state      byte
	seq        int
	sender     string
	recipients map[string]byte
	file       *os.File
	p_errs     byte //protocol errors (e.g. syntex error, command out-of-order)
	r_errs     byte //relay errors
	*Settings
}

func (s svrSession) expects() (reply string) {
	reply = "503 Bad sequence of commands"
	cmds := ""
	switch s.state {
	case 1:
		cmds = "EHLO, HELO"
	case 2:
		cmds = "MAIL"
	default:
		if len(s.recipients) == 0 {
			cmds = "RCPT"
		}
	}
	if len(cmds) > 0 {
		reply += ", expecting: " + cmds
	}
	return
}

func (s svrSession) expnList(ctrl map[string][]string, list []string, name string) {
	for _, r := range list {
		if r == name {
			s.Log("CFGERR: Cyclic recipient name: " + r)
		} else {
			at := strings.Index(r, "@")
			if at > 0 && at < len(r)-1 {
				s.Debugf("%s>   =>%s", s.CliAddr(), r)
				s.recipients[r] = 1
			} else {
				expn, ok := ctrl[r]
				if ok {
					s.Debugf("%s>   =>[%s, %d addr(s)]", s.CliAddr(), r, len(expn))
					s.expnList(ctrl, expn, r)
				} else {
					s.Log("CFGERR: Unresolved recpient: " + r)
				}
			}
		}
	}
}

func (s *svrSession) relay(addr string) string {
	parts := strings.SplitN(addr, "@", 2)
	if len(parts) < 2 {
		return "Relay denied for " + addr
	}
	ctrl, ok := s.RelayCtrl[parts[1]]
	if !ok {
		return "Relay denied for " + addr
	}
	expn, ok := ctrl[parts[0]]
	if !ok {
		return "Relay denied for " + addr
	}
	rcpts, ok := ctrl[s.sender]
	if !ok {
		return "Relay denied for " + s.sender
	}
	if len(rcpts) > 0 {
		noMatch := true
		for _, u := range rcpts {
			if u == parts[0] {
				noMatch = false
				break
			}
		}
		if noMatch {
			return "Relay denied for " + s.sender
		}
	}
	s.expnList(ctrl, expn, parts[0])
	return ""
}

func (s svrSession) CliAddr() string {
	addr := s.conn.RemoteAddr().String()
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		addr = strings.Split(addr, ":")[0]
	}
	return addr
}

func (s svrSession) svrAddr() string {
	return strings.Split(s.conn.LocalAddr().String(), ":")[0]
}

func (s *svrSession) Reset(reason byte) {
	if s.file != nil {
		s.file.Close()
		s.file = nil
	}
	s.state = 2
	s.sender = ""
	s.recipients = make(map[string]byte)
	idir := s.Spool + "/inbound/" + s.path + "/"
	odir := s.Spool + "/outbound/"
	ls := len(s.Spool + "/inbound/")
	switch reason {
	case PROC_SUBMIT:
		os.MkdirAll(odir, 0777)
		dir, err := os.Open(idir)
		if err == nil {
			msgs, err := dir.Readdirnames(0)
			if err != nil {
				s.Log("PROC_SUBMIT_READDIR: " + err.Error())
			}
			s.Debug("Queueing inbound messages...")
			envs := 0
			for _, fn := range msgs {
				if strings.HasSuffix(fn, ".env") {
					envs++
				}
				fi := idir + fn
				fo := odir + s.path + "." + fn
				s.Debugf("  %s", fi[ls:])
				err = os.Rename(fi, fo)
				if err != nil {
					s.Logf("PROC_SUBMIT_MOVEFILE(%s): %s", fi, err.Error())
				}
			}
			s.Debugf("Envelope(s) queued: %d", envs)
		} else if !os.IsNotExist(err) {
			s.Log("PROC_SUBMIT_OPENDIR: " + err.Error())
		}
	case PROC_FLUSH:
		os.RemoveAll(s.Spool + "/inbound/" + s.path)
	}
}

func (s *svrSession) prep() error {
	inbound := s.Spool + "/inbound/" + s.path
	err := os.MkdirAll(inbound, 0777)
	if err != nil {
		return err
	}
	domains := make(map[string][]string)
	for r, _ := range s.recipients {
		p := strings.SplitN(r, "@", 2)
		domains[p[1]] = append(domains[p[1]], r)
	}
	fromDomain := s.domain(s.sender)
	for d, u := range domains {
		file, err := os.Create(fmt.Sprintf("%s/%d@%s@0.env", inbound, s.seq, d))
		if err != nil {
			return err
		}
		defer file.Close()
		env := envelope{
			Sender:     s.sender,
			Recipients: u,
			Attempted:  0,
			Origin:     "postmaster@" + fromDomain,
		}
		enc := json.NewEncoder(file)
		if err = enc.Encode(&env); err != nil {
			return err
		}
	}
	s.file, err = os.Create(fmt.Sprintf("%s/%d.msg", inbound, s.seq))
	if err == nil {
		_, err = s.file.Write([]byte("Received: from " + strings.Split(s.CliAddr(), ":")[0] + " by " + fromDomain + "; " + time.Now().String()))
	}
	return nil
}

func (s *svrSession) handle(cmdline []byte) string {
	cmdstr := string(cmdline)
	if s.state < 4 {
		chunks := strings.SplitN(cmdstr, " ", 2)
		cmd := strings.ToUpper(chunks[0])
		param := ""
		if len(chunks) > 1 {
			param = chunks[1]
		}
		s.Debug(s.CliAddr() + "> " + cmdstr)
		switch cmd {
		case "EHLO", "HELO":
			s.state = 2
			return "250 At your service"
		case "DATA":
			if s.state < 3 {
				s.p_errs++
				return s.expects()
			}
			err := s.prep()
			if err == nil {
				s.state = 4
				return "354 Go ahead"
			}
			s.Logf("%s: ERROR! %s", s.CliAddr(), err.Error())
			s.state = 0
			return "421 Service temporarily unavailable"
		case "MAIL":
			if s.state < 2 {
				s.p_errs++
				return s.expects()
			}
			cmd, addr := normalize(param)
			if cmd == "FROM" {
				s.Debugf("%s>   =[%s]", s.CliAddr(), addr)
				s.sender = addr
				s.state = 3
				return "250 OK"
			} else {
				s.p_errs++
				return "500 Syntax error"
			}
		case "NOOP":
			return "250 OK"
		case "RCPT":
			if s.state < 3 {
				s.p_errs++
				return s.expects()
			}
			cmd, addr := normalize(param)
			if cmd == "TO" {
				s.Debugf("%s>   =[%s]", s.CliAddr(), addr)
				if msg := s.relay(addr); len(msg) > 0 {
					s.r_errs++
					return "553 " + msg
				}
				return "250 OK"
			} else {
				s.p_errs++
				return "500 Syntax error"
			}
		case "QUIT":
			s.Reset(PROC_SUBMIT)
			s.state = 0
			return "220 closing connection"
		case "RSET":
			s.Reset(PROC_FLUSH)
			return "250 Flushed"
		default:
			s.p_errs++
			return "502 Command not implemented"
		}
	} else {
		s.Debug(s.CliAddr() + "> " + cmdstr)
		if cmdstr == "." {
			s.Reset(PROC_QUEUED)
			return "250 OK"
		} else {
			s.file.Write([]byte("\r\n" + cmdstr))
		}
	}
	return ""
}

func (s *svrSession) Serve() error {
	br := bufio.NewReader(s.conn)
	for {
		s.conn.SetDeadline(time.Now().Add(5 * time.Minute))
		cmd, xl, err := br.ReadLine()
		if err != nil {
			return err
		}
		if xl {
			return errors.New("Line too long")
		}
		reply := s.handle(cmd)
		if len(reply) > 0 {
			s.Debug(s.CliAddr() + "< " + string(reply))
			s.conn.Write([]byte(reply + "\r\n"))
		}
		if s.state <= 0 || s.p_errs > 2 || s.r_errs > 2 {
			if s.p_errs > 0 || s.r_errs > 0 {
				s.Logf("%s: ERROR! P=%d, R=%d", s.CliAddr(), s.p_errs, s.r_errs)
			}
			break
		}
	}
	return nil
}

func NewSvrSession(conn net.Conn, env *Settings) (*svrSession, error) {
	path := newMsgId()
	err := os.MkdirAll(env.Spool+"/inbound/"+path, 0777)
	if err != nil {
		return nil, err
	}
	ss := &svrSession{
		conn,
		path,
		1,  //state
		1,  //seq
		"", //sender
		make(map[string]byte), //recipients
		nil,                   //file
		0,                     //p_errs
		0,                     //r_errs
		env,                   //Settings
	}
	_, err = conn.Write([]byte("220 Service ready\r\n"))
	return ss, err
}

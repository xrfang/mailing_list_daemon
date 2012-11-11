package smtp

import (
	"bufio"
	"errors"
	"fmt"
	"log4g"
	"net"
	"strings"
	"time"
)

type cliSession struct {
	server string
	reader *bufio.Reader
	lg     log4g.Logger
	net.Conn
}

func (s *cliSession) act(cmd string, expect string) (error, []string) {
	if len(cmd) > 0 {
		s.lg.Debug(s.server + "> " + cmd)
		_, err := s.Write([]byte(cmd + "\r\n"))
		if err != nil {
			return err, nil
		}
	}
	var reply []string
	for {
		s.SetDeadline(time.Now().Add(5 * time.Minute))
		msg, long, err := s.reader.ReadLine()
		if err != nil {
			return err, nil
		}
		if len(msg) == 0 {
			continue
		}
		s.lg.Debug(s.server + "< " + string(msg))
		if long {
			return errors.New(fmt.Sprintf("Server returned (too long): %s...", msg[:20])), nil
		}
		if len(msg) < 3 {
			return errors.New(fmt.Sprintf("Invalid SMTP reply: %s", msg)), nil
		}
		reply = append(reply, string(msg))
		if len(msg) == 3 || msg[3] == ' ' {
			if expect == "" {
				break
			}
			code := string(msg[:3])
			if strings.HasPrefix(code, expect) {
				break
			} else {
				return errors.New(string(msg)), nil
			}
		}
	}
	return nil, reply
}

func NewCliSession(server string, env *envelope) (*cliSession, error) {
	if strings.Index(server, ":") < 0 {
		server = server + ":25"
	}
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return nil, err
	}
	cs := &cliSession{
		server,
		bufio.NewReader(conn),
		env.SysLogger,
		conn,
	}
	err, _ = cs.act("", "2")
	if err != nil {
		return nil, err
	}
	p := strings.Split(env.Origin, "@")
	origin := p[len(p)-1]
	err, _ = cs.act("EHLO "+origin, "2")
	if err != nil {
		err, _ = cs.act("HELO "+origin, "")
	}
	return cs, err
}

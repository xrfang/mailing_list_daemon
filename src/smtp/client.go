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

func (s *cliSession) act(cmd string, expect string) error {
	if len(cmd) > 0 {
		s.lg.Debug(s.server + "> " + cmd)
		_, err := s.Write([]byte(cmd + "\r\n"))
		if err != nil {
			return err
		}
	}
	for {
		s.SetDeadline(time.Now().Add(5 * time.Minute))
		msg, long, err := s.reader.ReadLine()
		if len(msg) == 0 {
			continue
		}
		s.lg.Debug(s.server + "< " + string(msg))
		if err != nil {
			return err
		}
		if long {
			return errors.New(fmt.Sprintf("Server returned (too long): %s...", msg[:20]))
		}
		if len(msg) < 3 {
			return errors.New(fmt.Sprintf("Invalid SMTP reply: %s", msg))
		}
		if len(msg) == 3 || msg[3] == ' ' {
			if expect == "" {
				return nil
			}
			code := string(msg[:3])
			if strings.HasPrefix(code, expect) {
				return nil
			} else {
				return errors.New(string(msg))
			}
		}
	}
	return nil
}

func NewCliSession(server string, logger log4g.Logger) (*cliSession, error) {
	conn, err := net.Dial("tcp", server+":25")
	if err != nil {
		return nil, err
	}
	cs := &cliSession{
		server,
		bufio.NewReader(conn),
		logger,
		conn,
	}
	err = cs.act("", "2")
	if err != nil {
		return nil, err
	}
	err = cs.act("EHLO localhost", "")
	return cs, err
}

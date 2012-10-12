package log4g

import (
	"fmt"
	"log/syslog"
	"runtime"
	"strings"
)

const (
	QUITE_MODE = false
	DEBUG_MODE = true
)

type SysLogger struct {
	verbose bool
	writer  *syslog.Writer
}

func NewSysLogger(ident string, verbose bool) (*SysLogger, error) {
	writer, err := syslog.New(syslog.LOG_ALERT, ident)
	return &SysLogger{verbose: verbose, writer: writer}, err
}

func (sl *SysLogger) Mode(verbose bool) {
	sl.verbose = verbose
}

func (sl SysLogger) Logf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	sl.writer.Write([]byte(msg))
}

func (sl SysLogger) Log(v interface{}) {
	sl.Logf("%v", v)
}

func (sl SysLogger) Debugf(format string, v ...interface{}) {
	if sl.verbose {
		sl.Logf(format, v...)
	}
}

func (sl SysLogger) Debug(v interface{}) {
	sl.Debugf("%v", v)
}

func (sl SysLogger) Panic(v interface{}) {
	var cnt int
	sl.Log(v)
	stack := make([]byte, 8192)
	if sl.verbose {
		cnt = runtime.Stack(stack, true)
	} else {
		cnt = runtime.Stack(stack, false)
	}
	lines := strings.Split(string(stack[:cnt]), "\n")
	for _, line := range lines {
		sl.Log(strings.TrimSpace(line))
	}
	panic(v)
}

package main

import (
	"fmt"
	"net"
	"os"
	"path"
	"smtp"
	"time"
)

var (
	environ       smtp.Settings
	rateLimit chan int
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("USAGE: %s <config file>\n", path.Base(os.Args[0]))
		os.Exit(1)
	}
	environ, err := smtp.LoadSettings(os.Args[1])
	if err != nil {
		if environ == nil {
			panic(err)
		} else {
			environ.Panic(err)
		}
	}
	defer func() {
		err := recover()
		if err != nil {
			environ.Panic(err)
		}
	}()
	rateLimit = make(chan int, environ.MaxCli)
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(environ.Bind), Port: environ.Port})
	if err != nil {
		panic(err)
	}
	ln.SetDeadline(time.Now().Add(1 * time.Minute))
	svrState := "SMTP" + environ.Dump()
	environ.Log(svrState)
	fmt.Println(svrState)
	go smtp.MXResolver()
	for {
		go smtp.SendMails(environ.Spool+"/outbound", environ)
		conn, err := ln.Accept()
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Temporary() {
				if !opErr.Timeout() {
					environ.Log("RUNERR: " + opErr.Error())
				}
				ln.SetDeadline(time.Now().Add(1 * time.Minute))
				continue
			} else {
				panic(err)
			}
		}
		select {
		case rateLimit <- 1:
		default:
			environ.Debug("Overloaded: " + conn.RemoteAddr().String())
			conn.Write([]byte("421 Service temporarily unavailable\r\n"))
			conn.Close()
			continue
		}
		environ.Debug("Connected: " + conn.RemoteAddr().String())
		go func(environ *smtp.Settings) {
			s, err := smtp.NewSession(conn, environ)
			if err != nil {
				environ.Panic(err)
			}
			defer func() {
				environ.Debug("Disconnected: " + conn.RemoteAddr().String())
				conn.Close()
				<-rateLimit
				s.Reset(smtp.PROC_FLUSH)
				err := recover()
				if err != nil {
					environ.Panic(err)
				}
			}()
			err = s.Serve()
			if err != nil {
				environ.Logf("%s: ERROR! %s", s.CliAddr(), err.Error())
			}
		}(environ)
	}
}

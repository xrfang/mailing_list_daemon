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
	env       smtp.Settings
	rateLimit chan int
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("USAGE: %s <config file>\n", path.Base(os.Args[0]))
		os.Exit(1)
	}
	env, err := smtp.LoadSettings(os.Args[1])
	if err != nil {
		if env == nil {
			panic(err)
		} else {
			env.Panic(err)
		}
	}
	defer func() {
		err := recover()
		if err != nil {
			env.Panic(err)
		}
	}()
	rateLimit = make(chan int, env.MaxCli)
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(env.Bind), Port: env.Port})
	if err != nil {
		panic(err)
	}
	ln.SetDeadline(time.Now().Add(1 * time.Minute))
	svrState := "SMTP" + env.Dump()
	env.Log(svrState)
	fmt.Println(svrState)
	for {
		go smtp.SendMails(env.Spool+"/outbound", env)
		conn, err := ln.Accept()
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Temporary() {
				if !opErr.Timeout() {
					env.Log("RUNERR: " + opErr.Error())
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
			env.Debug("Overloaded: " + conn.RemoteAddr().String())
			conn.Write([]byte("421 Service temporarily unavailable\r\n"))
			conn.Close()
			continue
		}
		env.Debug("Connected: " + conn.RemoteAddr().String())
		go func(env *smtp.Settings) {
			s, err := smtp.NewSession(conn, env)
			if err != nil {
				env.Panic(err)
			}
			defer func() {
				env.Debug("Disconnected: " + conn.RemoteAddr().String())
				conn.Close()
				<-rateLimit
				s.Reset(smtp.PROC_FLUSH)
				err := recover()
				if err != nil {
					env.Panic(err)
				}
			}()
			err = s.Serve()
			if err != nil {
				env.Logf("%s: ERROR! %s", s.CliAddr(), err.Error())
			}
		}(env)
	}
}

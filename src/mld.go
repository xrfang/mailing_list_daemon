package main

import (
	"fmt"
	"net"
	"os"
	"path"
	"smtp"
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
	ln, err := net.Listen("tcp", env.Bind+":"+env.Port)
	if err != nil {
		panic(err)
	}
	svrState := "SMTP" + env.Dump()
	env.Log(svrState)
	fmt.Println(svrState)
	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
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

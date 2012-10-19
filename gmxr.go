package main

import (
	"fmt"
	"net"
	"os"
	"path"
	"sync"
)

var resolver chan chan string
func init() {
	resolver = make(chan chan string)
}
func MXResolver() {
	for {
		ch := <-resolver
		go func(c chan string) {
			domain := <-c
			mx, err := net.LookupMX(domain)
			if err == nil {
				cnt := 0
				for i := 0; i < len(mx); i++ {
					ips, err := net.LookupIP(mx[i].Host)
					if err == nil {
						for _, ip := range ips {
							c <- "@" + ip.String()
							cnt++
						}
					}
				}
				if cnt == 0 {
					c <- "!Cannot get MX record for " + domain
				}
			} else {
				c <- "!" + err.Error()
			}
			close(c)
		}(ch)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("USAGE: %s <domain1> <domain2> ... \n", path.Base(os.Args[0]))
		return
	}
	go MXResolver()
	var wg sync.WaitGroup
	for i := 1; i < len(os.Args); i++ {
		wg.Add(1)
		go func(domain string) {
			done := false
			ch := make(chan string)
			resolver <- ch
			ch <- domain
			for {
				ip, ok := <-ch
				if !ok {
					break
				}
				if ip[0] == '@' {
					if !done {
						fmt.Printf("TODO: connect to %s to send mail...\n", ip[1:])
						done = true
					}
				} else {
					//fmt.Println("  " + ip[1:])
					//when bail-out, the message is useful
					fmt.Printf("TODO: %s has problems, increase counter\n", domain)					
				}
			}
			wg.Done()
		}(os.Args[i])
	}
	wg.Wait()
}

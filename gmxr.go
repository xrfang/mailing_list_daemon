package main

import (
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"sync"
)

func MXResolver(ctrl chan chan string) {
	for {
		ch := <-ctrl
		go func(c chan string) {
			domain := <-c
			mx, err := net.LookupMX(domain)
			if err == nil {
				cnt := 0
				for i := 0; i < len(mx); i++ {
					ips, err := net.LookupIP(mx[i].Host)
					if err == nil {
						for _, ip := range ips {
							c <- fmt.Sprintf("@%s=%d", ip.String(), mx[i].Pref)
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
	mxrc := make(chan chan string)
	go MXResolver(mxrc)
	var wg sync.WaitGroup
	for i := 1; i < len(os.Args); i++ {
		wg.Add(1)
		go func(domain string) {
			mx := make(map[string]string)
			ch := make(chan string)
			mxrc <- ch
			ch <- domain
			for {
				ip, ok := <-ch
				if !ok {
					break
				}
				if ip[0] == '@' {
					s := strings.Split(ip[1:], "=")
					mx[s[0]] = s[1]
				} else {
					fmt.Println(domain)
					fmt.Println("  " + ip[1:])
				}
			}
			if len(mx) > 0 {
				fmt.Println(domain)
				for k, v := range mx {
					fmt.Println("  " + k + "\t" + v)
				}
			}
			wg.Done()
		}(os.Args[i])
	}
	wg.Wait()
}

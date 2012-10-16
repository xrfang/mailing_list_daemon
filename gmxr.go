package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
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
	mxrc := make(chan chan string)
	go MXResolver(mxrc)
	fmt.Println("Go MX Resolver Ready")
	br := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		input, _, err := br.ReadLine()
		if err != nil {
			panic(err)
		}
		name := strings.ToLower(strings.TrimSpace(string(input)))
		if name == "quit" || name == "exit" {
			break
		}
		ch := make(chan string)
		mxrc <- ch
		ch <- name
		for {
			ip, ok := <-ch
			if !ok {
				break
			}
			if ip[0] == '@' {
				fmt.Println(ip[1:])
			} else {
				fmt.Println("ERROR: " + ip[1:])
			}
		}
	}
}

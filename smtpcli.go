package main

import (
    "bufio"
	"net"
	"os"
	"fmt"
	"time"
)

func main() {
    conn, err := net.Dial("tcp", os.Args[1] + ":25")
	if err != nil {
		panic(err)
	}
	fmt.Println("connected")
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	br := bufio.NewReader(conn)
	for {
		msg, _, err := br.ReadLine()
		if err != nil {
			fmt.Println(err.Error())
			break
		}
		fmt.Println(string(msg))
	}
	conn.Close()
}

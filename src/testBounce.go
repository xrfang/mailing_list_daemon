package main

import (
    "fmt"
    "smtp"
    "os"
)

func main() {
    env, err := smtp.LoadEnvelope(os.Args[1], 3)
    if err != nil {
        fmt.Println(err)
    }
    fmt.Println(env.Bounce([]string{"xrfang@gmail.com"}, "550 Access denied"))
}

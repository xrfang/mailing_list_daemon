package main

import (
	"errors"
	"fmt"
	"strings"
)

func baseConv(num string, FromBase int, ToBase int) (string, error) {
	charMap := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	res := ""
	if FromBase < 2 || FromBase > 62 {
		return "", errors.New("<FromBase> must be 2..62")
	}
	if ToBase < 2 || ToBase > 62 {
		return "", errors.New("<ToBase> must be 2..62")
	}
	if FromBase < 37 {
		num = strings.ToLower(num)
	}
	length := len(num)
	digits := make([]int, length)
	for i := 0; i < length; i++ {
		pos := strings.Index(charMap, string(num[i]))
		if pos < 0 || pos >= FromBase {
			return "", errors.New(fmt.Sprintf("Invalid digit <%c> in base %d", num[i], FromBase))
		}
		digits[i] = pos
	}
	for {
		divide := 0
		newlen := 0
		for i := 0; i < length; i++ {
			divide = divide*FromBase + digits[i]
			if divide >= ToBase {
				digits[newlen] = divide / ToBase
				divide = divide % ToBase
				newlen++
			} else if newlen > 0 {
				digits[newlen] = 0
				newlen++
			}
		}
		length = newlen
		res = string(charMap[divide]) + res
		if newlen == 0 {
			break
		}
	}
	return res, nil
}

func main() {
	fmt.Println(baseConv("1A2B3C", 16, 62))
	fmt.Println(baseConv("7c9m", 62, 16))
}

package main

import (
	"fmt"
)

func main() {
	fmt.Printf("I am started\r\n")
	for {
		go LeakAway()
	}
	fmt.Printf("I am done, finally!!!!\r\n")
}

func LeakAway() {
	num := 1
	for {
		num++
	}
}

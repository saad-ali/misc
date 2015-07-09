package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Printf("I am started\r\n")
	time.Sleep(24 * 28 * time.Hour)
	fmt.Printf("I am done, finally!!!!\r\n")
}

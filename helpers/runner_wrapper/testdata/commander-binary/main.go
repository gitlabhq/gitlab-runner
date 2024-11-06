package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	time.Sleep(100 * time.Millisecond)

	if len(os.Args) > 1 && os.Args[1] == "fail" {
		fmt.Println("FAIL; exiting with 1")
		os.Exit(1)

		return
	}

	fmt.Println("NOOP; exiting with 0")
}

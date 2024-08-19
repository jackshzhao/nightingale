package main

import (
	"fmt"
)

func main() {
	// Write data
	f := make([]string, 10)
	f = nil

	if len(f) == 0 {
		fmt.Printf("是0")
	} else {
		fmt.Printf("error")
	}

}

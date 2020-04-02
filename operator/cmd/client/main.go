package main

import "fmt"

func main() {
	err := newClient()
	if err != nil {
		fmt.Printf("error: %+v\n", err)
	}
}

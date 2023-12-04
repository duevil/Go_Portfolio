package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Hello, World!")
	zip := os.Getenv("DATA")
	fmt.Println(zip)
}
